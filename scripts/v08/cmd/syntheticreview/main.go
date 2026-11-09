// Command syntheticreview produces explicitly synthetic, human-reviewable v0.8
// report samples. Its real comparison CLI invocation runs in a disposable Git
// repository containing ONLY generated illustrative documents. It does not
// read any evidence from the Firedoor checkout or contact Kubernetes/Terraform.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danielguillaumont/firedoor/internal/cli"
	"github.com/danielguillaumont/firedoor/internal/prooflens"
	"github.com/danielguillaumont/firedoor/internal/reporting"
)

const syntheticWarning = "SYNTHETIC DEMO — ILLUSTRATIVE INPUTS ONLY. NO REAL EVIDENCE, POLICY ENFORCEMENT OR SECURITY IMPROVEMENT IS PROVEN."

func main() {
	out := flag.String("out", "", "Absolute, not-yet-existing directory OUTSIDE the Firedoor checkout")
	flag.Parse()
	if flag.NArg() != 0 || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: syntheticreview --out ABSOLUTE_NEW_DIRECTORY")
		os.Exit(2)
	}
	if err := generate(*out); err != nil {
		fmt.Fprintln(os.Stderr, "synthetic review refused:", err)
		os.Exit(1)
	}
	fmt.Println("PASS: SYNTHETIC v0.8 review files created outside the repository:", *out)
	fmt.Println("Open SYNTHETIC-comparison.html for visual review. No real evidence was read.")
}

func generate(out string) (err error) {
	if !filepath.IsAbs(out) || filepath.Clean(out) != out {
		return errors.New("output directory must be an absolute, normalized path")
	}
	parent := filepath.Dir(out)
	parentResolved, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return errors.New("output parent does not exist")
	}
	working, err := os.Getwd()
	if err != nil {
		return errors.New("working directory is unavailable")
	}
	git := exec.Command("git", "rev-parse", "--show-toplevel")
	git.Dir = working
	rootBytes, err := git.Output()
	if err != nil {
		return errors.New("run from the Firedoor Git checkout")
	}
	checkout, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
	if err != nil {
		return errors.New("Firedoor checkout path unavailable")
	}
	actualTarget := filepath.Join(parentResolved, filepath.Base(out))
	if rel, err := filepath.Rel(checkout, actualTarget); err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))) {
		return errors.New("refusing output within the Firedoor Git checkout")
	}
	// Mkdir, unlike MkdirAll, fails rather than touching an existing destination.
	if err := os.Mkdir(out, 0700); err != nil {
		return errors.New("output directory already exists or cannot be created")
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(out) // Only the directory freshly created by this run.
		}
	}()

	fixture, err := os.MkdirTemp("", "firedoor-v08-synthetic-fixture-")
	if err != nil {
		return errors.New("cannot create a disposable fixture repository")
	}
	defer os.RemoveAll(fixture)
	init := exec.Command("git", "init", "-q", "-b", "feat/v08-proofs", fixture)
	if output, err := init.CombinedOutput(); err != nil {
		return fmt.Errorf("cannot initialize disposable fixture Git repository: %s", strings.TrimSpace(string(output)))
	}
	if err := os.WriteFile(filepath.Join(fixture, ".gitignore"), []byte("/docs/evidence/local/\n"), 0600); err != nil {
		return err
	}
	local := filepath.Join(fixture, "docs", "evidence", "local")
	if err := os.MkdirAll(local, 0700); err != nil {
		return err
	}
	baseline, hardened, err := syntheticDocuments()
	if err != nil {
		return err
	}
	baseJSON, err := reporting.RenderJSON(baseline)
	if err != nil {
		return err
	}
	hardJSON, err := reporting.RenderJSON(hardened)
	if err != nil {
		return err
	}
	for _, item := range []struct {
		rel  string
		data []byte
	}{
		{prooflens.LocalBaselineProofJSON, baseJSON},
		{prooflens.LocalHardenedProofJSON, hardJSON},
	} {
		path := filepath.Join(fixture, filepath.FromSlash(item.rel))
		if err := exclusiveWrite(path, item.data); err != nil {
			return err
		}
	}

	// Exercise the ACTUAL v0.8 Cobra CLI in a temporary repository, not merely
	// the pure comparison renderer. Consent applies ONLY to illustrative data.
	command := cli.NewRootCommand()
	command.SetArgs([]string{"proof", "export-comparison", "--root", fixture, "--consent", prooflens.BoundaryComparisonExportConsent})
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	if err := command.Execute(); err != nil {
		return errors.New("real offline comparison CLI rejected synthetic fixture")
	}
	comparisonJSON, err := os.ReadFile(filepath.Join(fixture, filepath.FromSlash(prooflens.LocalComparisonJSON)))
	if err != nil {
		return err
	}
	comparisonHTML, err := os.ReadFile(filepath.Join(fixture, filepath.FromSlash(prooflens.LocalComparisonHTML)))
	if err != nil {
		return err
	}
	var document reporting.ComparisonDocument
	if err := json.Unmarshal(comparisonJSON, &document); err != nil || document.Validate() != nil ||
		document.Verdict != reporting.VerdictInconclusive || document.DriftAssessment != reporting.DriftNotAssessed {
		return errors.New("synthetic CLI comparison contract invalid")
	}
	if document.TerraformIntent.EvidenceRelation != reporting.RelationDifferentDigest ||
		document.KubernetesObservation.EvidenceRelation != reporting.RelationNotComparable ||
		document.NetworkBehavior.EvidenceRelation != reporting.RelationSameDigest {
		return errors.New("synthetic comparison relationships incorrect")
	}
	canonicalJSON, _ := reporting.RenderComparisonJSON(document)
	canonicalHTML, _ := reporting.RenderComparisonHTML(document)
	if !bytes.Equal(canonicalJSON, comparisonJSON) || !bytes.Equal(canonicalHTML, comparisonHTML) {
		return errors.New("comparison CLI output differs from validated renderer")
	}
	// The CLI must also refuse overwriting the same pair.
	retry := cli.NewRootCommand()
	retry.SetArgs([]string{"proof", "export-comparison", "--root", fixture, "--consent", prooflens.BoundaryComparisonExportConsent})
	retry.SetOut(io.Discard)
	retry.SetErr(io.Discard)
	if retry.Execute() == nil {
		return errors.New("comparison CLI unexpectedly overwrote synthetic output")
	}

	baseHTML, err := reporting.RenderHTML(baseline)
	if err != nil {
		return err
	}
	hardHTML, err := reporting.RenderHTML(hardened)
	if err != nil {
		return err
	}
	// Review HTML is the real report markup plus an unmistakable visible banner.
	// This added banner is NOT part of the canonical raw CLI artifact.
	items := []struct {
		name string
		data []byte
	}{
		{"SYNTHETIC-baseline.json", baseJSON},
		{"SYNTHETIC-baseline.html", withSyntheticBanner(baseHTML)},
		{"SYNTHETIC-hardened.json", hardJSON},
		{"SYNTHETIC-hardened.html", withSyntheticBanner(hardHTML)},
		{"SYNTHETIC-comparison.json", comparisonJSON},
		{"SYNTHETIC-comparison.html", withSyntheticBanner(comparisonHTML)},
	}
	var sums strings.Builder
	for _, item := range items {
		if err := exclusiveWrite(filepath.Join(out, item.name), item.data); err != nil {
			return err
		}
		hash := sha256.Sum256(item.data)
		fmt.Fprintf(&sums, "%x  %s\n", hash, item.name)
	}
	if err := exclusiveWrite(filepath.Join(out, "SHA256SUMS-SYNTHETIC.txt"), []byte(sums.String())); err != nil {
		return err
	}
	readme := `FIREDOOR v0.8 — SYNTHETIC REVIEW, NOT REAL SECURITY EVIDENCE
================================================================

` + syntheticWarning + `

` + `Open SYNTHETIC-comparison.html first; then the baseline and hardened HTML pages.
All source IDs, digest inputs, capture timestamps, and evidence gaps are invented for illustration.
The real Firedoor offline comparison CLI was exercised in a disposable Git repository
containing only these synthetic source documents; its JSON and HTML output were validated.
The HTML files in this review folder add a conspicuous synthetic warning banner.
The sample individual Boundary Proofs were rendered directly from the reporting contract,
NOT produced by the historical v0.5 single-proof exporter.

Interpretation: different Terraform digests = different illustrative artifacts, NOT
infrastructure drift. Matching network digests = matching illustrative strings,
NOT equal traffic or enforced policy. Kubernetes observations are missing.
Verdict INCONCLUSIVE; drift NOT ASSESSED. No live Kubernetes, Terraform,
network scans, source authentication or verified containment occurred.

SHA256SUMS-SYNTHETIC.txt checks these demo files' current bytes only; it is not
an authenticated signature or evidence of a real experiment.
Never present this material as real infrastructure evidence or an attestation.
Private Firedoor evidence files were not read or copied.
`
	if err := exclusiveWrite(filepath.Join(out, "README-SYNTHETIC.txt"), []byte(readme)); err != nil {
		return err
	}
	complete = true
	return nil
}

