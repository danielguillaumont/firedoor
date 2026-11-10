// fd004deployreview reads Terraform's plan JSON from stdin. It never applies a plan.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/danielguillaumont/firedoor/internal/deployreview"
	"io"
	"os"
)

func main() {
	cluster := flag.String("cluster", "", "reserved disposable cluster name")
	kubeconfig := flag.String("kubeconfig", "", "expected private disposable kubeconfig path")
	flag.Parse()
	if *cluster == "" || *kubeconfig == "" {
		fmt.Fprintln(os.Stderr, "cluster and kubeconfig arguments required")
		os.Exit(2)
	}
	raw, e := io.ReadAll(io.LimitReader(os.Stdin, deployreview.Limit+1))
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	// Windows PowerShell 5.1 may add one UTF-8 BOM at stdin boundary.
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	result, e := deployreview.ReviewJSON(raw, *cluster, *kubeconfig)
	if e != nil {
		fmt.Fprintln(os.Stderr, "FD-004 initial deploy plan REJECTED:", e)
		os.Exit(1)
	}
	if e = json.NewEncoder(os.Stdout).Encode(result); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
