// Package doctor inspects host tool availability. It deliberately never invokes kubectl or Terraform.
package doctor

import (
	"os/exec"
	"runtime"
)

type Tool struct {
	Name  string `json:"name"`
	Found bool   `json:"found"`
	Path  string `json:"path,omitempty"`
}

type Result struct {
	OS    string `json:"os"`
	Arch  string `json:"arch"`
	Tools []Tool `json:"tools"`
	Note  string `json:"note"`
}

var requiredBinaries = []string{"git", "go", "terraform", "docker", "kind", "kubectl", "node", "npm", "ssh", "gh"}

func Collect() Result {
	result := Result{
		OS:    runtime.GOOS,
		Arch:  runtime.GOARCH,
		Tools: make([]Tool, 0, len(requiredBinaries)),
		Note:  "Path detection only. Does not check versions, Docker readiness, Kubernetes context, CNI enforcement, or environment safety.",
	}
	for _, name := range requiredBinaries {
		path, err := exec.LookPath(name)
		result.Tools = append(result.Tools, Tool{Name: name, Found: err == nil, Path: path})
	}
	return result
}
