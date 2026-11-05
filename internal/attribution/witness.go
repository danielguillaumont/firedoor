package attribution

// Compile-time constrained temporary NetworkPolicy manifests for v0.4.
// This package only renders and validates JSON. It cannot create or delete
// Kubernetes objects. A later runtime must separately prove ownership,
// acquire explicit operator approval, and guarantee cleanup.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
)

const WitnessPreviewSchema = "firedoor.v04.witness-preview.v1"

// Kubernetes types are intentionally minimal, making broadening detectable.
// An ingress peer has BOTH namespaceSelector AND podSelector as one entry.
type WitnessObject struct {
	APIVersion string          `json:"apiVersion"`
	Kind       string          `json:"kind"`
	Metadata   WitnessMetadata `json:"metadata"`
	Spec       WitnessPolicy   `json:"spec"`
}

type WitnessMetadata struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels"`
}

type WitnessPolicy struct {
	PodSelector WitnessSelector `json:"podSelector"`
	PolicyTypes []string        `json:"policyTypes"`
	Ingress     []WitnessRule   `json:"ingress"`
}

type WitnessSelector struct {
	MatchLabels map[string]string `json:"matchLabels"`
}

type WitnessRule struct {
	From  []WitnessPeer `json:"from"`
	Ports []WitnessPort `json:"ports"`
}

type WitnessPeer struct {
	NamespaceSelector WitnessSelector `json:"namespaceSelector"`
	PodSelector       WitnessSelector `json:"podSelector"`
}

type WitnessPort struct {
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
}

type WitnessPreview struct {
	ExperimentID         string        `json:"experiment_id"`
	RouteID              string        `json:"route_id"`
	ManifestSHA256       string        `json:"manifest_sha256"`
	Manifest             WitnessObject `json:"manifest"`
	SafetyInterpretation string        `json:"safety_interpretation"`
}

type PreviewBundle struct {
	Schema             string           `json:"schema"`
	Cluster            string           `json:"cluster"`
	Context            string           `json:"context"`
	Profile            string           `json:"profile"`
	Mode               string           `json:"mode"`
	MutationsPerformed bool             `json:"mutations_performed"`
	Witnesses          []WitnessPreview `json:"witnesses"`
	Limitations        []string         `json:"limitations"`
}

// ManifestFor constructs only the exact ingress allow that was already
// approved by the immutable plan; arbitrary experiments are rejected.
func ManifestFor(ex Experiment) (WitnessObject, error) {
	plan := ApprovedPlan()
	if err := Validate(plan); err != nil {
		return WitnessObject{}, err
	}
	approved := false
	for _, expected := range plan.Experiments {
		if reflect.DeepEqual(ex, expected) {
			approved = true
			break
		}
	}
	if !approved {
		return WitnessObject{}, errors.New("witness experiment is not an exact approved Firedoor contract")
	}
	manifest := WitnessObject{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata: WitnessMetadata{
			Name: ex.Witness.PolicyName, Namespace: ex.Witness.PolicyNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of":  "firedoor",
				"firedoor.dev/managed-by":    "firedoor-v04",
				"firedoor.dev/temporary":     "true",
				"firedoor.dev/experiment-id": ex.ID,
			},
		},
		Spec: WitnessPolicy{
			PodSelector: WitnessSelector{MatchLabels: map[string]string{
				ex.Witness.TargetPodSelectorKey: ex.Target.AppName,
			}},
			PolicyTypes: []string{"Ingress"},
			Ingress: []WitnessRule{{
				From: []WitnessPeer{{
					NamespaceSelector: WitnessSelector{MatchLabels: map[string]string{
						ex.Witness.NamespaceSelectorKey: ex.Source.Namespace,
					}},
					PodSelector: WitnessSelector{MatchLabels: map[string]string{
						ex.Witness.SourcePodSelectorKey: ex.Source.AppName,
					}},
				}},
				Ports: []WitnessPort{{Protocol: ex.Witness.Protocol, Port: ex.Witness.TargetPort}},
			}},
		},
	}
	if err := ValidateWitness(ex, manifest); err != nil {
		return WitnessObject{}, err
	}
	return manifest, nil
}

