package prooflens

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// Schema v2 adds plan receipts as a SEPARATE evidence category. A speculative
// plan is NOT a Proof Bundle: it supplies no actual observed/tested evidence.
var sqliteMigrationV2 = []string{
	`CREATE TABLE terraform_plan_receipts (
        receipt_id TEXT PRIMARY KEY NOT NULL,
        captured_utc TEXT NOT NULL,
        source_tree_sha256 TEXT NOT NULL,
        ui_stream_sha256 TEXT NOT NULL,
        receipt_sha256 TEXT NOT NULL,
        verdict TEXT NOT NULL CHECK(verdict = 'inconclusive'),
        receipt_json BLOB NOT NULL,
        inserted_utc TEXT NOT NULL
    )`,
	`CREATE INDEX terraform_plan_receipts_by_time ON terraform_plan_receipts(captured_utc DESC, receipt_id ASC)`,
}

func migrationV2Digest() string {
	h := sha256.Sum256([]byte(strings.Join(sqliteMigrationV2, "\n-- migration boundary --\n")))
	return hex.EncodeToString(h[:])
}

// migratePlanReceiptsV2 is monotonic, transactional, and idempotent. It verifies
// the previous migration receipt before making any changes to an existing DB.
func (s *SQLiteStore) migratePlanReceiptsV2(ctx context.Context) error {
	var version, application int
	if err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if err := s.db.QueryRowContext(ctx, `PRAGMA application_id`).Scan(&application); err != nil {
		return err
	}
	if application != firedoorApplicationID || version < 1 || version > SQLiteSchemaVersion {
		return errors.New("unexpected Firedoor SQLite application or schema version")
	}
	var digest string
	if err := s.db.QueryRowContext(ctx, `SELECT migration_sha256 FROM schema_migrations WHERE version = 1`).Scan(&digest); err != nil || digest != migrationDigest() {
		return errors.New("SQLite migration v1 receipt missing or changed")
	}
	if version == SQLiteSchemaVersion {
		if err := s.db.QueryRowContext(ctx, `SELECT migration_sha256 FROM schema_migrations WHERE version = 2`).Scan(&digest); err != nil || digest != migrationV2Digest() {
			return errors.New("SQLite migration v2 receipt missing or changed")
		}
		// A valid receipt is insufficient if the table itself is missing.
		if _, err := s.db.ExecContext(ctx, `SELECT receipt_id FROM terraform_plan_receipts LIMIT 0`); err != nil {
			return errors.New("SQLite plan receipt table missing")
		}
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range sqliteMigrationV2 {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return errors.New("SQLite migration v2 failed")
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version,migration_sha256,applied_utc) VALUES(2,?,?)`, migrationV2Digest(), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `PRAGMA user_version = 2`); err != nil {
		return err
	}
	return tx.Commit()
}

const (
	TerraformPlanReceiptSchema = "firedoor.v05.plan-receipt.v1"
	maxPlanReceiptBytes        = 16 * 1024
)

// PlanReceipt records a sanitized speculative plan. Its hash detects local
// inconsistency; it is NOT a signature, authenticated origin, or Kubernetes proof.
type PlanReceipt struct {
	SchemaVersion        string                 `json:"schema_version"`
	ReceiptID            string                 `json:"receipt_id"`
	RecordedUTC          string                 `json:"recorded_utc"`
	SpeculativeOnly      bool                   `json:"speculative_only"`
	PlanCompleted        bool                   `json:"plan_completed"`
	Applied              bool                   `json:"applied"`
	RawOutputSaved       bool                   `json:"raw_output_saved"`
	SourceTreeSHA256     string                 `json:"source_tree_sha256"`
	SanitizedPlanSummary TerraformUIPlanSummary `json:"sanitized_plan_summary"`
	ProofVerdict         string                 `json:"proof_verdict"`
	Limitations          []string               `json:"limitations"`
	ReceiptSHA256        string                 `json:"receipt_sha256"`
}

func receiptLimitations() []string {
	return []string{
		"Speculative Terraform plan only; not an apply or an assertion of deployed state.",
		"No Kubernetes observation or live network test is attached to this receipt.",
		"Local SHA256 checks detect changes, but do not authenticate origin or prevent a coordinated rewrite.",
	}
}

func validLiveReport(r TerraformLivePlanReport) bool {
	return r.SchemaVersion == TerraformLivePlanSchema && r.Mode == TerraformLivePlanMode &&
		r.SpeculativeOnly && r.PlanCompleted && !r.Applied && !r.RawOutputSaved &&
		validHex(r.SourceTreeSHA256, 64) &&
		VerifyTerraformUIPlanSummary(r.SanitizedPlanSummary) == nil &&
		r.SourceTreeSHA256 == r.SanitizedPlanSummary.SourceTreeSHA256 &&
		len(r.Limitations) == 3 && checkLimitations(r.Limitations)
}

func receiptChecksum(p PlanReceipt) (string, error) {
	p.ReceiptSHA256 = ""
	payload, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

// SealLivePlanReceipt only converts a completed, sanitized local-plan report.
// Never call it on raw UI output or on terraform show -json output.
func SealLivePlanReceipt(r TerraformLivePlanReport, receiptID string) (PlanReceipt, error) {
	if !validLiveReport(r) || !validText(receiptID, 128) || !strings.HasPrefix(receiptID, "TFPLAN-") {
		return PlanReceipt{}, errors.New("refusing incomplete or unsafe plan receipt")
	}
	p := PlanReceipt{
		SchemaVersion: TerraformPlanReceiptSchema, ReceiptID: receiptID,
		RecordedUTC:     time.Now().UTC().Format(time.RFC3339Nano),
		SpeculativeOnly: true, PlanCompleted: true, Applied: false, RawOutputSaved: false,
		SourceTreeSHA256: r.SourceTreeSHA256, SanitizedPlanSummary: r.SanitizedPlanSummary,
		ProofVerdict: Inconclusive, Limitations: receiptLimitations(),
	}
	hash, err := receiptChecksum(p)
	if err != nil {
		return PlanReceipt{}, err
	}
	p.ReceiptSHA256 = hash
	if err := VerifyLivePlanReceipt(p); err != nil {
		return PlanReceipt{}, err
	}
	return p, nil
}

func VerifyLivePlanReceipt(p PlanReceipt) error {
	if p.SchemaVersion != TerraformPlanReceiptSchema || !validText(p.ReceiptID, 128) ||
		!strings.HasPrefix(p.ReceiptID, "TFPLAN-") || !validUTC(p.RecordedUTC) ||
		!p.SpeculativeOnly || !p.PlanCompleted || p.Applied || p.RawOutputSaved ||
		!validHex(p.SourceTreeSHA256, 64) || p.ProofVerdict != Inconclusive ||
		VerifyTerraformUIPlanSummary(p.SanitizedPlanSummary) != nil ||
		p.SourceTreeSHA256 != p.SanitizedPlanSummary.SourceTreeSHA256 ||
		len(p.Limitations) != 3 || strings.Join(p.Limitations, "\n") != strings.Join(receiptLimitations(), "\n") ||
		!validHex(p.ReceiptSHA256, 64) {
		return errors.New("plan receipt shape or integrity metadata invalid")
	}
	actual, err := receiptChecksum(p)
	if err != nil || actual != p.ReceiptSHA256 {
		return errors.New("plan receipt SHA256 mismatch")
	}
	data, err := json.Marshal(p)
	if err != nil || len(data) > maxPlanReceiptBytes {
		return errors.New("plan receipt exceeds permitted size")
	}
	return nil
}

// PutPlanReceipt appends a separate immutable local planning record.
// It never synthesizes observed/tested evidence or upgrades the proof verdict.
func (s *SQLiteStore) PutPlanReceipt(ctx context.Context, p PlanReceipt) error {
	if s == nil || s.db == nil {
		return errors.New("SQLite store closed")
	}
	if err := VerifyLivePlanReceipt(p); err != nil {
		return err
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO terraform_plan_receipts(receipt_id,captured_utc,source_tree_sha256,ui_stream_sha256,receipt_sha256,verdict,receipt_json,inserted_utc) VALUES(?,?,?,?,?,?,?,?)`,
		p.ReceiptID, p.SanitizedPlanSummary.CapturedUTC, p.SourceTreeSHA256,
		p.SanitizedPlanSummary.UIStreamSHA256, p.ReceiptSHA256, Inconclusive, payload,
		time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("append-only plan receipt rejected: %w", err)
	}
	return tx.Commit()
}

