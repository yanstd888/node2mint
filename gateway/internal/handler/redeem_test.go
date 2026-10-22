package handler

import "testing"

func TestAccessDaysRequiresRemainingTokens(t *testing.T) {
	if accessDaysAllowed(0) || !accessDaysAllowed(1) {
		t.Fatal("duration redeem must require remaining tokens")
	}
}

func TestRedeemRewardValidation(t *testing.T) {
	for _, kind := range []string{"WALLET", "API_TOKEN", "ACCESS_DAYS"} {
		if !redeemRewardAllowed(kind, 1) {
			t.Fatalf("%s reward should accept a positive whole amount", kind)
		}
	}
	if redeemRewardAllowed("ACCESS_DAYS", 1.5) || redeemRewardAllowed("UNKNOWN", 1) {
		t.Fatal("redeem must reject invalid reward type or fractional token/day amount")
	}
}
