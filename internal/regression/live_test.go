package regression

import (
	"encoding/json"
	"strings"
	"testing"
)

func validLive() LiveObservation {
	cluster := "firedoor-v09-012345abcdef"
	return LiveObservation{
		SchemaVersion: LiveSchemaVersion, Source: "guarded_kind_calico_probe", RunID: cluster,
		Cluster: cluster, Context: "kind-" + cluster, Namespace: "firedoor-v09c1",
		PolicyName: "allow-trusted-to-data", PolicyUID: "123e4567-e89b-12d3-a456-426614174000",
		Phase: "protected", PolicyMode: "trusted_only", RouteID: "edge-to-data",
		Target: "service", Transport: "HTTP/TCP", Port: 8080,
		Observed: "unreachable", TrustedHealthy: true, EdgeExecHealthy: true,
		CNIReady: true, PolicyVerified: true,
	}
}

func TestLiveVerdicts(t *testing.T) {
	cases := []struct {
		name, phase, mode, observed string
		healthy                     bool
		want                        string
	}{
		{"protected", "protected", "trusted_only", "unreachable", true, "MATCH"},
		{"protected-breach", "protected", "trusted_only", "reachable", true, "REGRESSION"},
		{"weakened-breach", "weakened", "all_namespace_pods", "reachable", true, "REGRESSION"},
		{"restored", "restored", "trusted_only", "unreachable", true, "MATCH"},
		{"weak-but-unreachable", "weakened", "all_namespace_pods", "unreachable", true, "INCONCLUSIVE"},
		{"missing-observation", "protected", "trusted_only", "not_observed", true, "INCONCLUSIVE"},
		{"failed-positive", "protected", "trusted_only", "unreachable", false, "INCONCLUSIVE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := validLive()
			o.Phase = tc.phase
			o.PolicyMode = tc.mode
			o.Observed = tc.observed
			o.TrustedHealthy = tc.healthy
			out, err := EvaluateLive(o)
			if err != nil {
				t.Fatal(err)
			}
			if out.Verdict != tc.want {
				t.Fatalf("got %s, want %s", out.Verdict, tc.want)
			}
		})
	}
}

func TestLiveFailClosedSignals(t *testing.T) {
	cases := []func(*LiveObservation){
		func(o *LiveObservation) { o.TrustedHealthy = false },
		func(o *LiveObservation) { o.EdgeExecHealthy = false },
		func(o *LiveObservation) { o.CNIReady = false },
		func(o *LiveObservation) { o.PolicyVerified = false },
	}
	for i, mutate := range cases {
		o := validLive()
		mutate(&o)
		r, err := EvaluateLive(o)
		if err != nil || r.Verdict != "INCONCLUSIVE" {
			t.Fatalf("signal %d: %+v %v", i, r, err)
		}
	}
}

func TestLiveIdentityAndPhaseRejection(t *testing.T) {
	cases := []func(*LiveObservation){
		func(o *LiveObservation) { o.Source = "synthetic_fixture" },
		func(o *LiveObservation) { o.SchemaVersion = SchemaVersion },
		func(o *LiveObservation) { o.Cluster = "firedoor-local" },
		func(o *LiveObservation) { o.RunID = "another-run" },
		func(o *LiveObservation) { o.Context = "kind-firedoor-local" },
		func(o *LiveObservation) { o.Namespace = "default" },
		func(o *LiveObservation) { o.PolicyUID = "" },
		func(o *LiveObservation) { o.PolicyName = "other" },
		func(o *LiveObservation) { o.Phase = "unknown" },
		func(o *LiveObservation) { o.Phase = "weakened" },
		func(o *LiveObservation) { o.Target = "external" },
		func(o *LiveObservation) { o.Observed = "blocked" },
		func(o *LiveObservation) { o.Port = 80 },
	}
	for i, mutate := range cases {
		o := validLive()
		mutate(&o)
		if _, err := EvaluateLive(o); err == nil {
			t.Fatalf("accepted illegal change %d", i)
		}
	}
}

func TestStrictLiveDecode(t *testing.T) {
	o := validLive()
	b, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := DecodeLive(b)
	if err != nil || parsed.Cluster != o.Cluster {
		t.Fatalf("valid decode: %v", err)
	}
	failures := [][]byte{
		nil, []byte("[]"), []byte("{}"), []byte(`{"schema_version":"x"}`),
		append(append([]byte{}, b...), []byte("{}")...),
		[]byte(strings.Repeat(" ", MaxLiveObservationBytes+1)),
		[]byte(strings.Replace(string(b), `"target":"service"`, `"target":"invalid"`, 1)),
		[]byte(strings.Replace(string(b), `"port":8080`, `"port":"8080"`, 1)),
		[]byte(strings.Replace(string(b), `"policy_uid":"123e4567-e89b-12d3-a456-426614174000"`, `"policy_uid":null`, 1)),
		[]byte(strings.Replace(string(b), `"route_id":`, `"extra":true,"route_id":`, 1)),
		[]byte(strings.Replace(string(b), `"route_id":`, `"port":8080,"route_id":`, 1)),
	}
	for i, bad := range failures {
		if _, err := DecodeLive(bad); err == nil {
			t.Errorf("accepted malformed input %d", i)
		}
	}
}
