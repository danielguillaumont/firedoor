package prooflens

// Slice 8C2 reads two separately reviewed, detached Boundary Proof documents
// from fixed ignored local paths. It never creates reports, runs an experiment,
// queries the cluster, or treats recorded provenance as authenticated evidence.
import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/danielguillaumont/firedoor/internal/reporting"
)

const (
	BoundaryPairReadConsent = "I-APPROVE-FIREDOOR-V08-OFFLINE-PROOF-PAIR-READ"
	LocalBaselineProofJSON  = "docs/evidence/local/v08-baseline-boundary-proof.json"
	LocalHardenedProofJSON  = "docs/evidence/local/v08-hardened-boundary-proof.json"
)

var boundaryPairRejected = errors.New("offline Boundary Proof pair rejected; no files were written or evidence collected")

// LoadApprovedBoundaryProofPair reads only two fixed, ignored local inputs
// with explicit consent. Its output is an in-memory, INCONCLUSIVE comparison.
// Structural checks, local SHA-256 fields and separate file identities do NOT
// authenticate either source or establish a real baseline/hardened experiment.
// A reviewer must independently establish provenance before supplying inputs.
func LoadApprovedBoundaryProofPair(root, consent string) (reporting.ComparisonDocument, error) {
	if consent != BoundaryPairReadConsent {
		return reporting.ComparisonDocument{}, boundaryPairRejected
	}
	base, err := boundaryExportRepo(root)
	if err != nil {
		return reporting.ComparisonDocument{}, boundaryPairRejected
	}
	for _, rel := range []string{LocalBaselineProofJSON, LocalHardenedProofJSON} {
		tracked, err := boundaryTracked(base, rel)
		if err != nil || tracked || !GitIgnoredEvidencePath(base, rel) {
			return reporting.ComparisonDocument{}, boundaryPairRejected
		}
	}
	baseline, baselineInfo, err := readLocalBoundaryDocument(base, LocalBaselineProofJSON)
	if err != nil {
		return reporting.ComparisonDocument{}, boundaryPairRejected
	}
	hardened, hardenedInfo, err := readLocalBoundaryDocument(base, LocalHardenedProofJSON)
	if err != nil || os.SameFile(baselineInfo, hardenedInfo) {
		return reporting.ComparisonDocument{}, boundaryPairRejected
	}
	comparison, err := reporting.CompareBoundaryProofs(baseline, hardened)
	if err != nil {
		return reporting.ComparisonDocument{}, boundaryPairRejected
	}
	return comparison, nil
}

// readLocalBoundaryDocument refuses symlinks, directories, oversize files,
// unknown fields, trailing JSON, and noncanonical or forged schema values.
// JSON canonicalization is a formatting check, not a source signature.
func readLocalBoundaryDocument(base, rel string) (reporting.Document, os.FileInfo, error) {
	path := filepath.Join(base, filepath.FromSlash(rel))
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > reporting.MaxJSONBytes {
		return reporting.Document{}, nil, boundaryPairRejected
	}
	file, err := os.Open(path)
	if err != nil {
		return reporting.Document{}, nil, boundaryPairRejected
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) || opened.Size() <= 0 || opened.Size() > reporting.MaxJSONBytes {
		return reporting.Document{}, nil, boundaryPairRejected
	}
	data, err := io.ReadAll(io.LimitReader(file, reporting.MaxJSONBytes+1))
	if err != nil || len(data) > reporting.MaxJSONBytes {
		return reporting.Document{}, nil, boundaryPairRejected
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var doc reporting.Document
	if decoder.Decode(&doc) != nil {
		return reporting.Document{}, nil, boundaryPairRejected
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || doc.Validate() != nil {
		return reporting.Document{}, nil, boundaryPairRejected
	}
	canonical, err := reporting.RenderJSON(doc)
	if err != nil || !bytes.Equal(data, canonical) {
		return reporting.Document{}, nil, boundaryPairRejected
	}
	return doc, opened, nil
}
