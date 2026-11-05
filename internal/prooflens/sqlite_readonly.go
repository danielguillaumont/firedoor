package prooflens

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// OpenSQLiteStoreReadOnly opens an EXISTING Firedoor database without migration,
// journal changes, inserts or schema creation. The caller must register the
// sqlite driver (the Windows CLI registers modernc.org/sqlite).
// SQLite's mode=ro plus query_only defense-in-depth blocks SQL writes. This
// verifies local consistency; it is not authentication or forensic evidence.
func OpenSQLiteStoreReadOnly(ctx context.Context, path string) (*SQLiteStore, error) {
	if path == "" || strings.HasPrefix(strings.ToLower(path), "file:") ||
		strings.ContainsAny(path, "?\x00") || !strings.EqualFold(filepath.Ext(path), ".sqlite") {
		return nil, errors.New("expected an existing plain local .sqlite path")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.New("invalid SQLite path")
	}
	info, err := os.Lstat(absolute)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("read-only SQLite database must already exist as a regular file")
	}
	// Reject symlinked ancestors, including a replaced local evidence directory.
	for dir := filepath.Dir(absolute); ; dir = filepath.Dir(dir) {
		stat, err := os.Lstat(dir)
		if err != nil || !stat.IsDir() || stat.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("SQLite database has an unsafe parent directory")
		}
		if parent := filepath.Dir(dir); parent == dir {
			break
		}
	}
	uriPath := filepath.ToSlash(absolute)
	if filepath.VolumeName(absolute) != "" {
		// Refuse UNC paths: this tool is limited to a local Windows volume.
		if strings.HasPrefix(absolute, `\`) {
			return nil, errors.New("SQLite UNC paths are not supported")
		}
		uriPath = "/" + uriPath
	}
	uri := (&url.URL{Scheme: "file", Path: uriPath, RawQuery: "mode=ro"}).String()
	db, err := sql.Open("sqlite", uri)
	if err != nil {
		return nil, errors.New("SQLite read-only open failed")
	}
	db.SetMaxOpenConns(1)
	ok := false
	defer func() {
		if !ok {
			_ = db.Close()
		}
	}()
	// This PRAGMA modifies only the current connection, never the DB file.
	if _, err := db.ExecContext(ctx, `PRAGMA query_only=ON`); err != nil {
		return nil, errors.New("SQLite query_only enforcement failed")
	}
	var queryOnly, version, appID int
	if err := db.QueryRowContext(ctx, `PRAGMA query_only`).Scan(&queryOnly); err != nil || queryOnly != 1 {
		return nil, errors.New("SQLite query-only mode not active")
	}
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil || version != SQLiteSchemaVersion {
		return nil, errors.New("unexpected read-only SQLite schema version")
	}
	if err := db.QueryRowContext(ctx, `PRAGMA application_id`).Scan(&appID); err != nil || appID != firedoorApplicationID {
		return nil, errors.New("database is not the expected Firedoor application")
	}
	for _, m := range []struct {
		version int
		digest  string
	}{{1, migrationDigest()}, {2, migrationV2Digest()}} {
		var stored string
		if err := db.QueryRowContext(ctx, `SELECT migration_sha256 FROM schema_migrations WHERE version=?`, m.version).Scan(&stored); err != nil || stored != m.digest {
			return nil, errors.New("SQLite migration integrity check failed")
		}
	}
	if _, err := db.ExecContext(ctx, `SELECT receipt_id FROM terraform_plan_receipts LIMIT 0`); err != nil {
		return nil, errors.New("required SQLite receipt table missing")
	}
	ok = true
	return &SQLiteStore{db: db}, nil
}
