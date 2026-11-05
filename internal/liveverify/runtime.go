package liveverify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	ObservationSchema = "firedoor.v03.live-observations.v1"
	sourceContainer   = "demo-http"
	expectedNodeName  = "firedoor-local-control-plane"
	commandTimeout    = 7 * time.Second
)

// CommandRunner is injected so the entire executor can be tested without
// contacting Kubernetes. Only narrowly constructed invocations reach it.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// OSCommands is the opt-in real adapter. It captures stdout only: kubectl
// failures are errors, never evidence of denied traffic.
type OSCommands struct{}

func (OSCommands) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("command cancelled or timed out: %w", ctx.Err())
		}
		return nil, fmt.Errorf("command failed (not a connectivity observation): %w", err)
	}
	return output, nil
}

type OwnedPod struct {
	Zone      Zone   `json:"zone"`
	Namespace string `json:"namespace"`
	PodName   string `json:"pod_name"`
	UID       string `json:"uid"`
	IP        string `json:"ip"`
}

type Inventory struct {
	SchemaVersion string     `json:"schema_version"`
	Cluster       string     `json:"cluster"`
	Context       string     `json:"context"`
	CapturedAtUTC time.Time  `json:"captured_at_utc"`
	Pods          []OwnedPod `json:"pods"`
}

var podNamePattern = regexp.MustCompile(`^firedoor-(edge|app|data)-[a-z0-9]+-[a-z0-9]+$`)

func zoneNamespace(zone Zone) (string, error) {
	switch zone {
	case ZoneEdge, ZoneApp, ZoneData:
		return "firedoor-" + string(zone), nil
	default:
		return "", fmt.Errorf("out-of-scope zone %q", zone)
	}
}

func ownedLabels(labels map[string]string, zone Zone) bool {
	return labels["firedoor.dev/managed-by"] == "terraform" &&
		labels["firedoor.dev/zone"] == string(zone) &&
		labels["app.kubernetes.io/part-of"] == "firedoor"
}

func validPodIP(s string) bool {
	parsed := net.ParseIP(s)
	return parsed != nil && parsed.To4() != nil && parsed.IsPrivate() && !parsed.IsLoopback() && !parsed.IsUnspecified()
}

func (i Inventory) Validate() error {
	if i.SchemaVersion != ObservationSchema || i.Cluster != OwnedCluster || i.Context != OwnedContext || len(i.Pods) != 3 {
		return errors.New("inventory identity or zone count mismatch")
	}
	seen := map[Zone]bool{}
	seenIP := map[string]bool{}
	for _, p := range i.Pods {
		ns, err := zoneNamespace(p.Zone)
		if err != nil || p.Namespace != ns || !podNamePattern.MatchString(p.PodName) ||
			!strings.HasPrefix(p.PodName, ns+"-") || p.UID == "" || !validPodIP(p.IP) || seen[p.Zone] || seenIP[p.IP] {
			return fmt.Errorf("invalid or duplicate owned Pod in zone %q", p.Zone)
		}
		seen[p.Zone] = true
		seenIP[p.IP] = true
	}
	return nil
}

func (i Inventory) pod(zone Zone) (OwnedPod, error) {
	for _, p := range i.Pods {
		if p.Zone == zone {
			return p, nil
		}
	}
	return OwnedPod{}, fmt.Errorf("zone %s absent from inventory", zone)
}

type kubeMeta struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	UID       string            `json:"uid"`
	Labels    map[string]string `json:"labels"`
}

type kubeNodeList struct {
	Items []struct {
		Metadata kubeMeta `json:"metadata"`
		Status   struct {
			Conditions []struct {
				Type   string `json:"type"`
				Status string `json:"status"`
			} `json:"conditions"`
		} `json:"status"`
	} `json:"items"`
}

type kubeNamespace struct {
	Metadata kubeMeta `json:"metadata"`
	Status   struct {
		Phase string `json:"phase"`
	} `json:"status"`
}

