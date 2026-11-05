package attribution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

const testWitnessUID = "c61188d0-9161-40d4-a321-25e81a2e009f"

type writeCall struct {
	binary string
	args   []string
	stdin  []byte
}

type fakeWitnessIO struct {
	fixtureReader
	calls     []writeCall
	liveRaw   []byte
	created   bool
	mutated   bool
	writerErr error
}

func newWitnessIO(t *testing.T, ex Experiment) *fakeWitnessIO {
	t.Helper()
	obj, err := ManifestFor(ex)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	var base map[string]any
	if err := json.Unmarshal(b, &base); err != nil {
		t.Fatal(err)
	}
	meta := base["metadata"].(map[string]any)
	meta["uid"] = testWitnessUID
	return &fakeWitnessIO{liveRaw: rawJSON(base)}
}

func (f *fakeWitnessIO) Read(ctx context.Context, bin string, args ...string) ([]byte, error) {
	command := bin + " " + strings.Join(args, " ")
	if strings.Contains(command, " get networkpolicy firedoor-v04-witness-") {
		if !f.created {
			return nil, errors.New("not found")
		}
		return f.liveRaw, nil
	}
	if strings.Contains(command, " get networkpolicies -o json") && f.created {
		return rawJSON(map[string]any{"items": []any{json.RawMessage(f.liveRaw)}}), nil
	}
	return f.fixtureReader.Read(ctx, bin, args...)
}

func (f *fakeWitnessIO) Write(_ context.Context, bin string, input []byte, args ...string) ([]byte, error) {
	f.calls = append(f.calls, writeCall{binary: bin, args: append([]string{}, args...), stdin: append([]byte{}, input...)})
	if f.writerErr != nil {
		return nil, f.writerErr
	}
	line := strings.Join(args, " ")
	if strings.Contains(line, " create --raw=") {
		if f.created {
			return nil, errors.New("AlreadyExists")
		}
		f.created = true
		return f.liveRaw, nil
	}
	if strings.Contains(line, " delete --raw=") {
		var opts struct {
			Preconditions struct {
				UID string `json:"uid"`
			} `json:"preconditions"`
		}
		if err := json.Unmarshal(input, &opts); err != nil {
			return nil, err
		}
		if opts.Preconditions.UID != testWitnessUID {
			return nil, errors.New("UID precondition mismatch")
		}
		f.created = false
		return []byte(`{"kind":"Status","status":"Success"}`), nil
	}
	return nil, fmt.Errorf("unexpected write: %s", line)
}

func TestAdapterSourcePreviewNeverEnablesMutation(t *testing.T) {
	preview, err := NewAdapterPreview()
	if err != nil {
		t.Fatal(err)
	}
	if preview.LiveWritesEnabled || preview.Mode != "SOURCE_ONLY" || len(preview.Operations) != 2 || preview.Cluster != OwnedCluster {
		t.Fatalf("preview incorrect: %+v", preview)
	}
	for _, op := range preview.Operations {
		if !strings.HasPrefix(op.CreatePath, witnessAPIPrefix) || !strings.HasPrefix(op.DeletePath, witnessAPIPrefix) ||
			!strings.Contains(op.Deletion, "UID precondition") {
			t.Fatal("unsafe preview", op)
		}
	}
}

func TestKubeDriverCreateInspectDeleteUsesServerUIDPreconditions(t *testing.T) {
	ex := ApprovedPlan().Experiments[0]
	io := newWitnessIO(t, ex)
	d := NewWitnessKubeDriver(io, io)
	if err := d.Preflight(context.Background()); err != nil {
		t.Fatal(err)
	}
	obj, _ := ManifestFor(ex)
	uid, err := d.Create(context.Background(), ex, obj)
	if err != nil {
		t.Fatal(err)
	}
	if uid != testWitnessUID {
		t.Fatal("server UID not retained")
	}
	if err := d.Inspect(context.Background(), ex, uid); err != nil {
		t.Fatal(err)
	}
	if err := d.Delete(context.Background(), ex, uid); err != nil {
		t.Fatal(err)
	}
	if err := d.VerifyGone(context.Background(), ex); err != nil {
		t.Fatal(err)
	}
	if len(io.calls) != 2 {
		t.Fatalf("unexpected write count: %d", len(io.calls))
	}
	first := io.calls[0]
	if first.binary != "kubectl" || !reflect.DeepEqual(first.args, []string{"--context", OwnedContext, "create", "--raw=/apis/networking.k8s.io/v1/namespaces/firedoor-data/networkpolicies", "-f", "-"}) {
		t.Fatalf("unsafe create call: %#v", first)
	}
	if err := ValidateWitness(ex, func() WitnessObject { var o WitnessObject; _ = json.Unmarshal(first.stdin, &o); return o }()); err != nil {
		t.Fatal(err)
	}
	second := io.calls[1]
	if !reflect.DeepEqual(second.args, []string{"--context", OwnedContext, "delete", "--raw=/apis/networking.k8s.io/v1/namespaces/firedoor-data/networkpolicies/firedoor-v04-witness-edge-data", "-f", "-"}) {
		t.Fatalf("unsafe delete call: %#v", second)
	}
	var opts struct {
		Kind          string `json:"kind"`
		Preconditions struct {
			UID string `json:"uid"`
		} `json:"preconditions"`
	}
	if err := json.Unmarshal(second.stdin, &opts); err != nil || opts.Kind != "DeleteOptions" || opts.Preconditions.UID != uid {
		t.Fatal("delete missing immutable UID precondition")
	}
	// No second create allowed without an additional full read-only preflight.
	if _, err := d.Create(context.Background(), ex, obj); err == nil {
		t.Fatal("repeated creation admitted without preflight")
	}
}

