package prooflens

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

const MaxBundleBytes = 1024 * 1024

// DecodeBundle accepts one bounded, strictly versioned JSON object. Windows
// PowerShell 5.1 UTF-8 BOM output is accepted for read-only imports.
func DecodeBundle(data []byte) (Bundle, error) {
	var b Bundle
	if len(data) > MaxBundleBytes {
		return b, errors.New("proof bundle exceeds 1 MiB")
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return Bundle{}, fmt.Errorf("decode proof bundle: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return Bundle{}, errors.New("proof bundle has trailing data or multiple JSON values")
	}
	return b, nil
}

// LoadBundle does not inspect Kubernetes, Terraform, or external resources.
func LoadBundle(path string) (Bundle, error) {
	var empty Bundle
	f, err := os.Open(path)
	if err != nil {
		return empty, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > MaxBundleBytes {
		return empty, errors.New("bundle is not a bounded regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxBundleBytes+1))
	if err != nil {
		return empty, err
	}
	return DecodeBundle(data)
}
