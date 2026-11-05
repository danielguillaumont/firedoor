package prooflens

import (
	"reflect"
	"testing"
	"time"

	"github.com/danielguillaumont/firedoor/internal/attribution"
)

func fixturePreflight() attribution.PreflightReport {
	plan := attribution.ApprovedPlan()
	return attribution.PreflightReport{
		Schema: "firedoor.v04.policy-preflight.v1", Cluster: attribution.OwnedCluster,
		Context: attribution.OwnedContext, Profile: attribution.RequiredProfile,
		Status: "admissible_for_later_review", CapturedAtUTC: time.Date(2026, 11, 5, 17, 27, 0, 0, time.UTC),
		PlannedExperiments: []string{plan.Experiments[0].ID, plan.Experiments[1].ID},
		Namespaces: []attribution.PreflightNamespace{
			{Name: "firedoor-edge", PodReady: true, PolicyCount: 0},
			{Name: "firedoor-app", PodReady: true, PolicyCount: 1, ExpectedPolicy: "firedoor-app-ingress", SourceNamespace: "firedoor-edge"},
			{Name: "firedoor-data", PodReady: true, PolicyCount: 1, ExpectedPolicy: "firedoor-data-ingress", SourceNamespace: "firedoor-app"},
		},
	}
}

func fixtureTerraformSourceManifest(t *testing.T) TerraformSourceManifest {
	t.Helper()
	files := []TerraformSourceFile{
		{Path: "infrastructure/terraform/modules/network-boundary/main.tf", Bytes: 1, SHA256: "1111111111111111111111111111111111111111111111111111111111111111"},
		{Path: "infrastructure/terraform/modules/workload/main.tf", Bytes: 1, SHA256: "2222222222222222222222222222222222222222222222222222222222222222"},
		{Path: "infrastructure/terraform/testbed/main.tf", Bytes: 1, SHA256: "3333333333333333333333333333333333333333333333333333333333333333"},
	}
	src := TerraformSourceManifest{SchemaVersion: TerraformSourceSchema, Mode: TerraformSourceOnly,
		CapturedUTC: "2026-11-05T17:26:00Z", Files: files, FileCount: len(files), Limitations: terraformLimitations()}
	digest, err := terraformTreeDigest(src.Files)
	if err != nil {
		t.Fatal(err)
	}
	src.TreeSHA256 = digest
	if err := VerifyTerraformSourceManifest(src); err != nil {
		t.Fatal(err)
	}
	return src
}

func TestKubernetesObservationShapeDigestAndNoProofUpgrade(t *testing.T) {
	src := fixtureTerraformSourceManifest(t)
	o, err := BuildKubernetesObservation(fixturePreflight(), src)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyKubernetesObservation(o); err != nil {
		t.Fatal(err)
	}
	if o.ProofVerdict != Inconclusive || o.ReceiptCorrelation != KubernetesUnlinkedStatus {
		t.Fatal("unexpected proof upgrade or receipt join")
	}
	if o.PolicyCount != 2 || o.ReadyPodCount != 3 {
		t.Fatal("bad counts")
	}
	if o.ObservationSHA256 == "" {
		t.Fatal("missing checksum")
	}

	changed := o
	changed.ProofVerdict = "supported"
	if VerifyKubernetesObservation(changed) == nil {
		t.Fatal("proof upgrade accepted")
	}
	changed = o
	changed.Namespaces = append([]ObservedNamespace(nil), o.Namespaces...)
	changed.Namespaces[2].PolicyCount = 2
	if VerifyKubernetesObservation(changed) == nil {
		t.Fatal("changed policy count accepted")
	}
	changed = o
	changed.ObservationSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if VerifyKubernetesObservation(changed) == nil {
		t.Fatal("tamper accepted")
	}
}

func TestKubernetesObservationRejectsWrongPreflight(t *testing.T) {
	src := fixtureTerraformSourceManifest(t)
	cases := map[string]func(*attribution.PreflightReport){
		"wrong context":     func(p *attribution.PreflightReport) { p.Context = "production" },
		"wrong profile":     func(p *attribution.PreflightReport) { p.Profile = "open" },
		"unready pod":       func(p *attribution.PreflightReport) { p.Namespaces[1].PodReady = false },
		"extra policy":      func(p *attribution.PreflightReport) { p.Namespaces[0].PolicyCount = 1 },
		"widened source":    func(p *attribution.PreflightReport) { p.Namespaces[2].SourceNamespace = "firedoor-edge" },
		"wrong experiments": func(p *attribution.PreflightReport) { p.PlannedExperiments = []string{"fake"} },
		"missing namespace": func(p *attribution.PreflightReport) { p.Namespaces = p.Namespaces[:2] },
		"wrong order": func(p *attribution.PreflightReport) {
			p.Namespaces[0], p.Namespaces[1] = p.Namespaces[1], p.Namespaces[0]
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := fixturePreflight()
			mutate(&p)
			if _, err := BuildKubernetesObservation(p, src); err == nil {
				t.Fatal("unsafe preflight accepted")
			}
		})
	}
	if reflect.DeepEqual(fixturePreflight().Namespaces, []attribution.PreflightNamespace{}) {
		t.Fatal("missing fixture")
	}
}
