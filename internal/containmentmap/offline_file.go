package containmentmap

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

// ProjectV03HistoricalFile is an explicit, offline CLI-only bridge from a
// locally supplied v0.3 suite into the already reviewed 7B2 map adapter.
// The fixed six-route plan is verified for internal consistency; neither the
// source, the operator-supplied profile, nor NetworkPolicy causality is
// independently authenticated. This function does not expose any HTTP route.
//
// It intentionally returns generic errors: private input paths, Pod IPs,
// names, and decoder snippets must not leak into command output or logs.
func ProjectV03HistoricalFile(path string, profile liveverify.Profile) (Map, error) {
	rejected := errors.New("saved v0.3 historical observation unavailable or rejected")
	if !validProfile(string(profile)) || strings.TrimSpace(path) == "" {
		return Map{}, rejected
	}

	file, err := os.Open(path)
	if err != nil {
		return Map{}, rejected
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil || !stat.Mode().IsRegular() || stat.Size() <= 0 || stat.Size() > liveverify.MaxObservationBytes {
		return Map{}, rejected
	}
	data, err := io.ReadAll(io.LimitReader(file, liveverify.MaxObservationBytes+1))
	if err != nil || len(data) == 0 || len(data) > liveverify.MaxObservationBytes {
		return Map{}, rejected
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var suite liveverify.SuiteObservation
	if err := decoder.Decode(&suite); err != nil {
		return Map{}, rejected
	}
	var tail any
	if err := decoder.Decode(&tail); !errors.Is(err, io.EOF) {
		return Map{}, rejected
	}
	if suite.Profile != profile {
		return Map{}, rejected
	}

	// Includes liveverify.Verify and strict fixed-lab Map.Validate. Valid JSON
	// alone never elevates a saved observation into enforcement proof.
	projection, err := ProjectV03HistoricalObservation(suite)
	if err != nil {
		return Map{}, rejected
	}
	return projection, nil
}
