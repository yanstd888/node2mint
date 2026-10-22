package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"ai-sell/gateway/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Uji jalur OAuth/Telegram (oauthDone) terhadap DB nyata: IP pendaftar harus
// ter-hash, akun sosial BARU harus dapat hak signup 2M/1hari, dan IP yang sama
// dipakai akun lain = nol. Dilewati (bukan gagal) tanpa DATABASE_URL.
// Tidak ada reset DB: hanya user/kode sementara milik tes ini, dihapus lagi.

func oauthReq(ip string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback", nil)
	r.RemoteAddr = ip + ":54321"
	return r
}

// signupPackage: (jumlah baris, total token) paket 'Free - Just Signup' user.
func signupPackage(t *testing.T, ctx context.Context, db *pgxpool.Pool, uid string) (int, int) {
	t.Helper()
	var rows, tokens int
	if err := db.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(SUM(utp.remaining),0) FROM user_token_package utp
		   JOIN token_package tp ON tp.id = utp.token_package_id
		  WHERE utp.user_id=$1 AND tp.name='Free - Just Signup' AND tp.token_count=2000000`, uid).Scan(&rows, &tokens); err != nil {
		t.Fatalf("baca paket signup: %v", err)
	}
	return rows, tokens
}

func oauthFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool, prefix string) string {
	t.Helper()
	return prefix + "-" + randCode(8) + "@example.invalid"
}

func TestOAuthReferralStateRoundTrip(t *testing.T) {
	rr := httptest.NewRecorder()
	state := oauthState(rr, " ref-123 ")
	if got := oauthReferral(state); got != "REF-123" {
		t.Fatalf("referral dari state=%q, mau REF-123", got)
	}
}

func TestOAuthSignupHashesIPAndGrantsEntitlementOnce(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL tidak diset — lewati uji jalur OAuth")
	}
	ctx := context.Background()
	db, err := store.Connect(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()

	email := oauthFixture(t, ctx, db, "oauth-signup-test")
	defer func() { db.Exec(ctx, `DELETE FROM "user" WHERE email=$1`, email) }()

	// 1) Pendaftaran sosial pertama: 2M + 1 hari, IP ter-hash (bukan raw).
	first := oauthReq("10.7.0.1")
	rr := httptest.NewRecorder()
	oauthDone(rr, first, db, email, "Uji Sosial", "")
	if rr.Code != http.StatusFound {
		t.Fatalf("signup sosial: status=%d, mau 302", rr.Code)
	}
	var uid, ipHash, ipRaw string
	if err := db.QueryRow(ctx, `SELECT id, COALESCE(signup_ip_hash,''), COALESCE(signup_ip_hash,'') FROM "user" WHERE email=$1`, email).Scan(&uid, &ipHash, &ipRaw); err != nil {
		t.Fatalf("user oauth tidak dibuat: %v", err)
	}
	if want := signupIPHash(first); ipHash != want {
		t.Fatalf("signup_ip_hash=%q, mau %q (hash yang sama dengan Register)", ipHash, want)
	}
	if ipHash == "10.7.0.1" || len(ipHash) != 64 {
		t.Fatalf("IP tidak di-hash: %q", ipHash)
	}
	if rows, tokens := signupPackage(t, ctx, db, uid); rows != 1 || tokens != 2000000 {
		t.Fatalf("hak signup sosial: rows=%d tokens=%d, mau 1/2000000", rows, tokens)
	}

	// 2) Login ulang dari IP lain: BUKAN signup baru — hak tidak dobel, dan
	// jejak IP pendaftaran asli tidak ditimpa.
	again := httptest.NewRecorder()
	oauthDone(again, oauthReq("10.7.0.2"), db, email, "Uji Sosial", "")
	if again.Code != http.StatusFound {
		t.Fatalf("login ulang sosial: status=%d, mau 302", again.Code)
	}
	if rows, _ := signupPackage(t, ctx, db, uid); rows != 1 {
		t.Fatalf("hak signup dobel setelah login ulang: rows=%d, mau 1", rows)
	}
	var after string
	if err := db.QueryRow(ctx, `SELECT COALESCE(signup_ip_hash,'') FROM "user" WHERE id=$1`, uid).Scan(&after); err != nil || after != ipHash {
		t.Fatalf("IP pendaftaran tertimpa saat login ulang: %q (err=%v), mau %q", after, err, ipHash)
	}

	// 3) Akun lain dari IP yang sama: nol (reuse), walau tetap ter-hash.
	reused := oauthFixture(t, ctx, db, "oauth-reuse-test")
	defer func() { db.Exec(ctx, `DELETE FROM "user" WHERE email=$1`, reused) }()
	rr3 := httptest.NewRecorder()
	oauthDone(rr3, oauthReq("10.7.0.1"), db, reused, "Uji Reuse", "")
	if rr3.Code != http.StatusFound {
		t.Fatalf("signup IP daur ulang: status=%d, mau 302", rr3.Code)
	}
	var uid3 string
	if err := db.QueryRow(ctx, `SELECT id FROM "user" WHERE email=$1`, reused).Scan(&uid3); err != nil {
		t.Fatalf("user daur ulang tidak dibuat: %v", err)
	}
	if rows, tokens := signupPackage(t, ctx, db, uid3); rows != 0 || tokens != 0 {
		t.Fatalf("IP daur ulang dapat bonus: rows=%d tokens=%d, mau 0/0", rows, tokens)
	}
}

func TestOAuthSignupUsesReferralCode(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL tidak diset — lewati uji referral OAuth")
	}
	ctx := context.Background()
	db, err := store.Connect(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()

	inviter := oauthFixture(t, ctx, db, "oauth-ref-inviter")
	invitee := oauthFixture(t, ctx, db, "oauth-ref-invitee")
	code := "OR" + randCode(6)
	defer func() {
		db.Exec(ctx, `DELETE FROM referral_code WHERE code=$1`, code)
		db.Exec(ctx, `DELETE FROM "user" WHERE email IN ($1,$2)`, inviter, invitee)
	}()

	var inviterID string
	if err := db.QueryRow(ctx,
		`INSERT INTO "user" (email, role, wallet_balance, signup_ip_hash) VALUES ($1,'USER',0,$2) RETURNING id`,
		inviter, signupIPHash(oauthReq("10.8.0.1"))).Scan(&inviterID); err != nil {
		t.Fatalf("seed inviter: %v", err)
	}
	if _, err := db.Exec(ctx,
		`INSERT INTO referral_code (code, created_by, type, reward) VALUES ($1,$2,'SIGNUP',1000000)`,
		code, inviterID); err != nil {
		t.Fatalf("seed kode: %v", err)
	}

	rr := httptest.NewRecorder()
	oauthDone(rr, oauthReq("10.8.0.2"), db, invitee, "OAuth Referral", code)
	if rr.Code != http.StatusFound {
		t.Fatalf("signup OAuth referral: status=%d, mau 302", rr.Code)
	}
	var inviteeID string
	if err := db.QueryRow(ctx, `SELECT id FROM "user" WHERE email=$1`, invitee).Scan(&inviteeID); err != nil {
		t.Fatalf("invitee tidak dibuat: %v", err)
	}
	var referrals int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM referral_signup WHERE invitee_id=$1 AND inviter_id=$2`, inviteeID, inviterID).Scan(&referrals); err != nil || referrals != 1 {
		t.Fatalf("referral OAuth tercatat=%d err=%v, mau 1", referrals, err)
	}
	if rows, _ := signupPackage(t, ctx, db, inviteeID); rows != 0 {
		t.Fatalf("invitee referral mendapat paket signup biasa: rows=%d, mau 0", rows)
	}
	for _, uid := range []string{inviterID, inviteeID} {
		var rows, tokens int
		var oneDay bool
		if err := db.QueryRow(ctx, `SELECT COUNT(*), COALESCE(SUM(utp.remaining),0),
			BOOL_AND(utp.expires_at > now()+interval '23 hours' AND utp.expires_at < now()+interval '25 hours')
			FROM user_token_package utp JOIN token_package tp ON tp.id=utp.token_package_id
			WHERE utp.user_id=$1 AND tp.name='Reward Referral' AND tp.token_count=1000000`, uid).Scan(&rows, &tokens, &oneDay); err != nil {
			t.Fatalf("baca reward referral: %v", err)
		}
		if rows != 1 || tokens != 1000000 || !oneDay {
			t.Fatalf("reward referral user=%s: rows=%d tokens=%d oneDay=%v, mau 1/1000000/true", uid, rows, tokens, oneDay)
		}
	}
}

