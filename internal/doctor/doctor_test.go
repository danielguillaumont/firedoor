package doctor

import "testing"

func TestCollectHasStableToolInventory(t *testing.T) {
	result := Collect()
	if result.OS == "" || result.Arch == "" {
		t.Fatal("OS and architecture must be set")
	}
	if len(result.Tools) != len(requiredBinaries) {
		t.Fatalf("tool count = %d, want %d", len(result.Tools), len(requiredBinaries))
	}
	for i, tool := range result.Tools {
		if tool.Name != requiredBinaries[i] {
			t.Fatalf("tool[%d] name = %q", i, tool.Name)
		}
		if !tool.Found && tool.Path != "" {
			t.Fatalf("tool %s missing yet has path", tool.Name)
		}
	}
}
