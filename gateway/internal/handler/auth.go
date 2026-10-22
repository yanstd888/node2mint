package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"ai-sell/gateway/internal/auth"
	"ai-sell/gateway/internal/secret"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type DB = *pgxpool.Pool

type registerInput struct {
	Email           string `json:"email"`
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"password_confirm"`
	AffCode         string `json:"aff_code,omitempty"`
	TurnstileToken  string `json:"turnstile_token,omitempty"`
}

func applySignupBenefits(ctx context.Context, db DB, userID, affCode, ipHash, source string) {
	referralUsed := false
	if ac := strings.ToUpper(strings.TrimSpace(affCode)); ac != "" {
		var err error
		referralUsed, err = affiliateSignup(ctx, db, userID, ac, ipHash)
		if err != nil {
			log.Printf("[WARN] referral signup gagal (%s): user=%s err=%v", source, userID, err)
			referralUsed = true // fail closed: jangan beri bonus signup saat audit referral gagal.
		}
	}
	if referralUsed {
		return
	}
	var reusedIP bool
	_ = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM "user" WHERE signup_ip_hash=$1 AND id<>$2)`, ipHash, userID).Scan(&reusedIP)
	if !reusedIP {
		if err := grantSignupFreePackage(ctx, db, userID); err != nil {
			log.Printf("[WARN] free signup package gagal (%s): user=%s err=%v", source, userID, err)
		}
	}
}

func validateRegistration(in registerInput) (registerInput, string) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.FirstName = strings.TrimSpace(in.FirstName)
	in.LastName = strings.TrimSpace(in.LastName)
	in.AffCode = strings.ToUpper(strings.TrimSpace(in.AffCode))

	addr, err := mail.ParseAddress(in.Email)
	if err != nil || addr.Address != in.Email {
		return in, "format email tidak valid"
	}
	if len(in.FirstName) < 2 || len(in.FirstName) > 120 || len(in.LastName) > 120 {
		return in, "nama lengkap wajib 2-120 karakter"
	}
	if len(in.Password) < 8 || len(in.Password) > 72 {
		return in, "password wajib 8-72 karakter"
	}
	if in.Password != in.PasswordConfirm {
		return in, "verifikasi password tidak sama"
	}
	return in, ""
}

func validationCode(message string) string {
	switch message {
	case "format email tidak valid":
		return "email_invalid"
	case "nama lengkap wajib 2-120 karakter":
		return "name_invalid"
	case "password wajib 8-72 karakter":
		return "password_invalid"
	case "verifikasi password tidak sama":
		return "password_mismatch"
	default:
		return "validation_failed"
	}
}