// Inviter tanpa signup_ip_hash (akun era sebelum kolom itu ada) TIDAK boleh
// membuat referralnya layak — kesamaan IP tak bisa dibuktikan.
func TestReferralMissingInviterIPPaysNothing(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL tidak diset — lewati uji referral")
	}
	ctx := context.Background()
	db, err := store.Connect(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()

	inviter := oauthFixture(t, ctx, db, "ref-noip-inviter")
	invitee := oauthFixture(t, ctx, db, "ref-noip-invitee")
	code := "ZZ" + randCode(6)
	defer func() {
		db.Exec(ctx, `DELETE FROM referral_code WHERE code=$1`, code)
		db.Exec(ctx, `DELETE FROM "user" WHERE email IN ($1,$2)`, inviter, invitee)
	}()

	// Inviter sengaja tanpa IP (NULL), invitee punya IP.
	var inviterID string
	if err := db.QueryRow(ctx,
		`INSERT INTO "user" (email, role, wallet_balance, signup_ip_hash) VALUES ($1,'USER',0,NULL) RETURNING id`,
		inviter).Scan(&inviterID); err != nil {
		t.Fatalf("seed inviter: %v", err)
	}
	var inviteeID string
	if err := db.QueryRow(ctx,
		`INSERT INTO "user" (email, role, wallet_balance, signup_ip_hash) VALUES ($1,'USER',0,'hashB') RETURNING id`,
		invitee).Scan(&inviteeID); err != nil {
		t.Fatalf("seed invitee: %v", err)
	}
	if _, err := db.Exec(ctx,
		`INSERT INTO referral_code (code, created_by, type, reward) VALUES ($1,$2,'SIGNUP',1000000)`,
		code, inviterID); err != nil {
		t.Fatalf("seed kode referral: %v", err)
	}

	used, err := affiliateSignup(ctx, db, inviteeID, code, "hashB")
	if err != nil {
		t.Fatalf("affiliateSignup: %v", err)
	}
	if !used {
		t.Fatal("affiliateSignup = false, mau true (dicatat, bonus ditahan)")
	}
	for _, uid := range []string{inviterID, inviteeID} {
		var n int
		if err := db.QueryRow(ctx,
			`SELECT COUNT(*) FROM user_token_package utp JOIN token_package tp ON tp.id=utp.token_package_id
			  WHERE utp.user_id=$1 AND tp.name='Reward Referral'`, uid).Scan(&n); err != nil {
			t.Fatalf("baca reward referral: %v", err)
		}
		if n != 0 {
			t.Fatalf("IP inviter kosong tetap dibayar: user=%s baris=%d, mau 0", uid, n)
		}
	}
}
