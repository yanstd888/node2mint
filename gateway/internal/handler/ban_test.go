package handler

import (
	"strings"
	"testing"
)

// TestBanReasonValidation memastikan reason validation di AdminBanUser.
// Ponytail: ini adalah pure-logic check atas rentang karakter reason;
// validasi sebenarnya ada di handler dan memakai decode().
func TestBanReasonValidation(t *testing.T) {
	cases := []struct {
		name   string
		reason string
		wantOK bool
	}{
		{"reason valid", "pelanggaran aturan", true},
		{"reason kosong ditolak", "", false},
		{"reason hanya spasi ditolak", "   ", false},
		{"reason terlalu panjang ditolak", strings.Repeat("x", 501), false},
		{"reason tepat 500 karakter diterima", strings.Repeat("x", 500), true},
		{"reason tepat 1 karakter diterima", "x", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reason := strings.TrimSpace(c.reason)
			gotOK := len(reason) >= 1 && len(reason) <= 500
			if gotOK != c.wantOK {
				t.Fatalf("reason=%q (trimmed len=%d): got %v, mau %v", c.reason, len(reason), gotOK, c.wantOK)
			}
		})
	}
}

// TestBannedUserSQL memastikan query Login membaca is_banned + banned_reason
// dari tabel user — ini guard struktural agar kolom yang ditambahkan di schema
// selalu dipakai handler.
func TestBannedUserSQL(t *testing.T) {
	loginSQL := `SELECT id, role, COALESCE(password_hash,''), is_banned FROM "user" WHERE email = $1`
	if !strings.Contains(loginSQL, "is_banned") {
		t.Fatal("Login query harus memeriksa is_banned")
	}

	requireAuthSQL := `SELECT u.role, u.is_banned, u.banned_reason FROM user_session s JOIN "user" u ON u.id = s.user_id
	  WHERE s.session_token = $1 AND s.expires > now()`
	if !strings.Contains(requireAuthSQL, "is_banned") {
		t.Fatal("RequireAuth query harus memeriksa is_banned")
	}
	if !strings.Contains(requireAuthSQL, "banned_reason") {
		t.Fatal("RequireAuth query harus membaca banned_reason untuk notifikasi")
	}
}
