// Package api provides a localhost-only, read-only HTTP contract. It has
// no Kubernetes clients, Terraform execution, SQLite or filesystem access.
// No observation or security verdict is inferred from a contract.
package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/danielguillaumont/firedoor/internal/prooflens"
	"github.com/danielguillaumont/firedoor/pkg/models"
)

const (
	ListenAddress = "127.0.0.1:8787"
	SchemaVersion = "firedoor.v06.console-api.v1"
)

type Health struct {
	SchemaVersion  string `json:"schema_version"`
	Service        string `json:"service"`
	Status         string `json:"status"`
	Scope          string `json:"scope"`
	ClusterChecked bool   `json:"cluster_checked"`
	EvidenceLoaded bool   `json:"evidence_loaded"`
}

type Capability struct {
	Name         string `json:"name"`
	Path         string `json:"path,omitempty"`
	Availability string `json:"availability"`
}

type Capabilities struct {
	SchemaVersion string       `json:"schema_version"`
	Mode          string       `json:"mode"`
	Capabilities  []Capability `json:"capabilities"`
	Limitations   []string     `json:"limitations"`
}

type Scenario struct {
	models.Scenario
	ContractStatus  string `json:"contract_status"`
	SecurityVerdict string `json:"security_verdict"`
	EvidenceLoaded  bool   `json:"evidence_loaded"`
}

type Scenarios struct {
	SchemaVersion string     `json:"schema_version"`
	Source        string     `json:"source"`
	Scenarios     []Scenario `json:"scenarios"`
	Limitations   []string   `json:"limitations"`
}

// fixedScenarioContracts mirrors the five reviewed scenarios/FD-*/contract.json
// planning contracts. The contract-sync test fails if those source definitions
// change without an explicit API review. These are NOT executable scenarios.
func fixedScenarioContracts() []Scenario {
	return []Scenario{
		{Scenario: models.Scenario{SchemaVersion: "1.0.0", ID: "FD-001", Name: "Compromised Workload", Claim: "Synthetic frontend foothold has only explicitly allowed reach", Controls: []string{"C-004", "C-005"}}, ContractStatus: "spec_only_not_implemented", SecurityVerdict: string(models.NotTested)},
		{Scenario: models.Scenario{SchemaVersion: "1.0.0", ID: "FD-002", Name: "Identity Overreach", Claim: "Workload ServiceAccount cannot perform forbidden Kubernetes API actions", Controls: []string{"C-001", "C-003"}}, ContractStatus: "spec_only_not_implemented", SecurityVerdict: string(models.NotTested)},
		{Scenario: models.Scenario{SchemaVersion: "1.0.0", ID: "FD-003", Name: "Secret Boundary", Claim: "Unrelated workload ServiceAccount cannot read synthetic secret", Controls: []string{"C-003", "C-013"}}, ContractStatus: "spec_only_not_implemented", SecurityVerdict: string(models.NotTested)},
		{Scenario: models.Scenario{SchemaVersion: "1.0.0", ID: "FD-004", Name: "Lateral Movement", Claim: "Frontend cannot directly reach protected data service after hardening", Controls: []string{"C-004", "C-005"}}, ContractStatus: "spec_only_not_implemented", SecurityVerdict: string(models.NotTested)},
		{Scenario: models.Scenario{SchemaVersion: "1.0.0", ID: "FD-005", Name: "Privileged Workload Admission", Claim: "Unsafe workload specification is rejected by configured admission controls", Controls: []string{"C-008", "C-011"}}, ContractStatus: "spec_only_not_implemented", SecurityVerdict: string(models.NotTested)},
	}
}

// NewHandler is deterministic and pure: it accesses no files, processes,
// stores, cluster clients, or outbound network resources. Host and Origin
// checks reduce browser DNS rebinding and cross-site request exposure.
func NewHandler() http.Handler { return newHandler(nil) }

// NewHandlerWithApprovedProof uses an already checked in-memory 6C1 view.
// Only the explicit CLI import path may choose to provide this view; the
// default API never opens a file or loads evidence.
func NewHandlerWithApprovedProof(proof *prooflens.ConsoleProofSummary) http.Handler {
	if proof == nil {
		return NewHandler()
	}
	// Deep-copy the immutable JSON projection before constructing the handler.
	snapshot := *proof
	snapshot.Limitations = append([]string(nil), proof.Limitations...)
	snapshot.HistoricalWitnesses = append([]prooflens.ConsoleHistoricalWitness(nil), proof.HistoricalWitnesses...)
	for i := range snapshot.HistoricalWitnesses {
		snapshot.HistoricalWitnesses[i].Phases = append([]prooflens.ConsoleWitnessPhase(nil), proof.HistoricalWitnesses[i].Phases...)
	}
	return newHandler(&snapshot)
}

