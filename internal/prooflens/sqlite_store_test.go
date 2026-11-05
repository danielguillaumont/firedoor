package prooflens

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func makeSQLiteFixture(t *testing.T) Bundle {
	t.Helper()
	b, err := Seal(SyntheticExample())
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSQLiteMigrationPersistenceAndIdempotentReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "prooflens.sqlite")
	store, err := OpenSQLiteStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := store.Version(ctx); err != nil || v != SQLiteSchemaVersion {
		t.Fatalf("unexpected migration version %d: %v", v, err)
	}
	bundle := makeSQLiteFixture(t)
	if err := store.PutBundle(ctx, bundle); err != nil {
		t.Fatal(err)
	}
	if err := store.PutBundle(ctx, bundle); err == nil {
		t.Fatal("duplicate run insert unexpectedly allowed")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenSQLiteStore(ctx, path)
	if err != nil {
		t.Fatalf("reopening existing SQLite DB failed: %v", err)
	}
	defer store.Close()
	read, err := store.GetBundle(ctx, bundle.Run.RunID)
	if err != nil || read.BundleSHA256 != bundle.BundleSHA256 {
		t.Fatalf("reopen/roundtrip failed: %v", err)
	}
	runs, err := store.ListRuns(ctx, 10)
	if err != nil || len(runs) != 1 || runs[0].RunID != bundle.Run.RunID {
		t.Fatalf("incorrect local index: %v %v", runs, err)
	}
}

func TestSQLiteRejectsTamperAndNeverUpgradesProof(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLiteStore(ctx, filepath.Join(t.TempDir(), "prooflens.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	bundle := makeSQLiteFixture(t)
	if err := store.PutBundle(ctx, bundle); err != nil {
		t.Fatal(err)
	}
	read, err := store.GetBundle(ctx, bundle.Run.RunID)
	if err != nil || read.Proof.Verdict != Inconclusive {
		t.Fatalf("SQLite incorrectly promoted proof: %v", err)
	}
	if _, err := store.db.Exec(`UPDATE proof_evidence SET artifact_sha256 = 'tampered' WHERE run_id = ? AND stage = 'tested'`, bundle.Run.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetBundle(ctx, bundle.Run.RunID); err == nil {
		t.Fatal("modified SQL index was accepted")
	}
}

func TestSQLiteAtomicInvalidInsertAndLimits(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLiteStore(ctx, filepath.Join(t.TempDir(), "prooflens.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	bundle := makeSQLiteFixture(t)
	bundle.Proof.Verdict = "supported"
	if err := store.PutBundle(ctx, bundle); err == nil {
		t.Fatal("unsafe verdict inserted")
	}
	runs, err := store.ListRuns(ctx, 10)
	if err != nil || len(runs) != 0 {
		t.Fatalf("failed write left rows: %v %v", runs, err)
	}
	for _, count := range []int{0, -1, 51} {
		if _, err := store.ListRuns(ctx, count); err == nil {
			t.Fatalf("unsafe limit %d accepted", count)
		}
	}
	if _, err := store.GetBundle(ctx, "not-there"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected missing run, received %v", err)
	}
}

func TestSQLiteRefusesUnknownExistingDatabaseAndFutureSchema(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		init string
	}{
		{"other-app", `PRAGMA application_id = 777;`},
		{"future-version", `PRAGMA user_version = 77;`},
		{"unknown-schema", `CREATE TABLE other_data (secret TEXT);`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "prooflens.sqlite")
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(tc.init); err != nil {
				t.Fatal(err)
			}
			db.Close()
			if s, err := OpenSQLiteStore(ctx, path); err == nil {
				s.Close()
				t.Fatal("non-Firedoor SQLite database was adopted")
			}
		})
	}
	if _, err := OpenSQLiteStore(ctx, "file:memory:?mode=memory"); err == nil {
		t.Fatal("unsafe SQLite URI accepted")
	}
	if _, err := OpenSQLiteStore(ctx, filepath.Join(t.TempDir(), "no-parent", "prooflens.sqlite")); err == nil {
		t.Fatal("unknown parent directory created")
	}
}

func TestSQLiteFailsClosedWhenMigrationReceiptChanges(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "prooflens.sqlite")
	s, err := OpenSQLiteStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE schema_migrations SET migration_sha256='wrong' WHERE version=1`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if reopened, err := OpenSQLiteStore(ctx, path); err == nil {
		reopened.Close()
		t.Fatal("tampered migration checksum accepted")
	} else if !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSQLiteRefusesNonRegularPath(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "folder.sqlite")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if s, err := OpenSQLiteStore(ctx, path); err == nil {
		s.Close()
		t.Fatal("directory accepted as SQLite file")
	}
}
