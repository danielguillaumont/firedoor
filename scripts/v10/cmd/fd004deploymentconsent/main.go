package main

import (
	"encoding/json"
	"fmt"
	"github.com/danielguillaumont/firedoor/internal/recoverydeployconsent"
	"os"
)

func main() {
	request, err := recoverydeployconsent.Decode(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rejected review JSON:", err)
		os.Exit(1)
	}
	receipt, err := recoverydeployconsent.Evaluate(request)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rejected review request:", err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(receipt); err != nil {
		fmt.Fprintln(os.Stderr, "review output failed:", err)
		os.Exit(1)
	}
}
