package handler

import "testing"

func TestAffiliateCommissionUsesSellerNetAfterPlatformFee(t *testing.T) {
	// Harga 100.000, fee platform 10.000, net seller 90.000.
	// Komisi seller 20% berarti affiliator 18.000 dan seller menerima 72.000.
	net := 90000.0
	commission := affiliateCommission(net, 20)
	if commission != 18000 {
		t.Fatalf("commission = %.0f, want 18000", commission)
	}
	if seller := net - commission; seller != 72000 {
		t.Fatalf("seller payout = %.0f, want 72000", seller)
	}
}

func TestAffiliateCommissionRejectsInvalidPercent(t *testing.T) {
	for _, pct := range []float64{-1, 0, 100.01} {
		if got := affiliateCommission(90000, pct); got != 0 {
			t.Fatalf("percent %.2f returned %.0f, want 0", pct, got)
		}
	}
}
