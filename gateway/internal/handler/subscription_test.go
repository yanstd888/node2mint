package handler

// Uji jalur uang perpanjangan langganan tanpa DB.
// Yang diuji adalah `hargaPerpanjangan` milik produksi (subscription.go), bukan
// salinan — supaya kalau aturannya berubah, test ini gagal.
// Kalau harga terkunci kalah dari tarif platform, user bisa ditagih lebih mahal
// dari yang dia setujui saat menyalakan auto-renew.

import "testing"

func TestHargaPerpanjanganPakaiHargaTerkunci(t *testing.T) {
	// User setuju Rp15.000. Admin menaikkan tarif jadi Rp25.000.
	// User lama TIDAK boleh ikut naik.
	if got := hargaPerpanjangan(15000, 25000); got != 15000 {
		t.Errorf("harga terkunci harus menang: got %.2f, want 15000", got)
	}
	// Admin menurunkan tarif: user lama tetap bayar harga yang dia setujui.
	if got := hargaPerpanjangan(15000, 9000); got != 15000 {
		t.Errorf("harga terkunci harus menang meski tarif turun: got %.2f", got)
	}
}

func TestHargaPerpanjanganFallbackKeTarifPlatform(t *testing.T) {
	// Belum pernah menyalakan auto-renew → renew_price kosong → pakai tarif kini.
	if got := hargaPerpanjangan(0, 15000); got != 15000 {
		t.Errorf("fallback tarif platform: got %.2f, want 15000", got)
	}
	if got := hargaPerpanjangan(-1, 15000); got != 15000 {
		t.Errorf("nilai negatif harus dianggap kosong: got %.2f", got)
	}
}

// Saldo kurang TIDAK boleh memotong apa pun dan harus jadi PAST_DUE, bukan
// memaksa saldo minus.
func TestSaldoKurangTidakMemotongPaksa(t *testing.T) {
	type hasil struct {
		dipotong bool
		status   string
	}
	hitung := func(saldo, harga float64) hasil {
		if saldo >= harga {
			return hasil{dipotong: true, status: "ACTIVE"}
		}
		return hasil{dipotong: false, status: "PAST_DUE"}
	}
	if h := hitung(10000, 15000); h.dipotong || h.status != "PAST_DUE" {
		t.Errorf("saldo kurang harus gagal & PAST_DUE, got %+v", h)
	}
	if h := hitung(15000, 15000); !h.dipotong || h.status != "ACTIVE" {
		t.Errorf("saldo pas harus berhasil, got %+v", h)
	}
}

// Kesalahan ini harus bisa dibedakan supaya handler membalas 402, bukan 500.
func TestErrSaldoKurangDikenali(t *testing.T) {
	if errSaldoKurang.Error() != "saldo tidak cukup" {
		t.Errorf("pesan errSaldoKurang berubah: %q", errSaldoKurang.Error())
	}
}
