package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/danielguillaumont/firedoor/internal/attribution"
	"github.com/danielguillaumont/firedoor/internal/prooflens"
)

const consentText = "I-APPROVE-READ-ONLY-FIREDOOR-KUBERNETES-OBSERVATION"

func run() error {
	root := flag.String("root", ".", "Firedoor repository root")
	observe := flag.Bool("observe", false, "Read owned kind lab; no mutation")
	consent := flag.String("consent", "", "Required exact consent for Kubernetes read")
	flag.Parse()
	if len(flag.Args()) != 0 {
		return errors.New("unexpected positional arguments")
	}
	abs, err := filepath.Abs(*root)
	if err != nil {
		return err
	}
	manifest, err := prooflens.CollectTerraformSources(abs)
	if err != nil {
		return err
	}
	if !*observe {
		if *consent != "" {
			return errors.New("consent without --observe is not accepted")
		}
		return json.NewEncoder(os.Stdout).Encode(struct {
			Mode                string `json:"mode"`
			ObservationExecuted bool   `json:"observation_executed"`
			DatabaseOpened      bool   `json:"database_opened"`
			RawObjectsSaved     bool   `json:"raw_objects_saved"`
			ConsentRequired     bool   `json:"consent_required"`
			SourceTreeSHA256    string `json:"source_tree_sha256"`
		}{"REVIEW_ONLY_NO_KUBERNETES_ACCESS", false, false, false, true, manifest.TreeSHA256})
	}
	if *consent != consentText {
		return errors.New("Kubernetes read-only observation requires exact operator consent")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second)
	defer cancel()
	p, err := attribution.RunPreflight(ctx, attribution.ReadOnlyCommands{})
	if err != nil {
		return fmt.Errorf("owned hardened lab observation rejected: %w", err)
	}
	if err := prooflens.RecheckTerraformSources(abs, manifest); err != nil {
		return fmt.Errorf("Terraform source changed during Kubernetes observation: %w", err)
	}
	record, err := prooflens.BuildKubernetesObservation(p, manifest)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(record)
}

func main() {
	if err := run(); err != nil {
		// Never output raw Kubernetes objects or Terraform state.
		fmt.Fprintln(os.Stderr, "Kubernetes observation refused:", err)
		os.Exit(1)
	}
}
