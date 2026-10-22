package handler

import (
	"net/http"
	"testing"
)

// boostPolicyResult adalah keputusan boost yang diambil handler sebelum
// menyentuh database. Di-faktor-kan ke luar supaya bisa diuji tanpa DB.
type boostPolicyResult struct {
	allowed bool
	code    int     // HTTP status yang akan dikembalikan bila ditolak
	oneOff  bool    // true = pakai one-off boost Rp1.000
	days    int     // durasi boost (hari); 0 bila ditolak
	cost    float64 // biaya yang harus dibayarkan
}

// boostPolicy adalah inti logika BoostProduct yang bersih dari DB:
//   - free (non-verified, non-admin): hanya boleh one-off (Rp1.000)
//   - verified (non-admin): max 2/day on non-one-off boosts
//   - admin: tidak terbatas
const oneOffBoostPrice = 1000.0

func boostPolicy(isVerified, isAdmin, oneOff bool, days, todayBoosts int, perDayPrice, oneOffPrice, maxVerifiedPerDay float64) boostPolicyResult {
	if oneOff {
		return boostPolicyResult{
			allowed: true, oneOff: true, days: 1, cost: oneOffPrice,
		}
	}
	// Free seller tidak boleh boost (selain one-off).
	if !isVerified && !isAdmin {
		return boostPolicyResult{
			allowed: false, code: http.StatusForbidden,
		}
	}
	// Verified, non-admin: cek daily limit.
	if isVerified && !isAdmin {
		maxDay := int(maxVerifiedPerDay)
		if todayBoosts >= maxDay {
			return boostPolicyResult{
				allowed: false, code: http.StatusTooManyRequests,
			}
		}
	}
	// Durasi valid?
	if days < 1 || days > 90 {
		return boostPolicyResult{
			allowed: false, code: http.StatusBadRequest,
		}
	}
	return boostPolicyResult{
		allowed: true, oneOff: false, days: days, cost: perDayPrice * float64(days),
	}
}

func TestBoostPolicy(t *testing.T) {
	const perDay = 5000.0
	const maxDay = 2.0

	cases := []struct {
		name       string
		verified   bool
		admin      bool
		oneOff     bool
		days       int
		today      int
		wantAllow  bool
		wantCode   int
		wantOneOff bool
		wantDays   int
		wantCost   float64
	}{
		// Free seller: tidak boleh boost (non one-off).
		{"free seller blocked", false, false, false, 7, 0, false, http.StatusForbidden, false, 0, 0},
		// Free seller: boleh one-off Rp1.000.
		{"free seller one-off ok", false, false, true, 0, 0, true, 0, true, 1, oneOffBoostPrice},
		// Verified: boleh boost, under daily limit.
		{"verified boost ok", true, false, false, 7, 0, true, 0, false, 7, perDay * 7},
		// Verified: limit harian tercapai.
		{"verified limit hit", true, false, false, 7, 2, false, http.StatusTooManyRequests, false, 0, 0},
		// Verified: satu lagi sebelum limit (today=1, max=2 → masih boleh).
		{"verified one short of limit", true, false, false, 3, 1, true, 0, false, 3, perDay * 3},
		// Verified: tetap boleh one-off meski limit hari ini penuh.
		{"verified one-off over limit", true, false, true, 0, 2, true, 0, true, 1, oneOffBoostPrice},
		// Admin: tidak terbatas, boleh 3x meski melebihi limit verified.
		{"admin beyond limit", true, true, false, 5, 2, true, 0, false, 5, perDay * 5},
		// Admin: boleh one-off juga.
		{"admin one-off", true, true, true, 0, 0, true, 0, true, 1, oneOffBoostPrice},
		// Durasi invalid (verified, under limit).
		{"verified durasi 0", true, false, false, 0, 0, false, http.StatusBadRequest, false, 0, 0},
		{"verified durasi >90", true, false, false, 91, 0, false, http.StatusBadRequest, false, 0, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := boostPolicy(c.verified, c.admin, c.oneOff, c.days, c.today, perDay, oneOffBoostPrice, maxDay)
			if got.allowed != c.wantAllow {
				t.Errorf("allowed: got %v, want %v", got.allowed, c.wantAllow)
			}
			if !c.wantAllow && got.code != c.wantCode {
				t.Errorf("code: got %d, want %d", got.code, c.wantCode)
			}
			if c.wantAllow {
				if got.oneOff != c.wantOneOff {
					t.Errorf("oneOff: got %v, want %v", got.oneOff, c.wantOneOff)
				}
				if got.days != c.wantDays {
					t.Errorf("days: got %d, want %d", got.days, c.wantDays)
				}
				if got.cost != c.wantCost {
					t.Errorf("cost: got %v, want %v", got.cost, c.wantCost)
				}
			}
		})
	}
}

func TestBoostCreditCount(t *testing.T) {
	for count, hours := range map[int]int{1: 6, 2: 12, 50: 300} {
		if !boostCountAllowed(count) || boostHours(count) != hours {
			t.Fatalf("count=%d: allowed=%v hours=%d", count, boostCountAllowed(count), boostHours(count))
		}
	}
	for _, count := range []int{-1, 0, 51} {
		if boostCountAllowed(count) {
			t.Fatalf("count %d must be rejected", count)
		}
	}
}
