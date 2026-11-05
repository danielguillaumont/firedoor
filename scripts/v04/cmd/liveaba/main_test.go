package main

import "testing"

func TestLiveCommandGateRequiresTripleConsent(t *testing.T) {
	for _, tc := range []struct{ name, id, ack, env string }{
		{"no args", "", "", ""},
		{"foreign experiment", "bad-route", consent, liveEnvValue},
		{"missing acknowledgement", "witness-edge-to-data", "", liveEnvValue},
		{"wrong acknowledgement", "witness-edge-to-data", "I agree", liveEnvValue},
		{"missing environment", "witness-edge-to-data", consent, ""},
		{"wrong environment", "witness-edge-to-data", consent, "prod"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := approvedLiveInput(tc.id, tc.ack, tc.env); err == nil {
				t.Fatal("unsafe operator consent accepted")
			}
		})
	}
	for _, id := range []string{"witness-edge-to-data", "witness-data-to-app"} {
		if err := approvedLiveInput(id, consent, liveEnvValue); err != nil {
			t.Fatalf("complete exact operator consent rejected: %v", err)
		}
	}
}
