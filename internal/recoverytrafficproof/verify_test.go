package recoverytrafficproof

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func valid() Observation {
	ips := map[string]string{"edge": "10.96.0.10", "app": "10.96.0.11", "data": "10.96.0.12"}
	o := Observation{SchemaVersion: Schema, Cluster: Cluster, Context: "kind-" + Cluster,
		F4AuditPassed: true, ApplyResultSHA256: ApplyResultSHA, TerraformStateHashVerified: true,
		IngressPolicyCount: 2, NoKubernetesObjectWrites: true, NoTerraformExecution: true}
	for _, id := range []string{"edge-to-app", "app-to-data", "data-to-edge-control", "edge-to-data", "data-to-app"} {
		c := cases[id]
		p := Probe{ID: id, SourceZone: c.source, DestinationZone: c.dest, Expected: c.expectation,
			ServiceIP: ips[c.dest], SourcePodReady: true, DestinationServiceReady: true, TransportPrecheckOK: true}
		if c.expectation == "allow" {
			p.ResponseBody = c.body
		} else {
			p.WgetExitCode = 1
			p.KubectlExitCode = 1
		}
		o.Probes = append(o.Probes, p)
	}
	return o
}

func TestAcceptFiveProbes(t *testing.T) {
	r, err := Evaluate(valid())
	if err != nil || !r.NetworkTrafficEnforcementVerified || r.AllowedVerified != 3 || r.DeniedVerified != 2 || r.RecoveryProven {
		t.Fatalf("unexpected: %+v %v", r, err)
	}
}
func TestRejectNegativeCases(t *testing.T) {
	cases := map[string]func(*Observation){
		"wrong context":          func(o *Observation) { o.Context = "kind-firedoor-local" },
		"wrong cluster":          func(o *Observation) { o.Cluster = "firedoor-local" },
		"plan mismatch":          func(o *Observation) { o.ApplyResultSHA256 = "00" },
		"not audited":            func(o *Observation) { o.F4AuditPassed = false },
		"no state verification":  func(o *Observation) { o.TerraformStateHashVerified = false },
		"policy missing":         func(o *Observation) { o.IngressPolicyCount = 1 },
		"object mutation":        func(o *Observation) { o.NoKubernetesObjectWrites = false },
		"terraform run":          func(o *Observation) { o.NoTerraformExecution = false },
		"probe missing":          func(o *Observation) { o.Probes = o.Probes[:4] },
		"duplicate":              func(o *Observation) { o.Probes[4] = o.Probes[3] },
		"wrong source":           func(o *Observation) { o.Probes[3].SourceZone = "app" },
		"wrong expectation":      func(o *Observation) { o.Probes[3].Expected = "allow" },
		"not-ready":              func(o *Observation) { o.Probes[0].SourcePodReady = false },
		"kubectl transport fail": func(o *Observation) { o.Probes[3].TransportPrecheckOK = false },
		"deny wrong kubectl rc":  func(o *Observation) { o.Probes[3].KubectlExitCode = 2 },
		"allow kubectl failed":   func(o *Observation) { o.Probes[0].KubectlExitCode = 1 },
		"allow timeout":          func(o *Observation) { o.Probes[0].WgetExitCode = 1 },
		"allow wrong body":       func(o *Observation) { o.Probes[0].ResponseBody = "WRONG" },
		"deny succeeded":         func(o *Observation) { o.Probes[3].WgetExitCode = 0 },
		"deny bad response":      func(o *Observation) { o.Probes[3].ResponseBody = "FAIL" },
		"deny missing wget":      func(o *Observation) { o.Probes[3].WgetExitCode = 127 },
		"deny wrong service":     func(o *Observation) { o.Probes[3].ServiceIP = "10.96.0.99" },
		"bad IP":                 func(o *Observation) { o.Probes[3].ServiceIP = "127.0.0.1" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			o := valid()
			change(&o)
			if _, err := Evaluate(o); err == nil {
				t.Fatal("unsafe observation accepted")
			}
		})
	}
}
func TestDecoderBOMAndStrict(t *testing.T) {
	b, err := json.Marshal(valid())
	if err != nil {
		t.Fatal(err)
	}
	good := append([]byte{0xef, 0xbb, 0xbf}, b...)
	o, err := Decode(bytes.NewReader(good))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Evaluate(o); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{
		"double BOM": append([]byte{0xef, 0xbb, 0xbf}, good...),
		"unknown":    bytes.Replace(b, []byte(`"context":`), []byte(`"hijack":true,"context":`), 1),
		"trailing":   append(b, []byte(` {}`)...),
		"empty":      []byte{},
		"malformed":  []byte(`{`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode(bytes.NewReader(body)); err == nil {
				t.Fatal("accepted invalid JSON")
			}
		})
	}
	big := strings.Repeat("x", (1<<20)+1)
	if _, err := Decode(strings.NewReader(big)); err == nil {
		t.Fatal("accepted oversize JSON")
	}
}
