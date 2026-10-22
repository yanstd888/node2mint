package handler

import (
	"net/http"

	"ai-sell/gateway/internal/auth"
	"ai-sell/gateway/internal/provider"
)

// referralEligible: satu gerbang anti-farming untuk SEMUA jalur bonus undangan
// (bayar saat daftar lewat affiliateSignup, dan lunasi tertunda lewat
// payDeferredReferralBonus).
//
// Fail closed: kalau salah satu IP tidak tersimpan, kesamaan IP tidak bisa
// dibuktikan — jadi bonusnya TIDAK dibayar. Sebelumnya IP kosong dianggap
// "layak", sehingga akun tanpa signup_ip_hash (mis. hasil OAuth lama) bisa
// meloloskan referral berulang dari satu mesin.
func referralEligible(inviterIP, inviteeIP string, selfRef, reusedIP bool) bool {
	if selfRef || reusedIP || inviterIP == "" || inviteeIP == "" {
		return false
	}
	return inviterIP != inviteeIP
}

// signupIPHash: IP pendaftar → hash (tanpa raw IP di DB), sama untuk semua
// jalur daftar. Dipakai sebagai kolom user.signup_ip_hash dan kunci anti-reuse.
func signupIPHash(r *http.Request) string {
	ip := ipOf(r)
	if ip == "" {
		return ""
	}
	return provider.HashKey(ip + auth.Secret)
}