// ValidateWitness rejects a structurally widened manifest. Strictly typed
// equality catches extra peers, rules, ports, labels, and non-approved routes.
func ValidateWitness(ex Experiment, manifest WitnessObject) error {
	approved := ApprovedPlan()
	if err := Validate(approved); err != nil {
		return err
	}
	var expected *Experiment
	for i := range approved.Experiments {
		if reflect.DeepEqual(ex, approved.Experiments[i]) {
			expected = &approved.Experiments[i]
			break
		}
	}
	if expected == nil {
		return errors.New("unapproved experiment for witness validation")
	}
	// Build the unique expected object independently of the input; avoid
	// recursive ManifestFor calls and ensure all fields are exact.
	want := WitnessObject{
		APIVersion: "networking.k8s.io/v1", Kind: "NetworkPolicy",
		Metadata: WitnessMetadata{
			Name:      expected.Witness.PolicyName,
			Namespace: expected.Witness.PolicyNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of":  "firedoor",
				"firedoor.dev/managed-by":    "firedoor-v04",
				"firedoor.dev/temporary":     "true",
				"firedoor.dev/experiment-id": expected.ID,
			},
		},
		Spec: WitnessPolicy{
			PodSelector: WitnessSelector{MatchLabels: map[string]string{
				expected.Witness.TargetPodSelectorKey: expected.Target.AppName,
			}},
			PolicyTypes: []string{"Ingress"},
			Ingress: []WitnessRule{{
				From: []WitnessPeer{{
					NamespaceSelector: WitnessSelector{MatchLabels: map[string]string{
						expected.Witness.NamespaceSelectorKey: expected.Source.Namespace,
					}},
					PodSelector: WitnessSelector{MatchLabels: map[string]string{
						expected.Witness.SourcePodSelectorKey: expected.Source.AppName,
					}},
				}},
				Ports: []WitnessPort{{Protocol: "TCP", Port: 8080}},
			}},
		},
	}
	if !reflect.DeepEqual(manifest, want) {
		return fmt.Errorf("witness manifest differs from immutable %s allow policy", expected.ID)
	}
	return nil
}

func NewWitnessPreview() (PreviewBundle, error) {
	plan := ApprovedPlan()
	if err := Validate(plan); err != nil {
		return PreviewBundle{}, err
	}
	out := PreviewBundle{
		Schema: WitnessPreviewSchema, Cluster: OwnedCluster, Context: OwnedContext,
		Profile: RequiredProfile, Mode: "preview_only", MutationsPerformed: false,
		Limitations: []string{
			"Manifest preview is not permission to create or delete a policy.",
			"A fresh live read-only preflight must pass before any future mutation.",
			"A temporary ingress allow cannot override a source-side egress denial.",
			"Later runtime must verify witness UID, A1/B/A2 traffic, bounded delays and verified cleanup; unexpected changes must fail closed.",
		},
	}
	for _, ex := range plan.Experiments {
		obj, err := ManifestFor(ex)
		if err != nil {
			return PreviewBundle{}, err
		}
		b, err := json.Marshal(obj)
		if err != nil {
			return PreviewBundle{}, err
		}
		sum := sha256.Sum256(b)
		out.Witnesses = append(out.Witnesses, WitnessPreview{
			ExperimentID: ex.ID, RouteID: ex.RouteID,
			ManifestSHA256: hex.EncodeToString(sum[:]), Manifest: obj,
			SafetyInterpretation: "one destination Pod label; one source Pod label in one named namespace; ingress TCP/8080 only",
		})
	}
	if err := ValidateWitnessPreview(out); err != nil {
		return PreviewBundle{}, err
	}
	return out, nil
}

// ValidateWitnessPreview also rejects swapped or modified manifest listings.
func ValidateWitnessPreview(bundle PreviewBundle) error {
	if bundle.Schema != WitnessPreviewSchema || bundle.Cluster != OwnedCluster ||
		bundle.Context != OwnedContext || bundle.Profile != RequiredProfile ||
		bundle.Mode != "preview_only" || bundle.MutationsPerformed || len(bundle.Witnesses) != 2 {
		return errors.New("invalid witness preview header or witness count")
	}
	approved := ApprovedPlan()
	for i, entry := range bundle.Witnesses {
		ex := approved.Experiments[i]
		if entry.ExperimentID != ex.ID || entry.RouteID != ex.RouteID || entry.SafetyInterpretation == "" {
			return fmt.Errorf("witness preview entry %d identity mismatch", i)
		}
		if err := ValidateWitness(ex, entry.Manifest); err != nil {
			return err
		}
		b, err := json.Marshal(entry.Manifest)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		if entry.ManifestSHA256 != hex.EncodeToString(sum[:]) {
			return fmt.Errorf("witness preview entry %d manifest digest mismatch", i)
		}
	}
	return nil
}

// ReadPreview rejects JSON with unknown fields and trailing JSON values.
func ReadPreview(raw []byte) (PreviewBundle, error) {
	var bundle PreviewBundle
	if len(raw) == 0 || len(raw) > 32*1024 {
		return bundle, errors.New("witness preview is empty or oversized")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&bundle); err != nil {
		return PreviewBundle{}, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return PreviewBundle{}, errors.New("trailing or malformed JSON after witness preview")
	}
	if err := ValidateWitnessPreview(bundle); err != nil {
		return PreviewBundle{}, err
	}
	return bundle, nil
}
