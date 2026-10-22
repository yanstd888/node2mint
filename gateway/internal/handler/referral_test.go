package handler

import (
	"net/http"
	"testing"
)

func TestReferralEligible(t *testing.T) {
	cases := []struct {
		name     string
		invIP    string
		invteeIP string
		self     bool
		reused   bool
		want     bool
	}{
		{"IP berbeda — layak", "hashA", "hashB", false, false, true},
		{"IP sama — dummy, ditolak", "hashA", "hashA", false, false, false},
		{"IP invitee sudah dipakai akun lain — ditolak", "hashA", "hashB", false, true, false},
		{"Self-referral — ditolak", "hashA", "hashA", true, false, false},
		// Fail closed: IP tidak tersimpan = kesamaan IP tak bisa dibuktikan.
		// Dulu dua kasus ini lolos, jadi akun tanpa signup_ip_hash (mis. hasil
		// OAuth lama) bisa memanen bonus berulang dari satu mesin.
		{"IP inviter kosong — ditolak (fail closed)", "", "hashB", false, false, false},
		{"IP invitee kosong — ditolak (fail closed)", "hashA", "", false, false, false},
		{"IP kedua sisi kosong — ditolak", "", "", false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := referralEligible(c.invIP, c.invteeIP, c.self, c.reused); got != c.want {
				t.Fatalf("referralEligible(%q,%q,%v,%v) = %v, mau %v", c.invIP, c.invteeIP, c.self, c.reused, got, c.want)
			}
		})
	}
}

func TestSignupIPHashMissingRemoteAddrFailsClosed(t *testing.T) {
	if got := signupIPHash(&http.Request{}); got != "" {
		t.Fatalf("missing RemoteAddr must not become a reusable hash: %q", got)
	}
}
