package handler

import "testing"

func TestPublicStatusOverallRequiresObservedChecks(t *testing.T) {
	cases := []struct {
		name                       string
		monitorStatus              string
		monitorChecks, modelChecks int
		modelsHealthy              bool
		want                       string
	}{
		{"no data", "unknown", 0, 0, true, "UNKNOWN"},
		{"healthy model data", "unknown", 0, 1, true, "OPERATIONAL"},
		{"monitor down", "down", 1, 0, true, "DEGRADED"},
		{"model degraded", "ok", 1, 1, false, "DEGRADED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := publicStatusOverall(tc.monitorStatus, tc.monitorChecks, tc.modelChecks, tc.modelsHealthy); got != tc.want {
				t.Fatalf("publicStatusOverall() = %q, want %q", got, tc.want)
			}
		})
	}
}
