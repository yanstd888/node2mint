package handler

import (
	"testing"
	"time"
)

// sparkBuckets: pasangan jam|status → 24 ember, indeks terakhir = jam sekarang.
func TestSparkBuckets(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	fmt := func(tm time.Time) string { return tm.Format("2006-01-02-15") }

	got := sparkBuckets([]string{
		fmt(now) + "|ok",                      // jam sekarang → ember 23
		fmt(now.Add(-3*time.Hour)) + "|down",  // 3 jam lalu → ember 20
		fmt(now.Add(-23*time.Hour)) + "|slow", // hampir 24 jam → ember 0
		fmt(now.Add(-99*time.Hour)) + "|down", // di luar jendela → diabaikan
		"bukan-tanggal|ok",                    // rusak → diabaikan
	})

	if len(got) != 24 {
		t.Fatalf("panjang = %d, mau 24", len(got))
	}
	if got[23] != "ok" || got[20] != "down" || got[0] != "slow" {
		t.Fatalf("ember salah: [0]=%q [20]=%q [23]=%q", got[0], got[20], got[23])
	}
	if got[5] != "" {
		t.Fatalf("ember tanpa probe harus kosong, dapat %q", got[5])
	}
}

// Tanpa probe sama sekali → 24 ember kosong (bukan panik / bukan 24 elemen salah).
func TestSparkBucketsKosong(t *testing.T) {
	got := sparkBuckets(nil)
	if len(got) != 24 {
		t.Fatalf("panjang = %d, mau 24", len(got))
	}
	for i, s := range got {
		if s != "" {
			t.Fatalf("ember %d = %q, mau kosong", i, s)
		}
	}
}
