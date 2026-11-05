package attribution

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestWitnessPreviewIsExactAndScoped(t *testing.T) {
	p, err := NewWitnessPreview()
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateWitnessPreview(p); err != nil {
		t.Fatal(err)
	}
	want := []struct{ ns, name, source, sourcePod, targetPod string }{
		{"firedoor-data", "firedoor-v04-witness-edge-data", "firedoor-edge", "firedoor-edge", "firedoor-data"},
		{"firedoor-app", "firedoor-v04-witness-data-app", "firedoor-data", "firedoor-data", "firedoor-app"},
	}
	for i, entry := range p.Witnesses {
		o := entry.Manifest
		w := want[i]
		if o.APIVersion != "networking.k8s.io/v1" || o.Kind != "NetworkPolicy" || o.Metadata.Namespace != w.ns || o.Metadata.Name != w.name {
			t.Fatalf("wrong scoped object %d: %+v", i, o)
		}
		if len(o.Spec.Ingress) != 1 || len(o.Spec.Ingress[0].From) != 1 || len(o.Spec.Ingress[0].Ports) != 1 ||
			!reflect.DeepEqual(o.Spec.PolicyTypes, []string{"Ingress"}) {
			t.Fatalf("unexpected broad policy shape: %+v", o.Spec)
		}
		from := o.Spec.Ingress[0].From[0]
		if !reflect.DeepEqual(from.NamespaceSelector.MatchLabels, map[string]string{"kubernetes.io/metadata.name": w.source}) ||
			!reflect.DeepEqual(from.PodSelector.MatchLabels, map[string]string{"app.kubernetes.io/name": w.sourcePod}) ||
			!reflect.DeepEqual(o.Spec.PodSelector.MatchLabels, map[string]string{"app.kubernetes.io/name": w.targetPod}) ||
			o.Spec.Ingress[0].Ports[0] != (WitnessPort{Protocol: "TCP", Port: 8080}) {
			t.Fatalf("widened selector/port for %s: %+v", w.name, o.Spec)
		}
		if len(o.Metadata.Labels) != 4 || o.Metadata.Labels["firedoor.dev/temporary"] != "true" ||
			o.Metadata.Labels["firedoor.dev/experiment-id"] != entry.ExperimentID {
			t.Fatalf("witness ownership labels invalid: %+v", o.Metadata.Labels)
		}
	}
	bytesJSON, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ReadPreview(bytesJSON)
	if err != nil || !reflect.DeepEqual(decoded, p) {
		t.Fatalf("round trip rejected: %v", err)
	}
}

func TestTamperedWitnessPreviewRejected(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*PreviewBundle)
	}{
		{"different_cluster", func(p *PreviewBundle) { p.Cluster = "prod" }},
		{"different_context", func(p *PreviewBundle) { p.Context = "prod" }},
		{"missing_witness", func(p *PreviewBundle) { p.Witnesses = p.Witnesses[:1] }},
		{"swapped_experiments", func(p *PreviewBundle) { p.Witnesses[0], p.Witnesses[1] = p.Witnesses[1], p.Witnesses[0] }},
		{"wrong_namespace", func(p *PreviewBundle) { p.Witnesses[0].Manifest.Metadata.Namespace = "default" }},
		{"renamed_policy", func(p *PreviewBundle) { p.Witnesses[0].Manifest.Metadata.Name = "foreign" }},
		{"removed_temporary_label", func(p *PreviewBundle) { delete(p.Witnesses[0].Manifest.Metadata.Labels, "firedoor.dev/temporary") }},
		{"widened_target", func(p *PreviewBundle) { p.Witnesses[0].Manifest.Spec.PodSelector.MatchLabels = map[string]string{} }},
		{"widened_namespace", func(p *PreviewBundle) {
			p.Witnesses[0].Manifest.Spec.Ingress[0].From[0].NamespaceSelector.MatchLabels = map[string]string{}
		}},
		{"widened_source_pod", func(p *PreviewBundle) {
			p.Witnesses[0].Manifest.Spec.Ingress[0].From[0].PodSelector.MatchLabels = map[string]string{}
		}},
		{"extra_source_peer", func(p *PreviewBundle) {
			p.Witnesses[0].Manifest.Spec.Ingress[0].From = append(p.Witnesses[0].Manifest.Spec.Ingress[0].From, WitnessPeer{})
		}},
		{"extra_port", func(p *PreviewBundle) {
			p.Witnesses[0].Manifest.Spec.Ingress[0].Ports = append(p.Witnesses[0].Manifest.Spec.Ingress[0].Ports, WitnessPort{Protocol: "TCP", Port: 80})
		}},
		{"wrong_protocol", func(p *PreviewBundle) { p.Witnesses[0].Manifest.Spec.Ingress[0].Ports[0].Protocol = "UDP" }},
		{"wrong_port", func(p *PreviewBundle) { p.Witnesses[0].Manifest.Spec.Ingress[0].Ports[0].Port = 80 }},
		{"more_policy_types", func(p *PreviewBundle) {
			p.Witnesses[0].Manifest.Spec.PolicyTypes = append(p.Witnesses[0].Manifest.Spec.PolicyTypes, "Egress")
		}},
		{"bad_digest", func(p *PreviewBundle) { p.Witnesses[0].ManifestSHA256 = strings.Repeat("0", 64) }},
		{"claim_mutated", func(p *PreviewBundle) { p.MutationsPerformed = true }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewWitnessPreview()
			if err != nil {
				t.Fatal(err)
			}
			tc.modify(&p)
			if err := ValidateWitnessPreview(p); err == nil {
				t.Fatal("unsafe witness change accepted")
			}
		})
	}
}

func TestManifestRejectsNonApprovedExperiment(t *testing.T) {
	p := ApprovedPlan()
	for _, edit := range []func(*Experiment){
		func(e *Experiment) { e.Target.Namespace = "default" },
		func(e *Experiment) { e.Witness.TargetPort = 80 },
		func(e *Experiment) { e.Witness.PolicyName = "anything" },
		func(e *Experiment) { e.Witness.NamespaceSelectorKey = "any" },
	} {
		wrong := p.Experiments[0]
		edit(&wrong)
		if _, err := ManifestFor(wrong); err == nil {
			t.Fatalf("unapproved experiment admitted: %+v", wrong)
		}
	}
}

func TestReadPreviewRejectsMalformedOrUntrustedJSON(t *testing.T) {
	p, _ := NewWitnessPreview()
	valid, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{
		nil,
		[]byte("invalid"),
		append(append([]byte{}, valid...), []byte(` {"another":"object"}`)...),
		append(append([]byte{}, valid...), []byte(` garbage`)...),
		bytes.Replace(valid, []byte(`"mode":"preview_only"`), []byte(`"mode":"preview_only","dangerous":true`), 1),
		bytes.Replace(valid, []byte(`"port":8080`), []byte(`"port":8080,"endPort":65535`), 1),
		bytes.Repeat([]byte("x"), 32*1024+1),
	} {
		if _, err := ReadPreview(raw); err == nil {
			t.Fatalf("untrusted preview accepted: %q", string(raw[:min(len(raw), 45)]))
		}
	}
}
