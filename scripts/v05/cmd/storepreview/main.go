// This command uses a real SQLite database, but writes SYNTHETIC fixtures only.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/danielguillaumont/firedoor/internal/prooflens"
	_ "modernc.org/sqlite" // Pure-Go driver; no GCC or system sqlite3 required.
)

func main() {
	path := flag.String("db", "", "absolute or relative LOCAL Git-ignored .sqlite file")
	flag.Parse()
	if *path == "" || len(flag.Args()) != 0 {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/v05/cmd/storepreview --db docs/evidence/local/v05-prooflens-synthetic.sqlite")
		os.Exit(2)
	}
	// This preview intentionally refuses to create or mutate a database outside
	// the dedicated local-evidence path. It never consumes real v0.4 evidence.
	root, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	required := filepath.Join(root, "docs", "evidence", "local", "v05-prooflens-synthetic.sqlite")
	actual, err := filepath.Abs(*path)
	if err != nil || actual != required {
		fmt.Fprintln(os.Stderr, "refusing database outside fixed local synthetic evidence path")
		os.Exit(2)
	}
	if err := os.MkdirAll(filepath.Dir(required), 0700); err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := prooflens.OpenSQLiteStore(ctx, actual)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer store.Close()
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		panic(err)
	}
	id := time.Now().UTC().Format("20060102T150405Z-") + hex.EncodeToString(nonce[:])
	b := prooflens.SyntheticExample()
	b.Run.RunID = "RUN-SYNTHETIC-" + id
	b.Run.StartedUTC = time.Now().UTC().Format(time.RFC3339Nano)
	b.Proof.ProofID = "PROOF-SYNTHETIC-" + id
	b.Proof.RunID = b.Run.RunID
	for i := range b.Evidence {
		b.Evidence[i].RunID = b.Run.RunID
		b.Evidence[i].EvidenceID += "-" + id
		b.Evidence[i].CapturedUTC = b.Run.StartedUTC
		b.Run.EvidenceIDs[i] = b.Evidence[i].EvidenceID
	}
	b.Proof.DeclaredEvidenceID = b.Evidence[0].EvidenceID
	b.Proof.ObservedEvidenceID = b.Evidence[1].EvidenceID
	b.Proof.TestedEvidenceID = b.Evidence[2].EvidenceID
	b, err = prooflens.Seal(b)
	if err != nil {
		panic(err)
	}
	if err := store.PutBundle(ctx, b); err != nil {
		panic(err)
	}
	if err := store.Close(); err != nil {
		panic(err)
	}
	store, err = prooflens.OpenSQLiteStore(ctx, actual)
	if err != nil {
		panic(err)
	}
	defer store.Close()
	found, err := store.GetBundle(ctx, b.Run.RunID)
	if err != nil {
		panic(err)
	}
	version, err := store.Version(ctx)
	if err != nil {
		panic(err)
	}
	runs, err := store.ListRuns(ctx, 50)
	if err != nil {
		panic(err)
	}
	p := prooflens.VerifyBundle(found)
	report := struct {
		Mode          string `json:"mode"`
		Database      string `json:"database"`
		SchemaVersion int    `json:"schema_version"`
		RunID         string `json:"run_id"`
		Integrity     string `json:"integrity"`
		ProofVerdict  string `json:"proof_verdict"`
		Reopened      bool   `json:"reopened_verified"`
		RunCount      int    `json:"listed_run_count"`
	}{"SYNTHETIC_SQLITE_ONLY", actual, version, found.Run.RunID, p.Integrity, p.ProofVerdict, found.BundleSHA256 == b.BundleSHA256, len(runs)}
	if !report.Reopened || report.Integrity != prooflens.IntegrityValid || report.ProofVerdict != prooflens.Inconclusive || version != prooflens.SQLiteSchemaVersion {
		fmt.Fprintln(os.Stderr, "SQLite evidence verification failed")
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		panic(err)
	}
}
