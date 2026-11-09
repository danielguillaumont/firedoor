package api

import (
	"encoding/json"
	"net/http"

	"github.com/danielguillaumont/firedoor/internal/containmentmap"
	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

// writeDeclaredContainmentMap responds with a fixed v0.7 planning topology.
// Neither profile imports observations, loads a report, queries a cluster,
// reads state, or claims NetworkPolicy enforcement. API validation of Host,
// Origin, query and method is enforced by the parent handler.
func writeDeclaredContainmentMap(w http.ResponseWriter, profile liveverify.Profile) {
	topology, err := containmentmap.NewDeclaredMap(profile)
	if err != nil {
		writeError(w, http.StatusInternalServerError)
		return
	}
	// Validate the exact JSON output before responding. Never send a partial
	// topology, or a result too large for the bounded client contract.
	data, err := json.Marshal(topology)
	if err != nil || len(data) == 0 || len(data) > containmentmap.MaxJSONBytes {
		writeError(w, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(append(data, '\n'))
}
