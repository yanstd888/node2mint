package handler

import (
	"testing"
)

// referralCommissionEligible memastikan pembayaran komisi hanya terjadi
// ketika qualifying_topups belum mencapai batas maksimum 2.
func referralCommissionEligible(qualifyingTopups int) bool {
	return qualifyingTopups < 2
}

func TestReferralCommissionEligible(t *testing.T) {
	cases := []struct {
		name   string
		topups int
		want   bool
	}{
		{"0 topups — boleh bayar komisi", 0, true},
		{"1 topup — masih boleh bayar komisi kedua", 1, true},
		{"2 topups — karena menccapai max, tidak boleh komisi lagi", 2, false},
		{"3 topups — sudah melebihi max", 3, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := referralCommissionEligible(c.topups); got != c.want {
				t.Fatalf("referralCommissionEligible(%d) = %v, mau %v", c.topups, got, c.want)
			}
		})
	}
}