// GetPlanReceipt verifies both the JSON body and independently indexed fields.
func (s *SQLiteStore) GetPlanReceipt(ctx context.Context, receiptID string) (PlanReceipt, error) {
	if s == nil || s.db == nil {
		return PlanReceipt{}, errors.New("SQLite store closed")
	}
	if !validText(receiptID, 128) {
		return PlanReceipt{}, errors.New("invalid plan receipt ID")
	}
	var data []byte
	var captured, source, stream, digest, verdict string
	err := s.db.QueryRowContext(ctx, `SELECT receipt_json,captured_utc,source_tree_sha256,ui_stream_sha256,receipt_sha256,verdict FROM terraform_plan_receipts WHERE receipt_id=?`, receiptID).
		Scan(&data, &captured, &source, &stream, &digest, &verdict)
	if err != nil {
		return PlanReceipt{}, err
	}
	if len(data) > maxPlanReceiptBytes {
		return PlanReceipt{}, errors.New("overlarge stored plan receipt")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var p PlanReceipt
	if err := decoder.Decode(&p); err != nil {
		return PlanReceipt{}, errors.New("invalid stored plan receipt JSON")
	}
	var tail any
	if err := decoder.Decode(&tail); !errors.Is(err, io.EOF) {
		return PlanReceipt{}, errors.New("trailing stored plan receipt JSON")
	}
	if p.ReceiptID != receiptID || p.SanitizedPlanSummary.CapturedUTC != captured ||
		p.SourceTreeSHA256 != source || p.SanitizedPlanSummary.UIStreamSHA256 != stream ||
		p.ReceiptSHA256 != digest || verdict != Inconclusive || VerifyLivePlanReceipt(p) != nil {
		return PlanReceipt{}, errors.New("SQLite plan receipt/index integrity mismatch")
	}
	return p, nil
}

type StoredPlanReceipt struct {
	ReceiptID        string `json:"receipt_id"`
	CapturedUTC      string `json:"captured_utc"`
	SourceTreeSHA256 string `json:"source_tree_sha256"`
	ReceiptSHA256    string `json:"receipt_sha256"`
	ProofVerdict     string `json:"proof_verdict"`
}

func (s *SQLiteStore) ListPlanReceipts(ctx context.Context, limit int) ([]StoredPlanReceipt, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("SQLite store closed")
	}
	if limit < 1 || limit > 50 {
		return nil, errors.New("limit must be between 1 and 50")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT receipt_id,captured_utc,source_tree_sha256,receipt_sha256,verdict FROM terraform_plan_receipts ORDER BY captured_utc DESC,receipt_id ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StoredPlanReceipt
	for rows.Next() {
		var r StoredPlanReceipt
		if err := rows.Scan(&r.ReceiptID, &r.CapturedUTC, &r.SourceTreeSHA256, &r.ReceiptSHA256, &r.ProofVerdict); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
