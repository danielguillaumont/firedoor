package main

import "testing"

func TestFD004RecoveryEvidenceCLIRefusesMissingInputs(t *testing.T) {
	cases := [][]string{
		nil,
		{"-initial", "initial.json"},
		{"-initial", "initial.json", "-baseline", "baseline.json"},
		{"-initial", "file.json", "-baseline", "file.json", "-restored", "file.json"},
		{"-initial", "a", "-baseline", "b", "-restored", "c", "unexpected"},
	}
	for _, args := range cases {
		if err := run(args); err == nil {
			t.Fatalf("accepted invalid arguments: %v", args)
		}
	}
}
