package handler

import "testing"

// Diskon cache: token cached (subset dari prompt_tokens) dibaca lebih murah.
func TestBillableTokensDiskonCache(t *testing.T) {
	cases := []struct {
		nama         string
		disc         float64
		in, out, cac int
		mau          float64
	}{
		{"tanpa cache", 0.5, 1000, 500, 0, 1500},
		{"cache 800 dari in 1000, diskon 0.5", 0.5, 1000, 500, 800, 1100}, // (200 + 400) + 500
		{"semua input cache, diskon 0.5", 0.5, 1000, 0, 1000, 500},
		{"diskon 1 = perilaku lama", 1, 1000, 500, 800, 1500},
		{"diskon 0 = cache gratis", 0, 1000, 500, 400, 1100}, // 600 + 500
		{"diskon ngawur >1 dibatasi 1", 3, 1000, 500, 800, 1500},
		{"cache > input diabaikan (data aneh)", 0.5, 100, 10, 500, 110},
	}
	for _, c := range cases {
		// Surcharge 0 → behavior sama seperti semula (tanpa surcharge).
		if got := billableTokens(c.in, c.out, c.cac, c.disc, 0, 1); got != c.mau {
			t.Errorf("%s: billableTokens(%d,%d,cache=%d,disc=%v) = %v, mau %v",
				c.nama, c.in, c.out, c.cac, c.disc, got, c.mau)
		}
	}
}

// Surcharge 25% diterapkan SEBELUM multiplier.
func TestBillableTokensSurcharge(t *testing.T) {
	cases := []struct {
		nama      string
		in, out   int
		cac       int
		disc      float64
		surcharge float64
		mult      float64
		mau       float64
	}{
		// 1000 in + 500 out (cac=0, disc=0) = 1500; ×1.25 = 1875; ×1 = 1875
		{"surcharge 25%, mult 1", 1000, 500, 0, 0, 0.25, 1, 1875},
		// 1500 × 1.25 = 1875; ×3 = 5625
		{"surcharge 25%, mult 3", 1000, 500, 0, 0, 0.25, 3, 5625},
		// surcharge 0 → sama seperti semula
		{"surcharge 0, mult 1", 1000, 500, 0, 0.5, 0, 1, 1500},
		// cache 800, diskon 0.5 → (200+400)+500 = 1100; ×1.25 = 1375; ×1 = 1375
		{"cache diskon + surcharge", 1000, 500, 800, 0.5, 0.25, 1, 1375},
	}
	for _, c := range cases {
		got := billableTokens(c.in, c.out, c.cac, c.disc, c.surcharge, c.mult)
		if got != c.mau {
			t.Errorf("%s: billableTokens(%d,%d,cache=%d,disc=%v,surcharge=%v,mult=%v) = %v, mau %v",
				c.nama, c.in, c.out, c.cac, c.disc, c.surcharge, c.mult, got, c.mau)
		}
	}
}