func syntheticDocuments() (reporting.Document, reporting.Document, error) {
	digest := func(text string) string {
		sum := sha256.Sum256([]byte("FIREDOOR-SYNTHETIC-ILLUSTRATION:" + text))
		return hex.EncodeToString(sum[:])
	}
	layer := func(ref, label, when string) reporting.Layer {
		return reporting.Layer{Status: reporting.StatusRecorded, Reference: ref, SHA256: digest(label), CapturedAtUTC: when}
	}
	baseline, err := reporting.NewDocument("BP-SYNTHETIC-BASE-004", "FD-004", "baseline", "edge-to-data",
		layer("synthetic-plan-base", "terraform-plan-base", "2026-11-01T12:00:00Z"),
		reporting.Layer{Status: reporting.StatusMissing},
		layer("synthetic-historical-witness", "unrelated-shared-historical-observation", "2026-10-31T12:00:00Z"))
	if err != nil {
		return reporting.Document{}, reporting.Document{}, err
	}
	hardened, err := reporting.NewDocument("BP-SYNTHETIC-HARD-004", "FD-004", "hardened", "edge-to-data",
		layer("synthetic-plan-hard", "terraform-plan-hard", "2026-11-02T12:00:00Z"),
		reporting.Layer{Status: reporting.StatusMissing},
		layer("synthetic-historical-witness", "unrelated-shared-historical-observation", "2026-10-31T12:00:00Z"))
	return baseline, hardened, err
}

func exclusiveWrite(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	n, writeErr := f.Write(data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func withSyntheticBanner(html []byte) []byte {
	const opening = "<body>"
	const banner = `<body><div role="note" style="position:sticky;top:0;z-index:999;background:#7c2d12;color:#fff;padding:14px 20px;font:700 15px system-ui,Arial,sans-serif;text-align:center;border-bottom:4px solid #fbbf24">` + syntheticWarning + `</div>`
	return bytes.Replace(html, []byte(opening), []byte(banner), 1)
}
