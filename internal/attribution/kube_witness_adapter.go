package attribution

// This file defines the Kubernetes API write adapter for a future, explicitly
// consented A/B/A run. No live entrypoint calls its mutation methods in Slice
// 3B2B1. Safety constraints are enforced again at each operation, not only at
// the beginning of a lifecycle.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
	"time"
)

const (
	witnessAPIPrefix  = "/apis/networking.k8s.io/v1/namespaces/"
	witnessAPISuffix  = "/networkpolicies"
	maxMutationOutput = 1024 * 1024
)

// RawWriter is separate from CommandReader: read-only inspection cannot
// accidentally gain permission to mutate when a driver is used in preview mode.
// Implementations must send stdin unchanged and must NOT invoke a shell.
type RawWriter interface {
	Write(context.Context, string, []byte, ...string) ([]byte, error)
}

type ExecRawWriter struct{}

func (ExecRawWriter) Write(ctx context.Context, binary string, stdin []byte, args ...string) ([]byte, error) {
	if binary != "kubectl" {
		return nil, errors.New("raw writer only accepts kubectl")
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("Kubernetes API write canceled or timed out: %w", ctx.Err())
		}
		return nil, fmt.Errorf("Kubernetes API write failed: %w; stderr: %.500s", err, stderr.String())
	}
	if stdout.Len() == 0 || stdout.Len() > maxMutationOutput {
		return nil, errors.New("empty or oversized Kubernetes write response")
	}
	return stdout.Bytes(), nil
}

// WitnessKubeDriver implements LifecycleDriver. Construct it only in a later
// operator-approved live entrypoint. NewWitnessKubeDriver has real write
// capability; simply constructing it does not make any Kubernetes calls.
type WitnessKubeDriver struct {
	Reader       CommandReader
	Writer       RawWriter
	ProbeAdapter *TrafficProbe
	admitted     bool
	createPermit bool
}

func NewWitnessKubeDriver(reader CommandReader, writer RawWriter) *WitnessKubeDriver {
	return &WitnessKubeDriver{Reader: reader, Writer: writer, ProbeAdapter: &TrafficProbe{Reader: reader}}
}

func (d *WitnessKubeDriver) Preflight(ctx context.Context) error {
	if d == nil || d.Reader == nil {
		return errors.New("missing Kubernetes reader")
	}
	d.admitted = false
	d.createPermit = false
	_, err := RunPreflight(ctx, d.Reader)
	if err != nil {
		return err
	}
	d.admitted = true
	d.createPermit = true
	return nil
}

func (d *WitnessKubeDriver) Probe(ctx context.Context, ex Experiment, phase Phase) (PhaseObservation, error) {
	if d == nil || d.ProbeAdapter == nil || !d.admitted {
		return PhaseObservation{}, errors.New("probe refused before successful preflight")
	}
	if err := approvedExperiment(ex); err != nil {
		return PhaseObservation{}, err
	}
	if phase != Witness && phase != Restored {
		return d.ProbeAdapter.Probe(ctx, ex, phase)
	}
	// Calico policy reconciliation is asynchronous. Constrain convergence to
	// the immutable 30-second settle budget; an unexpected or inconclusive
	// outcome never becomes a policy-effect match.
	settleCtx, cancel := context.WithTimeout(ctx, time.Duration(ApprovedPlan().MaxSettleSeconds)*time.Second)
	defer cancel()
	wanted := Reachable
	if phase == Restored {
		wanted = Unreachable
	}
	var last PhaseObservation
	for {
		got, err := d.ProbeAdapter.Probe(settleCtx, ex, phase)
		if err != nil {
			return got, err
		}
		last = got
		if got.Observed == wanted {
			return got, nil
		}
		select {
		case <-settleCtx.Done():
			return last, nil // lifecycle treats mismatch as failure
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func witnessPath(ex Experiment) (string, error) {
	if err := approvedExperiment(ex); err != nil {
		return "", err
	}
	return witnessAPIPrefix + ex.Witness.PolicyNamespace + witnessAPISuffix + "/" + ex.Witness.PolicyName, nil
}

func witnessCollection(ex Experiment) (string, error) {
	if err := approvedExperiment(ex); err != nil {
		return "", err
	}
	return witnessAPIPrefix + ex.Witness.PolicyNamespace + witnessAPISuffix, nil
}

var uidPattern = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)

func validUID(uid string) bool { return uidPattern.MatchString(uid) }

type liveWitness struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name              string            `json:"name"`
		Namespace         string            `json:"namespace"`
		UID               string            `json:"uid"`
		Labels            map[string]string `json:"labels"`
		DeletionTimestamp string            `json:"deletionTimestamp"`
	} `json:"metadata"`
	Spec struct {
		PodSelector WitnessSelector   `json:"podSelector"`
		PolicyTypes []string          `json:"policyTypes"`
		Ingress     []WitnessRule     `json:"ingress"`
		Egress      []json.RawMessage `json:"egress"`
	} `json:"spec"`
}

