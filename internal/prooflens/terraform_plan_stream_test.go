package prooflens

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

const syntheticUILines = `{"type":"version","terraform":"1.17.1","ui":"1.2","@level":"info","@message":"SYNTHETIC_SECRET_DO_NOT_PRINT"}
{"type":"planned_change","change":{"resource":{"addr":"SYNTHETIC_SECRET_DO_NOT_PRINT"},"action":"update"},"@message":"SYNTHETIC_SECRET_DO_NOT_PRINT"}
{"type":"outputs","outputs":{"PASSWORD":{"value":"SYNTHETIC_SECRET_DO_NOT_PRINT","sensitive":true}}}
{"type":"change_summary","changes":{"add":0,"change":1,"remove":0,"operation":"plan"},"@message":"SYNTHETIC_SECRET_DO_NOT_PRINT"}
`

func TestTerraformUISummaryStripsSensitiveFields(t *testing.T) {
	source, err := CollectTerraformSources(tfFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	s, err := SummarizeTerraformUIStream(strings.NewReader(syntheticUILines), source)
	if err != nil {
		t.Fatal(err)
	}
	if s.Changes.Change != 1 || s.EventCount != 4 || s.IgnoredEventCount != 2 || s.ProofVerdict != Inconclusive || !s.PlanSummaryPresent {
		t.Fatalf("unexpected summary: %+v", s)
	}
	if err := VerifyTerraformUIPlanSummary(s); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(s)
	if bytes.Contains(b, []byte("SYNTHETIC_SECRET_DO_NOT_PRINT")) || bytes.Contains(b, []byte("PASSWORD")) || bytes.Contains(b, []byte("planned_change")) {
		t.Fatal("sensitive event content leaked into summary")
	}
	s.Changes.Add = -1
	if VerifyTerraformUIPlanSummary(s) == nil {
		t.Fatal("corrupted summary accepted")
	}
}

func TestTerraformUIStreamFailClosed(t *testing.T) {
	source, err := CollectTerraformSources(tfFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"missing version":   `{"type":"change_summary","changes":{"add":0,"change":0,"remove":0,"operation":"plan"}}` + "\n",
		"missing summary":   `{"type":"version","terraform":"1.17.1","ui":"1.2"}` + "\n",
		"unsupported ui":    strings.Replace(syntheticUILines, `"ui":"1.2"`, `"ui":"2.0"`, 1),
		"malformed":         strings.Replace(syntheticUILines, `"type":"outputs"`, `"type":"outputs",`, 1),
		"duplicate summary": syntheticUILines + `{"type":"change_summary","changes":{"add":0,"change":0,"remove":0,"operation":"plan"}}` + "\n",
		"apply summary":     strings.Replace(syntheticUILines, `"operation":"plan"`, `"operation":"apply"`, 1),
		"apply event":       strings.Replace(syntheticUILines, `"type":"outputs"`, `"type":"apply_start"`, 1),
		"negative counts":   strings.Replace(syntheticUILines, `"change":1`, `"change":-1`, 1),
		"missing counts":    strings.Replace(syntheticUILines, `"change":1,`, ``, 1),
		"error level":       strings.Replace(syntheticUILines, `"@level":"info"`, `"@level":"error"`, 1),
		"diagnostic":        strings.Replace(syntheticUILines, `"type":"outputs"`, `"type":"diagnostic"`, 1),
		"blank line":        syntheticUILines + "\n",
		"giant line":        strings.Repeat("A", maxTerraformUILine+100) + "\n",
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, e := SummarizeTerraformUIStream(strings.NewReader(data), source); e == nil {
				t.Fatal("unsafe stream accepted")
			}
		})
	}
}

type badTerraformStream struct{}

func (badTerraformStream) Read(p []byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestTerraformUIStreamRejectsIOAndChangedSource(t *testing.T) {
	root := tfFixture(t)
	source, err := CollectTerraformSources(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SummarizeTerraformUIStream(badTerraformStream{}, source); err == nil {
		t.Fatal("io error accepted")
	}
	source.TreeSHA256 = strings.Repeat("0", 64)
	if _, err := SummarizeTerraformUIStream(strings.NewReader(syntheticUILines), source); err == nil {
		t.Fatal("invalid source accepted")
	}
}
