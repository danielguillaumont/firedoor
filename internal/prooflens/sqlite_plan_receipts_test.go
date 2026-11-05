package prooflens

import (
	"context"
	"database/sql"
	"encoding/json"
	_ "modernc.org/sqlite"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func syntheticCompletedPlanReport(t *testing.T) TerraformLivePlanReport {
	t.Helper()
	root := tfFixture(t)
	manifest, err := CollectTerraformSources(root)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := SummarizeTerraformUIStream(strings.NewReader(syntheticUILines), manifest)
	if err != nil {
		t.Fatal(err)
	}
	return TerraformLivePlanReport{
		SchemaVersion: TerraformLivePlanSchema, Mode: TerraformLivePlanMode,
		SpeculativeOnly: true, PlanCompleted: true, Applied: false, RawOutputSaved: false,
		SourceTreeSHA256: manifest.TreeSHA256, SanitizedPlanSummary: summary,
		Limitations: []string{"Speculative and may read state.", "No apply.", "Raw stream not preserved."},
	}
}

func makePlanReceipt(t *testing.T) PlanReceipt {
	t.Helper()
	receipt, err := SealLivePlanReceipt(syntheticCompletedPlanReport(t), "TFPLAN-TEST-0001")
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func TestPlanReceiptStorageAndReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "plans.sqlite")
	s, err := OpenSQLiteStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := s.Version(ctx); err != nil || v != 2 {
		t.Fatalf("expected schema2, got %d: %v", v, err)
	}
	r := makePlanReceipt(t)
	if err := s.PutPlanReceipt(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err := s.PutPlanReceipt(ctx, r); err == nil {
		t.Fatal("duplicate receipt accepted")
	}
	// Evidence bundles and planning receipts use separate tables.
	b := makeSQLiteFixture(t)
	if err := s.PutBundle(ctx, b); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = OpenSQLiteStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	loaded, err := s.GetPlanReceipt(ctx, r.ReceiptID)
	if err != nil || loaded.ReceiptSHA256 != r.ReceiptSHA256 || loaded.ProofVerdict != Inconclusive {
		t.Fatalf("reopen failed: %v", err)
	}
	found, err := s.ListPlanReceipts(ctx, 10)
	if err != nil || len(found) != 1 || found[0].ReceiptID != r.ReceiptID {
		t.Fatalf("list mismatch: %v %v", found, err)
	}
	bundles, err := s.ListRuns(ctx, 10)
	if err != nil || len(bundles) != 1 || bundles[0].RunID != b.Run.RunID {
		t.Fatalf("bundle index changed: %v", err)
	}
}

func TestPlanReceiptTamperAndFailClosed(t *testing.T) {
	ctx := context.Background()
	s, err := OpenSQLiteStore(ctx, filepath.Join(t.TempDir(), "plans.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := makePlanReceipt(t)
	for _, mutate := range []func(*PlanReceipt){
		func(r *PlanReceipt) { r.Applied = true },
		func(r *PlanReceipt) { r.PlanCompleted = false },
		func(r *PlanReceipt) { r.RawOutputSaved = true },
		func(r *PlanReceipt) { r.ProofVerdict = "supported" },
		func(r *PlanReceipt) { r.SanitizedPlanSummary.Changes.Add++ },
		func(r *PlanReceipt) { r.SourceTreeSHA256 = strings.Repeat("a", 64) },
	} {
		copy := p
		mutate(&copy)
		if err := s.PutPlanReceipt(ctx, copy); err == nil {
			t.Fatal("invalid or tampered record accepted")
		}
	}
	if err := s.PutPlanReceipt(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE terraform_plan_receipts SET ui_stream_sha256='changed' WHERE receipt_id=?`, p.ReceiptID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetPlanReceipt(ctx, p.ReceiptID); err == nil {
		t.Fatal("corrupted index accepted")
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE terraform_plan_receipts SET ui_stream_sha256=? WHERE receipt_id=?`, p.SanitizedPlanSummary.UIStreamSHA256, p.ReceiptID); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(p)
	body := strings.Replace(string(payload), `"proof_verdict":"inconclusive"`, `"proof_verdict":"supported"`, 1)
	if _, err := s.db.ExecContext(ctx, `UPDATE terraform_plan_receipts SET receipt_json=? WHERE receipt_id=?`, []byte(body), p.ReceiptID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetPlanReceipt(ctx, p.ReceiptID); err == nil {
		t.Fatal("corrupted payload accepted")
	}
	for _, limit := range []int{0, -1, 51} {
		if _, err := s.ListPlanReceipts(ctx, limit); err == nil {
			t.Fatal("invalid list limit accepted")
		}
	}
}

func TestSQLiteMigrationV1ToV2PreservesPriorEvidence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "prior.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range sqliteMigrationV1 {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version,migration_sha256,applied_utc) VALUES(1,?,?)`, migrationDigest(), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{`PRAGMA application_id=1178882613`, `PRAGMA user_version=1`} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	b := makeSQLiteFixture(t)
	data, _ := json.Marshal(b)
	if _, err := db.ExecContext(ctx, `INSERT INTO proof_runs(run_id,scenario_id,profile,started_utc,bundle_sha256,bundle_json,inserted_utc) VALUES(?,?,?,?,?,?,?)`, b.Run.RunID, b.Run.ScenarioID, b.Run.Profile, b.Run.StartedUTC, b.BundleSHA256, data, b.Run.StartedUTC); err != nil {
		t.Fatal(err)
	}
	for _, e := range b.Evidence {
		if _, err := db.ExecContext(ctx, `INSERT INTO proof_evidence(run_id,evidence_id,stage,kind,artifact_sha256,source_sha256) VALUES(?,?,?,?,?,?)`, e.RunID, e.EvidenceID, e.Stage, e.Kind, e.ArtifactSHA256, e.Source.SHA256); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO proof_claims(run_id,proof_id,verdict) VALUES(?,?,?)`, b.Run.RunID, b.Proof.ProofID, b.Proof.Verdict); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := OpenSQLiteStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if v, _ := s.Version(ctx); v != 2 {
		t.Fatalf("migration failed: %d", v)
	}
	old, err := s.GetBundle(ctx, b.Run.RunID)
	if err != nil || old.BundleSHA256 != b.BundleSHA256 {
		t.Fatalf("preexisting v1 proof was altered: %v", err)
	}
	if err := s.PutPlanReceipt(ctx, makePlanReceipt(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE schema_migrations SET migration_sha256='wrong' WHERE version=2`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if invalid, err := OpenSQLiteStore(ctx, path); err == nil {
		invalid.Close()
		t.Fatal("tampered v2 receipt accepted")
	}
}

func TestPlanReceiptCannotBeBuiltFromIncompleteOrAppliedPlan(t *testing.T) {
	good := syntheticCompletedPlanReport(t)
	for _, mutate := range []func(*TerraformLivePlanReport){
		func(r *TerraformLivePlanReport) { r.PlanCompleted = false },
		func(r *TerraformLivePlanReport) { r.Applied = true },
		func(r *TerraformLivePlanReport) { r.RawOutputSaved = true },
		func(r *TerraformLivePlanReport) { r.SanitizedPlanSummary.ProofVerdict = "supported" },
	} {
		r := good
		mutate(&r)
		if _, err := SealLivePlanReceipt(r, "TFPLAN-TEST"); err == nil {
			t.Fatal("unsafe report sealed")
		}
	}
	if _, err := SealLivePlanReceipt(good, "BADID"); err == nil {
		t.Fatal("bad ID accepted")
	}
}