func newHandler(approvedProof *prooflens.ConsoleProofSummary) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("Vary", "Origin")

		if r.Host != "127.0.0.1:8787" && r.Host != "localhost:8787" {
			writeError(w, http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
			writeError(w, http.StatusForbidden)
			return
		}
		if r.URL.RawQuery != "" {
			writeError(w, http.StatusBadRequest)
			return
		}
		// The optional route does not exist unless explicitly approved at
		// process startup; even HEAD/OPTIONS must not disclose it by default.
		if (r.URL.Path == "/api/v1/proof-summary" || r.URL.Path == "/api/v1/environment-snapshot" || r.URL.Path == "/api/v1/historical-activity") && approvedProof == nil {
			writeError(w, http.StatusNotFound)
			return
		}
		switch r.URL.Path {
		case "/api/v1/health", "/api/v1/capabilities", "/api/v1/scenarios", "/api/v1/proof-summary", "/api/v1/control-catalog", "/api/v1/environment-snapshot", "/api/v1/historical-activity":
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", "GET")
				writeError(w, http.StatusMethodNotAllowed)
				return
			}
		default:
			writeError(w, http.StatusNotFound)
			return
		}

		switch r.URL.Path {
		case "/api/v1/health":
			writeJSON(w, Health{SchemaVersion: SchemaVersion, Service: "firedoor-console-api", Status: "ok", Scope: "local_http_process_only", ClusterChecked: false, EvidenceLoaded: false})
		case "/api/v1/capabilities":
			proofCapability := Capability{Name: "proof_reports", Availability: "not_enabled"}
			environmentCapability := Capability{Name: "historical_environment_snapshot", Availability: "not_enabled"}
			historyCapability := Capability{Name: "historical_activity", Availability: "not_enabled"}
			mode := "offline_contracts_only"
			limitations := []string{"Health means only the local HTTP process responds; it is not Kubernetes readiness.", "No Terraform, Kubernetes, SQLite, traffic probes, evidence files or security verdicts are accessed."}
			if approvedProof != nil {
				proofCapability = Capability{Name: "proof_reports", Path: "/api/v1/proof-summary", Availability: "available"}
				environmentCapability = Capability{Name: "historical_environment_snapshot", Path: "/api/v1/environment-snapshot", Availability: "available"}
				historyCapability = Capability{Name: "historical_activity", Path: "/api/v1/historical-activity", Availability: "available"}
				mode = "approved_detached_historical_report_read_only"
				limitations = []string{"Health means only the local HTTP process responds; it is not Kubernetes readiness.", "An explicitly approved detached historical report was validated and loaded at startup only; it does not establish policy enforcement or authenticate original evidence."}
			}
			writeJSON(w, Capabilities{
				SchemaVersion: SchemaVersion, Mode: mode,
				Capabilities: []Capability{
					{Name: "health", Path: "/api/v1/health", Availability: "available"},
					{Name: "scenario_contracts", Path: "/api/v1/scenarios", Availability: "available"},
					proofCapability,
					{Name: "control_findings", Availability: "not_enabled"},
					{Name: "environment_inventory", Availability: "not_enabled"},
					{Name: "scenario_execution", Availability: "not_enabled"},
					{Name: "control_catalog", Path: "/api/v1/control-catalog", Availability: "available"},
					environmentCapability,
					historyCapability,
				},
				Limitations: limitations,
			})
		case "/api/v1/proof-summary":
			writeJSON(w, approvedProof)
		case "/api/v1/control-catalog":
			writeJSON(w, controlCatalog())
		case "/api/v1/environment-snapshot":
			writeJSON(w, projectHistoricalEnvironment(approvedProof))
		case "/api/v1/historical-activity":
			activity, err := projectHistoricalActivity(approvedProof)
			if err != nil { writeError(w, http.StatusNotFound); return }
			writeJSON(w, activity)
		case "/api/v1/scenarios":
			writeJSON(w, Scenarios{
				SchemaVersion: SchemaVersion, Source: "versioned_planning_contracts_only",
				Scenarios:   fixedScenarioContracts(),
				Limitations: []string{"These five contract records are not completed scenario validations.", "Separate historical network tests do not certify FD-001 through FD-005; no evidence is loaded."},
			})
		}
	})
}

func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(body) // all responses are bounded static values
}

func writeError(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
	}{Error: strings.ReplaceAll(http.StatusText(status), " ", "_")})
}
