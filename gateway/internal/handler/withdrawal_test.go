package handler

import "testing"

func TestWithdrawalAmounts(t *testing.T) {
	fee, net := withdrawalAmounts(1_000_000, 5)
	if fee != 50_000 || net != 950_000 {
		t.Fatalf("fee/net salah: %v/%v", fee, net)
	}
}
