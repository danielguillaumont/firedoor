package liveverify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/danielguillaumont/firedoor/internal/kubeinspect"
)

// FD004LiveCollectionSchema identifies a READ-ONLY inventory bundle, not a
// verified end-to-end security proof. Probe causality is deliberately absent.
const FD004LiveCollectionSchema = "firedoor.v10.fd004-live-collection.v1"

const maxFD004StateList = 32 << 10

type FD004ReadOnlyRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type FD004SnapshotProvider interface {
	Collect(ctx context.Context) (kubeinspect.Snapshot, error)
}

// FD004LiveCollector accepts injectable adapters. The only production calls to
// Terraform are the fixed, read-only `state list` and `output -json` commands.
// Kubernetes inventory is delegated to kubeinspect's existing allowlisted,
// ownership-checked, read-only collector.
type FD004LiveCollector struct {
	Terraform  FD004ReadOnlyRunner
	Kubernetes FD004SnapshotProvider
	Now        func() time.Time
}

type FD004LiveCollection struct {
	SchemaVersion                string               `json:"schema_version"`
	Scenario                     string               `json:"scenario"`
	Cluster                      string               `json:"cluster"`
	Context                      string               `json:"context"`
	Profile                      Profile              `json:"profile"`
	CapturedAtUTC                time.Time            `json:"captured_at_utc"`
	TerraformRoot                string               `json:"terraform_root"`
	TerraformStateAddresses      []string             `json:"terraform_state_addresses"`
	TerraformOutputsSHA256       string               `json:"terraform_outputs_sha256"`
	TerraformOutputs             json.RawMessage      `json:"terraform_outputs"`
	KubernetesSnapshot           kubeinspect.Snapshot `json:"kubernetes_snapshot"`
	PolicyStatus                 string               `json:"policy_status"`
	StateAddressInventoryChecked bool                 `json:"state_address_inventory_checked"`
	TerraformAuthenticated       bool                 `json:"terraform_authenticated"`
	TrafficVerified              bool                 `json:"traffic_verified"`
	Status                       string               `json:"status"`
	Limitations                  string               `json:"limitations"`
}

func fd004ExpectedStateAddresses() []string {
	out := make([]string, 0, 14)
	for _, zone := range []string{"edge", "app", "data"} {
		for _, resource := range []string{"kubernetes_namespace_v1", "kubernetes_service_account_v1", "kubernetes_deployment_v1", "kubernetes_service_v1"} {
			out = append(out, fmt.Sprintf(`module.workloads[%q].%s.this`, zone, resource))
		}
	}
	for _, zone := range []string{"app", "data"} {
		out = append(out, fmt.Sprintf(`module.network_boundaries[%q].kubernetes_network_policy_v1.this`, zone))
	}
	sort.Strings(out)
	return out
}

func parseFD004StateAddresses(b []byte) ([]string, error) {
	if len(b) == 0 || len(b) > maxFD004StateList || bytes.IndexByte(b, 0) >= 0 {
		return nil, errors.New("Terraform state address inventory missing, oversized, or invalid")
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	actual := make([]string, 0, 14)
	seen := make(map[string]bool, 14)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if seen[line] {
			return nil, errors.New("duplicate Terraform state resource address")
		}
		seen[line] = true
		actual = append(actual, line)
	}
	sort.Strings(actual)
	wanted := fd004ExpectedStateAddresses()
	if len(actual) != len(wanted) {
		return nil, fmt.Errorf("expected 14 Firedoor-managed state addresses, got %d", len(actual))
	}
	for i := range wanted {
		if wanted[i] != actual[i] {
			return nil, errors.New("Terraform state includes unexpected, missing, or foreign resource address")
		}
	}
	return actual, nil
}