func Login(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Email          string `json:"email"`
			Password       string `json:"password"`
			TOTP           string `json:"totp,omitempty"`
			TurnstileToken string `json:"turnstile_token,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			failCode(w, http.StatusBadRequest, "invalid_request", "bad json: "+err.Error())
			return
		}
		in.Email = strings.ToLower(strings.TrimSpace(in.Email))
		if err := verifyTurnstile(r.Context(), r, in.TurnstileToken, turnstileActionAuth); err != nil {
			failCode(w, http.StatusForbidden, "turnstile_failed", err.Error())
			return
		}
		var id, role, hashPw string
		var banned bool
		err := db.QueryRow(r.Context(),
			`SELECT id, role, COALESCE(password_hash,''), is_banned FROM "user" WHERE lower(email) = $1`, in.Email).
			Scan(&id, &role, &hashPw, &banned)
		if err != nil {
			failCode(w, http.StatusUnauthorized, "invalid_credentials", "email atau password salah")
			return
		}
		if banned {
			// Notifikasi ke user yang diblokir: terjimpan di notifications +
			// dikirimkan sebagai pesan error saat login. Admin menetapkan
			// banned_reason saat memblokir; di sini dibaca untuk pesan ini.
			reason := "akun Anda diblokir"
			var dbReason string
			_ = db.QueryRow(r.Context(), `SELECT COALESCE(banned_reason,'') FROM "user" WHERE id=$1`, id).Scan(&dbReason)
			if dbReason != "" {
				reason = dbReason
			}
			// Pastikan ada notifikasi tersimpan — jika admin memberi alasan
			// melalui endpoint ban, notif sudah ada; ini menutup celah kalau
			// is_banned di-set langsung atau tidak ada notif.
			genSystemNotification(r.Context(), db, id, "Akun Diblokir", reason)
			failCode(w, http.StatusForbidden, "account_banned", reason)
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(hashPw), []byte(in.Password)) != nil {
			log.Printf("[WARN] login failed: email=%s ip=%s", in.Email, ipOf(r))
			failCode(w, http.StatusUnauthorized, "invalid_credentials", "email atau password salah")
			return
		}
		// 2FA: berlaku hanya setelah dikonfirmasi satu kode.
		if totpSecret, on := totpEnabled(r, db, id); on {
			if in.TOTP == "" {
				failCode(w, http.StatusUnauthorized, "two_factor_required", "kode 2FA diperlukan")
				return
			}
			if !secret.VerifyTOTP(totpSecret, in.TOTP) {
				failCode(w, http.StatusUnauthorized, "two_factor_invalid", "kode 2FA salah")
				return
			}
		}
		token, err := auth.IssueToken(id, role)
		if err != nil {
			failCode(w, http.StatusInternalServerError, "internal_error", "gagal membuat token")
			return
		}
		// Token dicatat di user_session: itu satu-satunya cara logout/pencabutan
		// sesi bisa berlaku (RequireAuth memeriksa baris ini).
		newSession(r, db, id, token, 24*time.Hour)
		log.Printf("[INFO] login success: user_id=%s role=%s email=%s ip=%s", id, role, in.Email, ipOf(r))
		// Jalur web memakai cookie HttpOnly; token di badan respons tetap ada
		// untuk API client (curl/SDK) yang tidak punya cookie jar.
		SetSessionCookie(w, r, token)
		ok(w, map[string]any{"token": token, "role": role})
	}
}

func Register(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in registerInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			failCode(w, http.StatusBadRequest, "invalid_request", "bad json: "+err.Error())
			return
		}
		if err := verifyTurnstile(r.Context(), r, in.TurnstileToken, turnstileActionAuth); err != nil {
			failCode(w, http.StatusForbidden, "turnstile_failed", err.Error())
			return
		}
		var validationErr string
		in, validationErr = validateRegistration(in)
		if validationErr != "" {
			failCode(w, http.StatusBadRequest, validationCode(validationErr), validationErr)
			return
		}
		// Email sudah terdaftar: hentikan sebelum hashing/insert dan biarkan UI
		// menampilkan pesan yang jelas. UNIQUE di schema tetap menjadi guard race.
		var taken int
		if err := db.QueryRow(r.Context(), `SELECT 1 FROM "user" WHERE lower(email)=$1`, in.Email).Scan(&taken); err == nil {
			log.Printf("[INFO] register ditolak (email sudah ada): email=%s ip=%s", in.Email, r.RemoteAddr)
			failCode(w, http.StatusConflict, "email_registered", "email sudah terdaftar")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		if err != nil {
			failCode(w, http.StatusInternalServerError, "internal_error", "hash error")
			return
		}
		ipHash := signupIPHash(r)
		var newID string
		name := in.FirstName + " " + in.LastName
		err = db.QueryRow(r.Context(),
			`INSERT INTO "user" (email, name, first_name, last_name, role, wallet_balance, password_hash, signup_ip_hash) VALUES ($1,$2,$3,$4,'USER',0,$5,NULLIF($6,'')) RETURNING id`,
			in.Email, name, in.FirstName, in.LastName, hash, ipHash).Scan(&newID)
		if err != nil {
			log.Printf("[WARN] register failed: email=%s err=%v", in.Email, err)
			failCode(w, http.StatusConflict, "email_registered", "email sudah terdaftar")
			return
		}
		// Bonus signup dan referral saling eksklusif; aturan yang sama dipakai OAuth.
		applySignupBenefits(r.Context(), db, newID, in.AffCode, ipHash, "manual")
		log.Printf("[INFO] register success: email=%s", in.Email)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}
}

func Dashboard(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := auth.UserFromContext(r.Context())
		if claims == nil {
			browserError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var email, role string
		var bal float64
		var name string
		_ = db.QueryRow(r.Context(),
			`SELECT wallet_balance, email, role, COALESCE(name,'') FROM "user" WHERE id = $1`, claims.UserID).
			Scan(&bal, &email, &role, &name)
		var usage int
		_ = db.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM usage_log WHERE user_id = $1 AND created_at > now() - INTERVAL '24h'`,
			claims.UserID).Scan(&usage)
		// Tampilkan total token tersimpan, termasuk yang masa aktifnya habis;
		// ChargeTx tetap hanya memakai paket aktif.
		var tokenRem float64
		var tokExp any
		var tokenActive bool
		_ = db.QueryRow(r.Context(),
			`SELECT COALESCE(SUM(remaining),0), MAX(expires_at), COALESCE(BOOL_OR(expires_at > now() AND remaining > 0),false)
			   FROM user_token_package WHERE user_id=$1 AND remaining > 0`,
			claims.UserID).Scan(&tokenRem, &tokExp, &tokenActive)
		json.NewEncoder(w).Encode(map[string]any{
			"user_id":        claims.UserID,
			"email":          email,
			"name":           name,
			"role":           role,
			"balance":        bal,
			"tokenRemaining": tokenRem,
			"tokenExpiresAt": tokExp,
			"tokenActive":    tokenActive,
			"usageToday":     usage,
			"isAdmin":        role == "ADMIN" || role == "SUPER_ADMIN",
		})
	}
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

var _ = context.Background
