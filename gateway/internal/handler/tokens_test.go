package handler

import "testing"

func TestUsageSurcharge(t *testing.T) {
	cases := []struct {
		name string
		tok  float64
		rate float64
		want float64
	}{
		{"0 token", 0, 0.25, 0},
		{"token negatif", -100, 0.25, -100},
		{"rate 0 = tidak ada surcharge", 1000, 0, 1000},
		{"rate negatif = tidak ada surcharge", 1000, -0.1, 1000},
		{"25% surcharge", 1000, 0.25, 1250},
		{"100% surcharge", 1000, 1.0, 2000},
		{"0.5 surcharge pada 2.5M", 2500000, 0.25, 3125000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := usageSurcharge(c.tok, c.rate)
			if got != c.want {
				t.Errorf("usageSurcharge(%v,%v) = %v, mau %v", c.tok, c.rate, got, c.want)
			}
		})
	}
}

func TestEffectiveCostWithMultiplier(t *testing.T) {
	cases := []struct {
		name          string
		tokens        float64
		surchargeRate float64
		multiplier    float64
		want          float64
	}{
		{"surcharge 25% + multiplier 1", 1000, 0.25, 1, 1250},
		{"surcharge 25% + multiplier 3", 1000, 0.25, 3, 3750},
		{"surcharge 0 + multiplier 1", 1000, 0, 1, 1000},
		{"surcharge 25% + multiplier 0 → pakai 1", 1000, 0.25, 0, 1250},
		{"token 0", 0, 0.25, 1, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := effectiveCostWithMultiplier(c.tokens, c.surchargeRate, c.multiplier)
			if got != c.want {
				t.Errorf("effectiveCostWithMultiplier(%v,%v,%v) = %v, mau %v",
					c.tokens, c.surchargeRate, c.multiplier, got, c.want)
			}
		})
	}
}

func TestPackageDurationAction(t *testing.T) {
	if got := packageDurationAction(false); got != DurationOverwrite {
		t.Errorf("packageDurationAction(false) = %v, mau %v", got, DurationOverwrite)
	}
	if got := packageDurationAction(true); got != DurationExtend {
		t.Errorf("packageDurationAction(true) = %v, mau %v", got, DurationExtend)
	}
}

func TestExtendExpiry(t *testing.T) {
	// Unix epoch 1700000000 + 30 hari
	now := int64(1700000000)
	got := extendExpiry(now, 30)
	want := now + 30*86400
	if got != want {
		t.Errorf("extendExpiry(%d, 30) = %d, mau %d", now, got, want)
	}
	// 96 hari
	got = extendExpiry(now, 96)
	want = now + 96*86400
	if got != want {
		t.Errorf("extendExpiry(%d, 96) = %d, mau %d", now, got, want)
	}
}

func TestAccumulatedExpiryAndInactivePenalty(t *testing.T) {
	now := int64(1_700_000_000)
	if got, want := accumulatedExpiry(now, now+86400, 2), now+3*86400; got != want {
		t.Fatalf("active duration dipotong: got %d want %d", got, want)
	}
	if got, want := accumulatedExpiry(now, now-86400, 2), now+2*86400; got != want {
		t.Fatalf("expired duration tidak mulai dari sekarang: got %d want %d", got, want)
	}
	if got := inactivePenalty(10_000, 2, 25_000); got != 10_000 {
		t.Fatalf("denda boleh melebihi token: got %d", got)
	}
	if got := inactivePenalty(100_000, 2, 25_000); got != 50_000 {
		t.Fatalf("denda 2 hari: got %d", got)
	}
	// days=0 (komisi token) tidak menambah masa aktif bila paket masih aktif.
	if got := accumulatedExpiry(now, now+5*86400, 0); got != now+5*86400 {
		t.Fatalf("days=0 mengubah expiry: got %d", got)
	}
}

func TestReferralReward(t *testing.T) {
	// Default: 2.5M
	if got := referralReward(0); got != defaultReferralReward {
		t.Errorf("referralReward(0) = %v, mau %v", got, defaultReferralReward)
	}
	if got := referralReward(-1); got != defaultReferralReward {
		t.Errorf("referralReward(-1) = %v, mau %v", got, defaultReferralReward)
	}
	// Configured overrides default
	if got := referralReward(1000000); got != 1000000 {
		t.Errorf("referralReward(1000000) = %v, mau 1000000", got)
	}
	if got := referralReward(5000000); got != 5000000 {
		t.Errorf("referralReward(5000000) = %v, mau 5000000", got)
	}
}
