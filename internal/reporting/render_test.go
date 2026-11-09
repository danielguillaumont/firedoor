package reporting

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderJSONDeterministicAndValid(t *testing.T) {
	d := fixtureDocument(t)
	first, err := RenderJSON(d)
	if err != nil {
		t.Fatal(err)
	}
	again, err := RenderJSON(d)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, again) || len(first) > MaxJSONBytes {
		t.Fatal("JSON is not bounded/deterministic")
	}
	var got Document
	if err := json.Unmarshal(first, &got); err != nil {
		t.Fatal(err)
	}
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(first, []byte(`"contained"`)) {
		t.Fatal("unsupported verdict")
	}
}

func TestRenderHTMLOfflineAndDeterministic(t *testing.T) {
	d := fixtureDocument(t)
	first, err := RenderHTML(d)
	if err != nil {
		t.Fatal(err)
	}
	again, err := RenderHTML(d)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, again) || len(first) > MaxHTMLBytes {
		t.Fatal("HTML is not bounded/deterministic")
	}
	for _, want := range []string{"INCONCLUSIVE", "Terraform intent", "Kubernetes snapshot", "Network behavior", "Content-Security-Policy", "BP-TEST-001", "Not available", "This detached report"} {
		if !bytes.Contains(first, []byte(want)) {
			t.Fatalf("missing mandatory presentation %q", want)
		}
	}
	for _, forbidden := range []string{"<script", "https://", "http://", "<iframe", "file://", "terraform.tfstate"} {
		if strings.Contains(string(first), forbidden) {
			t.Fatalf("unexpected potentially unsafe content %q", forbidden)
		}
	}
}

func TestEmptyReportRejectedAndNoGeneratedTimestamp(t *testing.T) {
	if _, err := RenderJSON(Document{}); err == nil {
		t.Fatal("missing report accepted")
	}
	if _, err := RenderHTML(Document{}); err == nil {
		t.Fatal("missing report accepted")
	}
	d := fixtureDocument(t)
	data, err := RenderHTML(d)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("2026-11-09")) {
		t.Fatal("renderer generated a run or capture date")
	}
}
