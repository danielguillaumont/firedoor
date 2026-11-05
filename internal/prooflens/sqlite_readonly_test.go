package prooflens

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReadOnlyReceiptStoreReopensWithoutMigrationOrWrites(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "evidence.sqlite")
	writer, err := OpenSQLiteStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	p := makePlanReceipt(t)
	if err := writer.PutPlanReceipt(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := OpenSQLiteStoreReadOnly(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	version, err := reader.Version(ctx)
	if err != nil || version != 2 {
		t.Fatalf("schema mismatch: %d, %v", version, err)
	}
	got, err := reader.GetPlanReceipt(ctx, p.ReceiptID)
	if err != nil || got.ReceiptSHA256 != p.ReceiptSHA256 {
		t.Fatalf("independent read-only reopen failed: %v", err)
	}
	if err := reader.PutPlanReceipt(ctx, p); err == nil {
		t.Fatal("query-only connection accepted a plan receipt write")
	}
	if _, err := reader.db.ExecContext(ctx, `UPDATE schema_migrations SET migration_sha256='x' WHERE version=1`); err == nil {
		t.Fatal("query-only connection accepted schema update")
	}
}

func TestReadOnlyReceiptStoreRejectsMissingAndWrongSchema(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "missing.sqlite")
	if s, err := OpenSQLiteStoreReadOnly(ctx, path); err == nil {
		s.Close()
		t.Fatal("read-only open fabricated missing SQLite file")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("read-only open created database, stat error=%v", err)
	}
	if err := os.WriteFile(path, []byte("not a SQLite database"), 0600); err != nil {
		t.Fatal(err)
	}
	if s, err := OpenSQLiteStoreReadOnly(ctx, path); err == nil {
		s.Close()
		t.Fatal("invalid file accepted")
	}
	for _, candidate := range []string{"file:invalid.sqlite", "missing.db", "bad.sqlite?mode=rw"} {
		if s, err := OpenSQLiteStoreReadOnly(ctx, candidate); err == nil {
			s.Close()
			t.Fatal("unsafe path accepted:", candidate)
		}
	}
}

func TestReadOnlyReceiptStoreRejectsTamperedMigration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "evidence.sqlite")
	writer, err := OpenSQLiteStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.db.ExecContext(ctx, `UPDATE schema_migrations SET migration_sha256='broken' WHERE version=2`); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	if s, err := OpenSQLiteStoreReadOnly(ctx, path); err == nil {
		s.Close()
		t.Fatal("tampered migration accepted")
	}
}
