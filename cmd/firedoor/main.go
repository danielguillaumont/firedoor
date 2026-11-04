package main

import (
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