// Collect performs NO state changes. It reports drift rather than treating it
// as successful protection. Even a consistent result is configuration-only:
// local state addresses/outputs and Kubernetes inventory are not a proof of
// Terraform state authenticity, CNI enforcement, or HTTP route behavior.
func (c FD004LiveCollector) Collect(ctx context.Context, profile Profile) (FD004LiveCollection, error) {
	if _, err := NewPlan(profile); err != nil {
		return FD004LiveCollection{}, err
	}
	if c.Terraform == nil || c.Kubernetes == nil {
		return FD004LiveCollection{}, errors.New("read-only collectors are not configured")
	}

	// Verify Kubernetes identity and ownership FIRST, before accessing state.
	snapshot, err := c.Kubernetes.Collect(ctx)
	if err != nil {
		return FD004LiveCollection{}, fmt.Errorf("Kubernetes ownership inspection failed: %w", err)
	}
	if snapshot.Cluster != OwnedCluster || snapshot.Context != OwnedContext || snapshot.SchemaVersion != kubeinspect.Schema || snapshot.CapturedAtUTC.IsZero() {
		return FD004LiveCollection{}, errors.New("Kubernetes inventory identity, schema, or capture time invalid")
	}
	policyStatus, err := fd004PolicyStatus(snapshot, profile)
	if err != nil {
		return FD004LiveCollection{}, fmt.Errorf("Kubernetes policy inspection refused: %w", err)
	}

	state, err := c.Terraform.Run(ctx, "terraform", "-chdir="+FD004TerraformRoot, "state", "list")
	if err != nil {
		return FD004LiveCollection{}, fmt.Errorf("read-only Terraform state list failed: %w", err)
	}
	addresses, err := parseFD004StateAddresses(state)
	if err != nil {
		return FD004LiveCollection{}, err
	}

	outputs, err := c.Terraform.Run(ctx, "terraform", "-chdir="+FD004TerraformRoot, "output", "-json")
	if err != nil {
		return FD004LiveCollection{}, fmt.Errorf("read-only Terraform output failed: %w", err)
	}
	if err := parseTerraformFD004Outputs(outputs, profile); err != nil {
		return FD004LiveCollection{}, fmt.Errorf("Terraform output contract rejected: %w", err)
	}

	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	captured := now().UTC()
	if captured.Before(snapshot.CapturedAtUTC.Add(-2*time.Minute)) || captured.After(snapshot.CapturedAtUTC.Add(2*time.Minute)) {
		return FD004LiveCollection{}, errors.New("Kubernetes snapshot timestamp is stale or in the future")
	}
	outputHash := sha256.Sum256(outputs)
	status := "configuration_consistent_not_behaviorally_verified"
	if policyStatus == "drift" {
		status = "configuration_drift"
	}
	return FD004LiveCollection{
		SchemaVersion: FD004LiveCollectionSchema,
		Scenario:      "FD-004", Cluster: OwnedCluster, Context: OwnedContext,
		Profile: profile, CapturedAtUTC: captured,
		TerraformRoot: FD004TerraformRoot, TerraformStateAddresses: addresses,
		TerraformOutputsSHA256: hex.EncodeToString(outputHash[:]),
		TerraformOutputs:       append(json.RawMessage(nil), outputs...), KubernetesSnapshot: snapshot,
		PolicyStatus: policyStatus, StateAddressInventoryChecked: true,
		TerraformAuthenticated: false, TrafficVerified: false,
		Status:      status,
		Limitations: "READ-ONLY local-state address/output and Kubernetes configuration consistency only. State can be stale, fabricated, or refer to a different provider; no Terraform plan, refresh, apply, or drift remediation occurred. No direct Pod-IP traffic observations or CNI enforcement proof was collected. Do not call this a verified FD-004 security boundary.",
	}, nil
}

// FD004LocalCommands runs only fixed commands constructed in this package.
// It is not an arbitrary command proxy and never invokes apply, destroy, plan,
// refresh, kubectl mutation, or reads Secret objects.
type FD004LocalCommands struct{}

func (FD004LocalCommands) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if name != "terraform" || len(args) < 3 || args[0] != "-chdir="+FD004TerraformRoot ||
		!(len(args) == 3 && args[1] == "state" && args[2] == "list") &&
			!(len(args) == 3 && args[1] == "output" && args[2] == "-json") {
		return nil, errors.New("command rejected by Terraform read-only allowlist")
	}
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.New("read-only Terraform command failed")
	}
	if len(out) > maxTerraformOutput && args[1] == "output" {
		return nil, errors.New("Terraform output exceeded strict size bound")
	}
	if len(out) > maxFD004StateList && args[1] == "state" {
		return nil, errors.New("Terraform state list exceeded strict size bound")
	}
	return out, nil
}