func TestKubeDriverFailsClosedBeforeCreateAndOnMalformedManifest(t *testing.T) {
	ex := ApprovedPlan().Experiments[0]
	io := newWitnessIO(t, ex)
	d := NewWitnessKubeDriver(io, io)
	obj, _ := ManifestFor(ex)
	if _, err := d.Create(context.Background(), ex, obj); err == nil {
		t.Fatal("unadmitted create accepted")
	}
	if len(io.calls) != 0 {
		t.Fatal("write before preflight")
	}
	if err := d.Preflight(context.Background()); err != nil {
		t.Fatal(err)
	}
	obj.Spec.Ingress[0].Ports[0].Port = 0
	if _, err := d.Create(context.Background(), ex, obj); err == nil {
		t.Fatal("malformed manifest accepted")
	}
	if len(io.calls) != 0 {
		t.Fatal("write on malformed manifest")
	}
}

func TestKubeDriverRejectsTamperedPoliciesBeforeDeletion(t *testing.T) {
	ex := ApprovedPlan().Experiments[0]
	for _, tc := range []struct {
		name  string
		alter func(map[string]any)
	}{
		{"wrong_uid", func(v map[string]any) { v["metadata"].(map[string]any)["uid"] = "a61188d0-9161-40d4-a321-25e81a2e009f" }},
		{"widened_source", func(v map[string]any) {
			spec := v["spec"].(map[string]any)
			rule := spec["ingress"].([]any)[0].(map[string]any)
			rule["from"] = []any{map[string]any{}}
		}},
		{"extra_egress", func(v map[string]any) { v["spec"].(map[string]any)["egress"] = []any{map[string]any{}} }},
		{"unknown_spec_key", func(v map[string]any) { v["spec"].(map[string]any)["allowAll"] = true }},
		{"wrong_labels", func(v map[string]any) {
			v["metadata"].(map[string]any)["labels"] = map[string]string{"firedoor.dev/temporary": "true"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			io := newWitnessIO(t, ex)
			var obj map[string]any
			if err := json.Unmarshal(io.liveRaw, &obj); err != nil {
				t.Fatal(err)
			}
			tc.alter(obj)
			io.liveRaw = rawJSON(obj)
			io.created = true
			d := NewWitnessKubeDriver(io, io)
			if err := d.Delete(context.Background(), ex, testWitnessUID); err == nil {
				t.Fatal("tampered resource deleted")
			}
			if len(io.calls) != 0 {
				t.Fatal("delete sent after mismatch")
			}
		})
	}
}

func TestKubeDriverUntrustedCreateReplyLeavesManualRecovery(t *testing.T) {
	ex := ApprovedPlan().Experiments[0]
	io := newWitnessIO(t, ex)
	var obj map[string]any
	_ = json.Unmarshal(io.liveRaw, &obj)
	delete(obj["metadata"].(map[string]any), "uid")
	io.liveRaw = rawJSON(obj)
	d := NewWitnessKubeDriver(io, io)
	if err := d.Preflight(context.Background()); err != nil {
		t.Fatal(err)
	}
	manifest, _ := ManifestFor(ex)
	uid, err := d.Create(context.Background(), ex, manifest)
	if err == nil || uid != "" {
		t.Fatal("invalid UID incorrectly trusted")
	}
	if len(io.calls) != 1 || !io.created {
		t.Fatal("ambiguous create not reflected")
	}
}

func TestKubeDriverRefusesWrongTargetContext(t *testing.T) {
	bad := ApprovedPlan().Experiments[0]
	bad.Witness.PolicyNamespace = "kube-system"
	io := newWitnessIO(t, ApprovedPlan().Experiments[0])
	d := NewWitnessKubeDriver(io, io)
	if err := d.Preflight(context.Background()); err != nil {
		t.Fatal(err)
	}
	obj, _ := ManifestFor(ApprovedPlan().Experiments[0])
	if _, err := d.Create(context.Background(), bad, obj); err == nil {
		t.Fatal("foreign cluster write")
	}
	if len(io.calls) != 0 {
		t.Fatal("unexpected Kubernetes write")
	}
}