func checkLiveWitness(raw []byte, ex Experiment, uid string) error {
	if len(raw) == 0 || len(raw) > maxMutationOutput {
		return errors.New("empty or oversized live NetworkPolicy")
	}
	if !validUID(uid) {
		return errors.New("invalid expected server-assigned UID")
	}
	var observed liveWitness
	if err := json.Unmarshal(raw, &observed); err != nil {
		return fmt.Errorf("invalid Kubernetes witness JSON: %w", err)
	}
	want, err := ManifestFor(ex)
	if err != nil {
		return err
	}
	if observed.APIVersion != want.APIVersion || observed.Kind != want.Kind ||
		observed.Metadata.Name != want.Metadata.Name || observed.Metadata.Namespace != want.Metadata.Namespace ||
		observed.Metadata.UID != uid || observed.Metadata.DeletionTimestamp != "" ||
		!reflect.DeepEqual(observed.Metadata.Labels, want.Metadata.Labels) ||
		len(observed.Spec.Egress) != 0 ||
		!reflect.DeepEqual(observed.Spec.PodSelector, want.Spec.PodSelector) ||
		!reflect.DeepEqual(observed.Spec.PolicyTypes, want.Spec.PolicyTypes) ||
		!reflect.DeepEqual(observed.Spec.Ingress, want.Spec.Ingress) {
		return errors.New("live NetworkPolicy identity, UID, labels, or allow rules differ from fixed witness")
	}
	// Prevent unexpected policy-spec keys from slipping through the minimal
	// typed decoder above. Kubernetes defaulted empty egress is accepted.
	var top struct {
		Spec map[string]json.RawMessage `json:"spec"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		return err
	}
	for k := range top.Spec {
		if k != "podSelector" && k != "policyTypes" && k != "ingress" && k != "egress" {
			return errors.New("unexpected NetworkPolicy spec field")
		}
	}
	return nil
}

func readWitness(ctx context.Context, reader CommandReader, ex Experiment) ([]byte, error) {
	if err := approvedExperiment(ex); err != nil {
		return nil, err
	}
	return mustRead(ctx, reader, "kubectl", "--context", OwnedContext, "-n", ex.Witness.PolicyNamespace,
		"get", "networkpolicy", ex.Witness.PolicyName, "-o", "json")
}

func (d *WitnessKubeDriver) Create(ctx context.Context, ex Experiment, obj WitnessObject) (string, error) {
	if d == nil || d.Reader == nil || d.Writer == nil || !d.admitted || !d.createPermit {
		return "", errors.New("create refused: no verified preflight or missing adapter")
	}
	// Creation consumes the one-time admission. A fresh complete preflight is
	// needed before creating any subsequent witness.
	d.createPermit = false
	if err := ValidateWitness(ex, obj); err != nil {
		return "", err
	}
	path, err := witnessCollection(ex)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	// create-only POST, never apply/patch; fails if the name exists.
	child, cancel := context.WithTimeout(ctx, commandDeadline)
	defer cancel()
	raw, err := d.Writer.Write(child, "kubectl", payload, "--context", OwnedContext, "create", "--raw="+path, "-f", "-")
	if err != nil {
		return "", err
	} // uncertain create -> manual recovery
	var meta struct {
		Metadata struct {
			UID string `json:"uid"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return "", fmt.Errorf("server create returned invalid object: %w", err)
	}
	uid := strings.TrimSpace(meta.Metadata.UID)
	if err := checkLiveWitness(raw, ex, uid); err != nil {
		return uid, err // UID known -> guarded cleanup can still run
	}
	return uid, nil
}

