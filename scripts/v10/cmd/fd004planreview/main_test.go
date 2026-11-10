package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("input failed") }

func TestReadPlanInputOptionalUTF8BOM(t *testing.T) {
	valid := []byte(`{"format_version":"1.0","resource_changes":[]}`)
	withBOM := append(append([]byte{}, utf8BOM...), valid...)
	for name, input := range map[string][]byte{
		"plain":   valid,
		"one_bom": withBOM,
	} {
		t.Run(name, func(t *testing.T) {
			got, err := readPlanInput(bytes.NewReader(input))
			if err != nil {
				t.Fatalf("input: %v", err)
			}
			if !bytes.Equal(got, valid) {
				t.Fatalf("unexpected payload bytes: %x", got[:min(4, len(got))])
			}
			if !json.Valid(got) {
				t.Fatal("JSON became invalid")
			}
			// Valid JSON is not the same as an approved Terraform plan.
			if _, err := liveverify.ReviewFD004BaselinePlanJSON(got); err == nil {
				t.Fatal("incomplete plan incorrectly approved")
			}
		})
	}
}

func TestReadPlanInputDoesNotOverNormalize(t *testing.T) {
	for name, input := range map[string][]byte{
		"double_bom":    append(append([]byte{}, utf8BOM...), append(utf8BOM, []byte(`{}`)...)...),
		"midstream_bom": append([]byte(`{"a":`), append(utf8BOM, []byte(`1}`)...)...),
		"malformed":     []byte(`{"a":`),
		"utf16_bom":     {0xFF, 0xFE, 0x7B, 0x00},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := readPlanInput(bytes.NewReader(input))
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if json.Valid(got) {
				t.Fatal("malformed/unsupported JSON unexpectedly accepted")
			}
			if _, err := liveverify.ReviewFD004BaselinePlanJSON(got); err == nil {
				t.Fatal("invalid JSON incorrectly approved")
			}
		})
	}
}

func TestReadPlanInputEmptyOversizedAndError(t *testing.T) {
	for name, r := range map[string]io.Reader{
		"empty":              strings.NewReader(""),
		"bom_only":           bytes.NewReader(utf8BOM),
		"oversized":          bytes.NewReader(bytes.Repeat([]byte("x"), liveverify.FD004MaxPlanJSONBytes+1)),
		"oversized_with_bom": bytes.NewReader(append(append([]byte{}, utf8BOM...), bytes.Repeat([]byte("x"), liveverify.FD004MaxPlanJSONBytes+1)...)),
		"read_error":         failingReader{},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := readPlanInput(r); err == nil {
				t.Fatal("expected input rejection")
			}
		})
	}
}
