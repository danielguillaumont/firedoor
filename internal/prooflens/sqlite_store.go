package prooflens

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SQLiteStore persists validated v0.5 evidence bundles in a local SQLite file.
// Opening it changes that local file only; it never contacts Kubernetes or Terraform.
// The caller must register a SQLite database/sql driver (the v0.5 preview uses
// modernc.org/sqlite, a pure-Go driver) before calling OpenSQLiteStore.
type SQLiteStore struct {
	db *sql.DB
}

const (
	SQLiteSchemaVersion = 2
	// Identifies Firedoor-owned SQLite databases; not a security signature.
	firedoorApplicationID = 1178882613
)

var sqliteMigrationV1 = []string{
	`CREATE TABLE proof_runs (
		run_id TEXT PRIMARY KEY NOT NULL,
		scenario_id TEXT NOT NULL,
		profile TEXT NOT NULL,
		started_utc TEXT NOT NULL,
		bundle_sha256 TEXT NOT NULL,
		bundle_json BLOB NOT NULL,
		inserted_utc TEXT NOT NULL
	)`,
	`CREATE TABLE proof_evidence (
		run_id TEXT NOT NULL REFERENCES proof_runs(run_id) ON DELETE RESTRICT,
		evidence_id TEXT NOT NULL,
		stage TEXT NOT NULL CHECK(stage IN ('declared', 'observed', 'tested')),
		kind TEXT NOT NULL,
		artifact_sha256 TEXT NOT NULL,
		source_sha256 TEXT NOT NULL,
		PRIMARY KEY(run_id, evidence_id),
		UNIQUE(run_id, stage)
	)`,
	`CREATE TABLE proof_claims (
		run_id TEXT PRIMARY KEY NOT NULL REFERENCES proof_runs(run_id) ON DELETE RESTRICT,
		proof_id TEXT NOT NULL,
		verdict TEXT NOT NULL CHECK(verdict = 'inconclusive')
	)`,
	`CREATE INDEX proof_runs_by_time ON proof_runs(started_utc DESC, run_id ASC)`,
	`CREATE TABLE schema_migrations (
		version INTEGER PRIMARY KEY NOT NULL,
		migration_sha256 TEXT NOT NULL,
		applied_utc TEXT NOT NULL
	)`,
}

func migrationDigest() string {
	h := sha256.Sum256([]byte(strings.Join(sqliteMigrationV1, "\n-- migration boundary --\n")))
	return hex.EncodeToString(h[:])
}

