package handler

import "testing"

func TestKYCConsentAccepted(t *testing.T) {
	for _, value := range []string{"true", " TRUE ", "True"} {
		if !kycConsentAccepted(value) {
			t.Fatalf("consent %q must be accepted", value)
		}
	}
	for _, value := range []string{"", "false", "1", "yes"} {
		if kycConsentAccepted(value) {
			t.Fatalf("consent %q must be rejected", value)
		}
	}
}