type kubeDeployment struct {
	Metadata kubeMeta `json:"metadata"`
	Spec     struct {
		Replicas *int `json:"replicas"`
		Template struct {
			Metadata kubeMeta `json:"metadata"`
			Spec     struct {
				Containers []struct {
					Name  string `json:"name"`
					Image string `json:"image"`
				} `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
	Status struct {
		ReadyReplicas     int `json:"readyReplicas"`
		AvailableReplicas int `json:"availableReplicas"`
	} `json:"status"`
}

type kubePod struct {
	Metadata kubeMeta `json:"metadata"`
	Spec     struct {
		NodeName   string `json:"nodeName"`
		Containers []struct {
			Name  string `json:"name"`
			Image string `json:"image"`
		} `json:"containers"`
	} `json:"spec"`
	Status struct {
		Phase      string `json:"phase"`
		PodIP      string `json:"podIP"`
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"conditions"`
		ContainerStatuses []struct {
			Name  string `json:"name"`
			Ready bool   `json:"ready"`
		} `json:"containerStatuses"`
	} `json:"status"`
}

type kubePodList struct {
	Items []kubePod `json:"items"`
}

// Inspector verifies kind identity and all owned resources before enabling
// any exec. It will not inspect other namespaces or arbitrary object types.
type Inspector struct {
	Runner CommandRunner
	Now    func() time.Time
}

func (d Inspector) run(ctx context.Context, bin string, args ...string) ([]byte, error) {
	if d.Runner == nil {
		return nil, errors.New("no command runner configured")
	}
	// Invocations are only made by this package with compiled, fixed argv;
	// do not expose a generic command from user input.
	child, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	return d.Runner.Run(child, bin, args...)
}

func (d Inspector) get(ctx context.Context, out any, args ...string) error {
	all := append([]string{"--context", OwnedContext}, args...)
	data, err := d.run(ctx, "kubectl", all...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("invalid Kubernetes response: %w", err)
	}
	return nil
}

func (d Inspector) Discover(ctx context.Context) (Inventory, error) {
	inv := Inventory{SchemaVersion: ObservationSchema, Cluster: OwnedCluster, Context: OwnedContext}
	current, err := d.run(ctx, "kubectl", "config", "current-context")
	if err != nil || strings.TrimSpace(string(current)) != OwnedContext {
		return Inventory{}, errors.New("active kubectl context does not match dedicated Firedoor kind context")
	}
	clusters, err := d.run(ctx, "kind", "get", "clusters")
	if err != nil {
		return Inventory{}, fmt.Errorf("kind discovery failed: %w", err)
	}
	present := false
	for _, line := range strings.Split(string(clusters), "\n") {
		if strings.TrimSpace(line) == OwnedCluster {
			present = true
		}
	}
	if !present {
		return Inventory{}, errors.New("dedicated Firedoor kind cluster is missing")
	}
	var nodes kubeNodeList
	if err := d.get(ctx, &nodes, "get", "nodes", "-o", "json"); err != nil {
		return Inventory{}, err
	}
	if len(nodes.Items) != 1 || nodes.Items[0].Metadata.Name != expectedNodeName {
		return Inventory{}, errors.New("unexpected kind node inventory")
	}
	ready := false
	for _, cond := range nodes.Items[0].Status.Conditions {
		if cond.Type == "Ready" && cond.Status == "True" {
			ready = true
		}
	}
	if !ready {
		return Inventory{}, errors.New("Firedoor kind node not Ready")
	}

	// Check ownership of all namespaces before reading workload objects.
	zones := []Zone{ZoneEdge, ZoneApp, ZoneData}
	for _, zone := range zones {
		ns, _ := zoneNamespace(zone)
		var obj kubeNamespace
		if err := d.get(ctx, &obj, "get", "namespace", ns, "-o", "json"); err != nil {
			return Inventory{}, err
		}
		if obj.Metadata.Name != ns || !ownedLabels(obj.Metadata.Labels, zone) || obj.Status.Phase != "Active" {
			return Inventory{}, fmt.Errorf("namespace %s identity/ownership mismatch", ns)
		}
	}
	for _, zone := range zones {
		ns, _ := zoneNamespace(zone)
		var deploy kubeDeployment
		if err := d.get(ctx, &deploy, "-n", ns, "get", "deployment", ns, "-o", "json"); err != nil {
			return Inventory{}, err
		}
		if deploy.Metadata.Name != ns || deploy.Metadata.Namespace != ns || !ownedLabels(deploy.Metadata.Labels, zone) ||
			deploy.Spec.Replicas == nil || *deploy.Spec.Replicas != 1 || deploy.Status.ReadyReplicas != 1 || deploy.Status.AvailableReplicas != 1 ||
			len(deploy.Spec.Template.Spec.Containers) != 1 || deploy.Spec.Template.Spec.Containers[0].Name != sourceContainer ||
			deploy.Spec.Template.Spec.Containers[0].Image != "busybox:1.36.1" {
			return Inventory{}, fmt.Errorf("deployment %s not the expected single ready Firedoor workload", ns)
		}
		var pods kubePodList
		if err := d.get(ctx, &pods, "-n", ns, "get", "pods", "-l", "app.kubernetes.io/name="+ns, "-o", "json"); err != nil {
			return Inventory{}, err
		}
		if len(pods.Items) != 1 {
			return Inventory{}, fmt.Errorf("namespace %s: expected exactly one workload Pod", ns)
		}
		pod := pods.Items[0]
		if err := validateObservedPod(pod, zone); err != nil {
			return Inventory{}, err
		}
		inv.Pods = append(inv.Pods, OwnedPod{Zone: zone, Namespace: ns, PodName: pod.Metadata.Name, UID: pod.Metadata.UID, IP: pod.Status.PodIP})
	}
	if d.Now == nil {
		inv.CapturedAtUTC = time.Now().UTC()
	} else {
		inv.CapturedAtUTC = d.Now().UTC()
	}
	if err := inv.Validate(); err != nil {
		return Inventory{}, err
	}
	return inv, nil
}

func validateObservedPod(p kubePod, zone Zone) error {
	ns, _ := zoneNamespace(zone)
	if p.Metadata.Namespace != ns || !podNamePattern.MatchString(p.Metadata.Name) || !strings.HasPrefix(p.Metadata.Name, ns+"-") ||
		p.Metadata.UID == "" || !ownedLabels(p.Metadata.Labels, zone) || p.Metadata.Labels["app.kubernetes.io/name"] != ns ||
		p.Status.Phase != "Running" || p.Spec.NodeName != expectedNodeName || !validPodIP(p.Status.PodIP) ||
		len(p.Spec.Containers) != 1 || p.Spec.Containers[0].Name != sourceContainer || p.Spec.Containers[0].Image != "busybox:1.36.1" ||
		len(p.Status.ContainerStatuses) != 1 || p.Status.ContainerStatuses[0].Name != sourceContainer || !p.Status.ContainerStatuses[0].Ready {
		return fmt.Errorf("Pod in %s does not match expected owned, ready identity", ns)
	}
	ready := false
	for _, cond := range p.Status.Conditions {
		if cond.Type == "Ready" && cond.Status == "True" {
			ready = true
		}
	}
	if !ready {
		return fmt.Errorf("Pod in %s is not Ready", ns)
	}
	return nil
}

// Recheck guards against Pod replacement, IP rotation, or lost readiness
// between discovery and execution. A mismatch yields inconclusive evidence.
func (d Inspector) Recheck(ctx context.Context, original OwnedPod) error {
	ns, err := zoneNamespace(original.Zone)
	if err != nil || ns != original.Namespace {
		return errors.New("Pod moved out of scope")
	}
	var pod kubePod
	if err := d.get(ctx, &pod, "-n", ns, "get", "pod", original.PodName, "-o", "json"); err != nil {
		return err
	}
	if err := validateObservedPod(pod, original.Zone); err != nil {
		return err
	}
	if pod.Metadata.UID != original.UID || pod.Status.PodIP != original.IP {
		return errors.New("Pod UID or IP changed after discovery")
	}
	return nil
}
