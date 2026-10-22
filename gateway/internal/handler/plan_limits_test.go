package handler

import (
	"testing"
	"time"
)

func TestAPILimits(t *testing.T) {
	for _, tc := range []struct {
		paid     bool
		rpm, tpm int
	}{{false, 20, 250000}, {true, 55, 1000000}} {
		rpm, tpm := apiPlanLimits(tc.paid)
		if rpm != tc.rpm || tpm != tc.tpm {
			t.Fatalf("apiPlanLimits(%v)=(%d,%d), mau (%d,%d)", tc.paid, rpm, tpm, tc.rpm, tc.tpm)
		}
	}
}

func TestReferralWeeklyWindowAnchoredAtFirstJoin(t *testing.T) {
	first := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	start, reset := referralWeeklyWindow(first, first.Add(8*24*time.Hour))
	if want := first.Add(7 * 24 * time.Hour); !start.Equal(want) {
		t.Fatalf("start=%v, mau %v", start, want)
	}
	if want := first.Add(14 * 24 * time.Hour); !reset.Equal(want) {
		t.Fatalf("reset=%v, mau %v", reset, want)
	}
}
