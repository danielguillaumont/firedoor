// fd004consoleliveproof is an explicit, offline, one-file projection tool.
// It never discovers filesystem paths, connects to a cluster, or writes a file.
// stdout contains only an allowlisted historical console JSON payload.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/danielguillaumont/firedoor/internal/consoleliveproof"
)

func main() {
	source := flag.String("receipt", "", "explicit private receipt file")
	expected := flag.String("sha256", "", "expected receipt SHA-256 from verified F5 log")
	flag.Parse()
	if *source == "" || len(*expected) != 64 || len(flag.Args()) != 0 {
		fail("receipt and 64-character SHA-256 required")
	}
	expectedDigest, err := hex.DecodeString(*expected)
	if err != nil || len(expectedDigest) != 32 {
		fail("invalid SHA-256")
	}
	fi, err := os.Lstat(*source)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() < 2 || fi.Size() > 32*1024 {
		fail("receipt must be a regular bounded file")
	}
	data, err := os.ReadFile(*source)
	if err != nil {
		fail("could not read receipt")
	}
	digest := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(digest[:]), *expected) {
		fail("receipt SHA-256 does not match the approved value")
	}
	projection, err := consoleliveproof.DecodeAndProject(data)
	if err != nil {
		fail("receipt outside approved projection contract")
	}
	output, err := json.MarshalIndent(projection, "", "  ")
	if err != nil {
		fail("cannot encode projection")
	}
	fmt.Println(string(output))
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "G1 projection rejected: "+message)
	os.Exit(1)
}
