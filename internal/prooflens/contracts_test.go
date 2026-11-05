package prooflens

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sealedFixture(t *testing.T) Bundle {
	t.Helper()
	b, err := Seal(SyntheticExample())
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestContractRoundTripAndHonestVerdict(t *testing.T) {
	b := sealedFixture(t)
	r := VerifyBundle(b)
	if r.Integrity != IntegrityValid || r.ProofVerdict != Inconclusive || len(r.Problems) != 0 || r.EvidenceCount != 3 {
		t.Fatalf("unexpected validation: %+v", r)
	}
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeBundle(data)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.BundleSHA256 != b.BundleSHA256 || VerifyBundle(decoded).Integrity != IntegrityValid {
		t.Fatal("JSON round trip lost integrity")
	}
	b2, err := Seal(SyntheticExample())
	if err != nil || b2.BundleSHA256 != b.BundleSHA256 {
		t.Fatal("expected deterministic bundle digest")
	}
}

func TestIntegrityTamperingIsInconclusive(t *testing.T) {
	tests := map[string]func(*Bundle){
		"changed Terraform source digest": func(b *Bundle) { b.Evidence[0].Source.SHA256 = syntheticHash("altered") },
		"changed test observation":        func(b *Bundle) { b.Evidence[2].Observed = "reachable" },
		"changed run profile":             func(b *Bundle) { b.Run.Profile = "hardened" },
		"changed claim":                   func(b *Bundle) { b.Proof.Claim = "Verified production isolation" },
		"changed bundle checksum":         func(b *Bundle) { b.BundleSHA256 = strings.Repeat("0", 64) },
		"missing evidence checksum":       func(b *Bundle) { b.Evidence[0].ArtifactSHA256 = "" },
		"wrong proof stage reference":     func(b *Bundle) { b.Proof.DeclaredEvidenceID = b.Proof.TestedEvidenceID },
		"wrong run evidence reference":    func(b *Bundle) { b.Run.EvidenceIDs[0] = "EV-UNKNOWN" },
		"mismatched run ID":               func(b *Bundle) { b.Evidence[0].RunID = "RUN-OTHER" },
		"duplicate evidence ID":           func(b *Bundle) { b.Evidence[1].EvidenceID = b.Evidence[0].EvidenceID },
		"duplicated stage":                func(b *Bundle) { b.Evidence[1].Stage = Declared },
		"unsupported schema version":      func(b *Bundle) { b.SchemaVersion = "firedoor.v05.proof-bundle.v2" },
		"unjustified supported verdict":   func(b *Bundle) { b.Proof.Verdict = "supported" },
		"missing limitations":             func(b *Bundle) { b.Proof.Limitations = nil },
		"unknown stage":                   func(b *Bundle) { b.Evidence[0].Stage = "other" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			b := sealedFixture(t)
			mutate(&b)
			r := VerifyBundle(b)
			if r.Integrity != Inconclusive || r.ProofVerdict != Inconclusive || len(r.Problems) == 0 {
				t.Fatalf("failed to reject mutation: %+v", r)
			}
		})
	}
}

func TestSealDoesNotUpgradeSecurityVerdict(t *testing.T) {
	b := SyntheticExample()
	b.Run.Verdict = "supported"
	if _, err := Seal(b); err == nil {
		t.Fatal("must reject run claiming proof before semantic verification")
	}
	b = SyntheticExample()
	b.Proof.Verdict = "supported"
	if _, err := Seal(b); err == nil {
		t.Fatal("must reject proof claiming support before semantic verification")
	}
}

func TestStrictBoundedDecodeAndFiles(t *testing.T) {
	b := sealedFixture(t)
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	for name, input := range map[string][]byte{
		"unknown JSON field": []byte(strings.Replace(string(data), `"bundle_sha256":`, `"unexpected":true,"bundle_sha256":`, 1)),
		"trailing value":     append(append([]byte{}, data...), []byte(` {}`)...),
		"too large":          bytes.Repeat([]byte{'A'}, MaxBundleBytes+1),
		"malformed":          []byte(`{"run":`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeBundle(input); err == nil {
				t.Fatal("malformed JSON unexpectedly accepted")
			}
		})
	}
	bom := append([]byte{0xEF, 0xBB, 0xBF}, data...)
	if _, err := DecodeBundle(bom); err != nil {
		t.Fatalf("PowerShell 5.1 UTF-8 BOM should work: %v", err)
	}
	path := filepath.Join(t.TempDir(), "fixture.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadBundle(path)
	if err != nil || VerifyBundle(loaded).Integrity != IntegrityValid {
		t.Fatalf("safe local read failed: %v", err)
	}
	if _, err := LoadBundle(t.TempDir()); err == nil {
		t.Fatal("directory is not a regular evidence file")
	}
}

func TestSourceHashIsNotAuthentication(t *testing.T) {
	b := sealedFixture(t)
	b.Evidence[2].Observed = "reachable"
	// A fully empowered attacker may rewrite the file and recompute hashes.
	// The result remains structurally valid, with an INCONCLUSIVE proof verdict.
	resigned, err := Seal(b)
	if err != nil {
		t.Fatal(err)
	}
	r := VerifyBundle(resigned)
	if r.Integrity != IntegrityValid || r.ProofVerdict != Inconclusive {
		t.Fatalf("integrity must not imply verified enforcement: %+v", r)
	}
}
