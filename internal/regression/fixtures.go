package regression

import (
	"embed"
	"fmt"
)

//go:embed testdata/*.json
var syntheticFixtures embed.FS

// Fixture returns only bundled synthetic test inputs, never private files.
func Fixture(name string) ([]byte, error) {
	switch name {
	case "intact", "weakened", "missing":
		return syntheticFixtures.ReadFile("testdata/" + name + ".json")
	default:
		return nil, fmt.Errorf("unsupported synthetic fixture %q", name)
	}
}