func (d *WitnessKubeDriver) Inspect(ctx context.Context, ex Experiment, uid string) error {
	if d == nil || d.Reader == nil {
		return errors.New("missing Kubernetes reader")
	}
	if !validUID(uid) {
		return errors.New("invalid witness UID")
	}
	raw, err := readWitness(ctx, d.Reader, ex)
	if err != nil {
		return err
	}
	return checkLiveWitness(raw, ex, uid)
}

func (d *WitnessKubeDriver) Delete(ctx context.Context, ex Experiment, uid string) error {
	if d == nil || d.Writer == nil {
		return errors.New("missing Kubernetes write adapter")
	}
	if err := d.Inspect(ctx, ex, uid); err != nil {
		return fmt.Errorf("pre-delete identity inspection rejected: %w", err)
	}
	path, err := witnessPath(ex)
	if err != nil {
		return err
	}
	// Kubernetes DeleteOptions UID preconditions prevent the read/delete race.
	// Standard kubectl delete by name lacks these preconditions.
	options := struct {
		APIVersion    string `json:"apiVersion"`
		Kind          string `json:"kind"`
		Preconditions struct {
			UID string `json:"uid"`
		} `json:"preconditions"`
	}{APIVersion: "v1", Kind: "DeleteOptions"}
	options.Preconditions.UID = uid
	body, err := json.Marshal(options)
	if err != nil {
		return err
	}
	child, cancel := context.WithTimeout(ctx, commandDeadline)
	defer cancel()
	_, err = d.Writer.Write(child, "kubectl", body, "--context", OwnedContext, "delete", "--raw="+path, "-f", "-")
	if err != nil {
		return fmt.Errorf("UID-preconditioned witness deletion rejected: %w", err)
	}
	return nil // lifecycle must STILL independently verify absence
}

func (d *WitnessKubeDriver) VerifyGone(ctx context.Context, ex Experiment) error {
	if d == nil || d.Reader == nil {
		return errors.New("missing Kubernetes reader")
	}
	if err := approvedExperiment(ex); err != nil {
		return err
	}
	// Checking complete namespace policy inventory is stricter than simply
	// checking NotFound for the temporary policy name.
	raw, err := mustRead(ctx, d.Reader, "kubectl", "--context", OwnedContext, "-n", ex.Witness.PolicyNamespace,
		"get", "networkpolicies", "-o", "json")
	if err != nil {
		return err
	}
	_, err = ValidatePolicySnapshot(raw, ex.Witness.PolicyNamespace)
	if err != nil {
		return fmt.Errorf("witness absence or original policy inventory unverified: %w", err)
	}
	return nil
}

// These operation previews are data only. No Kubernetes write operation is
// exposed by the preview executable shipped in this slice.
type WitnessOperationPreview struct {
	ExperimentID string `json:"experiment_id"`
	CreatePath   string `json:"create_path"`
	DeletePath   string `json:"delete_path"`
	Creation     string `json:"creation"`
	Deletion     string `json:"deletion"`
}

type AdapterPreview struct {
	Schema            string                    `json:"schema"`
	Cluster           string                    `json:"cluster"`
	Context           string                    `json:"context"`
	Mode              string                    `json:"mode"`
	LiveWritesEnabled bool                      `json:"live_writes_enabled"`
	Operations        []WitnessOperationPreview `json:"operations"`
	Limitations       []string                  `json:"limitations"`
}

func NewAdapterPreview() (AdapterPreview, error) {
	p := ApprovedPlan()
	if err := Validate(p); err != nil {
		return AdapterPreview{}, err
	}
	out := AdapterPreview{Schema: "firedoor.v04.adapter-preview.v1", Cluster: OwnedCluster, Context: OwnedContext,
		Mode: "SOURCE_ONLY", LiveWritesEnabled: false,
		Limitations: []string{"No live runner is included in this slice.", "UID-preconditioned deletion requires a successful create and inspect of the exact temporary policy.", "A later slice must implement operator consent and crash-recovery guidance before live mutation."},
	}
	for _, ex := range p.Experiments {
		c, err := witnessCollection(ex)
		if err != nil {
			return AdapterPreview{}, err
		}
		d, err := witnessPath(ex)
		if err != nil {
			return AdapterPreview{}, err
		}
		out.Operations = append(out.Operations, WitnessOperationPreview{ExperimentID: ex.ID, CreatePath: c, DeletePath: d,
			Creation: "POST create-only with fixed manifest; reject name collisions", Deletion: "DELETE with server UID precondition; verify original policy inventory afterwards"})
	}
	return out, nil
}