// OpenSQLiteStore requires an existing parent directory and a plain .sqlite file
// path. It refuses SQLite URI parameters and unknown/non-Firedoor schemas.
func OpenSQLiteStore(ctx context.Context, path string) (*SQLiteStore, error) {
	if path == "" || strings.HasPrefix(strings.ToLower(path), "file:") ||
		strings.ContainsAny(path, "?\x00") || !strings.EqualFold(filepath.Ext(path), ".sqlite") {
		return nil, errors.New("provide a plain local .sqlite path (not a SQLite URI)")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	parent, err := os.Stat(filepath.Dir(absolute))
	if err != nil || !parent.IsDir() {
		return nil, errors.New("SQLite parent directory must already exist")
	}
	if info, statErr := os.Lstat(absolute); statErr == nil {
		if !info.Mode().IsRegular() {
			return nil, errors.New("refusing non-regular SQLite file")
		}
	} else if !os.IsNotExist(statErr) {
		return nil, statErr
	}

	db, err := sql.Open("sqlite", absolute)
	if err != nil {
		return nil, fmt.Errorf("open SQLite driver (ensure modernc.org/sqlite is registered): %w", err)
	}
	db.SetMaxOpenConns(1) // Keep connection-local foreign-key enforcement consistent.
	cleanup := true
	defer func() {
		if cleanup {
			_ = db.Close()
		}
	}()
	var busyTimeout int
	if err := db.QueryRowContext(ctx, `PRAGMA busy_timeout = 5000`).Scan(&busyTimeout); err != nil || busyTimeout < 5000 {
		return nil, fmt.Errorf("SQLite busy_timeout not configured (value=%d, err=%v)", busyTimeout, err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		return nil, fmt.Errorf("SQLite foreign-key setup: %w", err)
	}
	var fk int
	if err := db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
		return nil, errors.New("SQLite foreign-key enforcement was not enabled")
	}
	store := &SQLiteStore{db: db}
	if err := store.migrate(ctx); err != nil {
		return nil, err
	}
	// Enable durable WAL mode only after verifying the Firedoor DB identity.
	// This avoids changing journal settings on an unrelated existing database.
	var journalMode string
	if err := db.QueryRowContext(ctx, `PRAGMA journal_mode = WAL`).Scan(&journalMode); err != nil || !strings.EqualFold(journalMode, "wal") {
		return nil, fmt.Errorf("Firedoor SQLite requires WAL journal mode (mode=%q, err=%v)", journalMode, err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA synchronous = FULL`); err != nil {
		return nil, fmt.Errorf("configure Firedoor SQLite durability: %w", err)
	}
	cleanup = false
	return store, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) migrate(ctx context.Context) error {
	if err := s.migrateLegacyV1(ctx); err != nil {
		return err
	}
	return s.migratePlanReceiptsV2(ctx)
}

func (s *SQLiteStore) migrateLegacyV1(ctx context.Context) error {
	// A SQLite database from another application must never be silently adopted.
	var version, appID int
	if err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if err := s.db.QueryRowContext(ctx, `PRAGMA application_id`).Scan(&appID); err != nil {
		return err
	}
	if version > SQLiteSchemaVersion || version < 0 {
		return fmt.Errorf("unsupported future SQLite schema version %d", version)
	}
	if appID != 0 && appID != firedoorApplicationID {
		return errors.New("SQLite database belongs to a different application")
	}
	if version >= 1 {
		if appID != firedoorApplicationID {
			return errors.New("SQLite application identity is missing or unexpected")
		}
		var storedDigest string
		if err := s.db.QueryRowContext(ctx, `SELECT migration_sha256 FROM schema_migrations WHERE version = 1`).Scan(&storedDigest); err != nil {
			return fmt.Errorf("missing migration receipt: %w", err)
		}
		if storedDigest != migrationDigest() {
			return errors.New("SQLite migration checksum mismatch")
		}
		return nil
	}
	// v0 -> v1 only: refuse nonempty, unknown databases rather than overwrite.
	var existing int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`).Scan(&existing); err != nil {
		return err
	}
	if existing != 0 || appID != 0 {
		return errors.New("refusing to migrate an existing unknown SQLite schema")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range sqliteMigrationV1 {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("SQLite schema migration v1: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, migration_sha256, applied_utc) VALUES(1, ?, ?)`, migrationDigest(), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	for _, pragma := range []string{`PRAGMA application_id = 1178882613`, `PRAGMA user_version = 1`} {
		if _, err := tx.ExecContext(ctx, pragma); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// PutBundle is append-only. It verifies cross-record hashes before inserting
// all rows atomically; duplicate run IDs never overwrite evidence.
func (s *SQLiteStore) PutBundle(ctx context.Context, b Bundle) error {
	if s == nil || s.db == nil {
		return errors.New("SQLite store is closed")
	}
	review := VerifyBundle(b)
	if review.Integrity != IntegrityValid || review.ProofVerdict != Inconclusive {
		return fmt.Errorf("refusing invalid proof bundle: %s", strings.Join(review.Problems, "; "))
	}
	payload, err := json.Marshal(b)
	if err != nil {
		return err
	}
	if len(payload) > MaxBundleBytes {
		return errors.New("proof bundle exceeds 1 MiB")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO proof_runs(run_id,scenario_id,profile,started_utc,bundle_sha256,bundle_json,inserted_utc) VALUES(?,?,?,?,?,?,?)`,
		b.Run.RunID, b.Run.ScenarioID, b.Run.Profile, b.Run.StartedUTC, b.BundleSHA256, payload, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("run already exists or SQLite insert failed: %w", err)
	}
	for _, e := range b.Evidence {
		if _, err := tx.ExecContext(ctx, `INSERT INTO proof_evidence(run_id,evidence_id,stage,kind,artifact_sha256,source_sha256) VALUES(?,?,?,?,?,?)`,
			e.RunID, e.EvidenceID, string(e.Stage), e.Kind, e.ArtifactSHA256, e.Source.SHA256); err != nil {
			return fmt.Errorf("insert evidence: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO proof_claims(run_id,proof_id,verdict) VALUES(?,?,?)`,
		b.Proof.RunID, b.Proof.ProofID, b.Proof.Verdict); err != nil {
		return fmt.Errorf("insert proof: %w", err)
	}
	return tx.Commit()
}

// GetBundle checks the serialized bundle plus the independent SQL index rows.
// Local SQL data remains unauthenticated even when all hashes match.
func (s *SQLiteStore) GetBundle(ctx context.Context, runID string) (Bundle, error) {
	if s == nil || s.db == nil {
		return Bundle{}, errors.New("SQLite store is closed")
	}
	if !validText(runID, 128) {
		return Bundle{}, errors.New("invalid run ID")
	}
	var payload []byte
	var storedHash, scenario, profile, started string
	err := s.db.QueryRowContext(ctx, `SELECT bundle_json,bundle_sha256,scenario_id,profile,started_utc FROM proof_runs WHERE run_id = ?`, runID).
		Scan(&payload, &storedHash, &scenario, &profile, &started)
	if err != nil {
		return Bundle{}, err
	}
	b, err := DecodeBundle(payload)
	if err != nil {
		return Bundle{}, err
	}
	if b.Run.RunID != runID || b.BundleSHA256 != storedHash || b.Run.ScenarioID != scenario || b.Run.Profile != profile || b.Run.StartedUTC != started ||
		VerifyBundle(b).Integrity != IntegrityValid {
		return Bundle{}, errors.New("SQLite bundle/index integrity mismatch")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT evidence_id,stage,kind,artifact_sha256,source_sha256 FROM proof_evidence WHERE run_id = ?`, runID)
	if err != nil {
		return Bundle{}, err
	}
	matches := make(map[string]bool, len(b.Evidence))
	for rows.Next() {
		var id, stage, kind, artifactHash, sourceHash string
		if err := rows.Scan(&id, &stage, &kind, &artifactHash, &sourceHash); err != nil {
			rows.Close()
			return Bundle{}, err
		}
		found := false
		for _, e := range b.Evidence {
			if e.EvidenceID == id && string(e.Stage) == stage && e.Kind == kind && e.ArtifactSHA256 == artifactHash && e.Source.SHA256 == sourceHash && !matches[id] {
				matches[id] = true
				found = true
				break
			}
		}
		if !found {
			rows.Close()
			return Bundle{}, errors.New("SQLite evidence index does not match the bundle")
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Bundle{}, err
	}
	rows.Close()
	if len(matches) != len(b.Evidence) {
		return Bundle{}, errors.New("SQLite evidence rows are missing")
	}
	var proofID, verdict string
	if err := s.db.QueryRowContext(ctx, `SELECT proof_id,verdict FROM proof_claims WHERE run_id = ?`, runID).Scan(&proofID, &verdict); err != nil {
		return Bundle{}, err
	}
	if proofID != b.Proof.ProofID || verdict != b.Proof.Verdict {
		return Bundle{}, errors.New("SQLite proof row does not match the bundle")
	}
	return b, nil
}

type StoredRun struct {
	RunID        string `json:"run_id"`
	ScenarioID   string `json:"scenario_id"`
	Profile      string `json:"profile"`
	StartedUTC   string `json:"started_utc"`
	BundleSHA256 string `json:"bundle_sha256"`
}

// ListRuns returns at most 50 recent local records; no unbounded scans.
func (s *SQLiteStore) ListRuns(ctx context.Context, limit int) ([]StoredRun, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("SQLite store is closed")
	}
	if limit < 1 || limit > 50 {
		return nil, errors.New("limit must be between 1 and 50")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT run_id,scenario_id,profile,started_utc,bundle_sha256 FROM proof_runs ORDER BY started_utc DESC,run_id ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StoredRun
	for rows.Next() {
		var r StoredRun
		if err := rows.Scan(&r.RunID, &r.ScenarioID, &r.Profile, &r.StartedUTC, &r.BundleSHA256); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Version(ctx context.Context) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("SQLite store is closed")
	}
	var v int
	err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&v)
	return v, err
}
