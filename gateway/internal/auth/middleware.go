// Middleware chi: verify Bearer token, set user di context.
package auth

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"ai-sell/gateway/internal/apilog"
	"ai-sell/gateway/internal/provider"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ctxKey struct{}

// CookieName: cookie HttpOnly tempat web menyimpan JWT. Browser mengirimnya
// otomatis (lewat rewrite /api Next), jadi skrip halaman tidak pernah melihat
// token — XSS tidak bisa mencuri sesi. API client tetap memakai Bearer.
const CookieName = "aisell_session"

// RawToken: Bearer header menang (dipakai API client & skrip), kalau tidak ada
// cookie sesi. Kosong = tidak ada kredensial sama sekali.
func RawToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if c, err := r.Cookie(CookieName); err == nil {
		return c.Value
	}
	return ""
}

func UserFromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(ctxKey{}).(*Claims)
	return c
}

// APIKeyFromContext mengambil API key info yang sudah diverifikasi.
func APIKeyFromContext(ctx context.Context) (userID string, scopes string) {
	userID, _ = ctx.Value(apiKeyUserKey{}).(string)
	scopes, _ = ctx.Value(apiKeyScopesKey{}).(string)
	return
}

type apiKeyUserKey struct{}
type apiKeyScopesKey struct{}
type apiKeyIDKey struct{}
type apiKeyModelsKey struct{}
type apiKeyRateKey struct{}

// APIKeyExtraFromContext: whitelist model CSV (” = semua) + rate limit per key
// (0 = pakai default global). Diisi RequireAPIKey dari baris api_key.
func APIKeyExtraFromContext(ctx context.Context) (models string, rateLimit int) {
	models, _ = ctx.Value(apiKeyModelsKey{}).(string)
	rateLimit, _ = ctx.Value(apiKeyRateKey{}).(int)
	return
}

// APIKeyIDFromContext: id baris api_key — dipakai billing untuk menegakkan
// credit_limit/credit_used key itu.
func APIKeyIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(apiKeyIDKey{}).(string)
	return id
}

// Middleware required auth.
//
// JWT tidak punya daftar cabut: token yang sudah diterbitkan tetap sah sampai
// kedaluwarsa. Baris user_session adalah daftar cabut itu — tanpa pemeriksaan
// ini, DELETE /api/account/sessions/{id} tidak menghentikan apa pun.
func RequireAuth(db *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := RawToken(r)
			if raw == "" {
				WriteBrowserError(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			claims, err := VerifyToken(raw)
			if err != nil {
				WriteBrowserError(w, "invalid token", http.StatusUnauthorized)
				return
			}
			// Peran dibaca ulang dari DB, bukan dari klaim token: kalau tidak,
			// admin yang perannya dicabut tetap bisa menyetujui penarikan uang
			// sampai token kedaluwarsa.
			//
			// is_banned juga dicek di sini: akun yang diblokir tidak bisa pakai
			// sesi sama pun — semua request ke endpoint terlindungi ditolak.
			var role string
			var banned bool
			var bannedReason sql.NullString
			err = db.QueryRow(r.Context(),
				`SELECT u.role, u.is_banned, u.banned_reason FROM user_session s JOIN "user" u ON u.id = s.user_id
				  WHERE s.session_token = $1 AND s.expires > now()`,
				provider.HashKey(raw)).Scan(&role, &banned, &bannedReason)
			if err != nil {
				WriteBrowserError(w, "sesi tidak berlaku — masuk ulang", http.StatusUnauthorized)
				return
			}
			if banned {
				reason := "akun Anda diblokir"
				if bannedReason.Valid && bannedReason.String != "" {
					reason = bannedReason.String
				}
				WriteBrowserError(w, reason, http.StatusForbidden)
				return
			}
			claims.Role = role
			ctx := context.WithValue(r.Context(), ctxKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AdminOnly.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := UserFromContext(r.Context())
		if claims == nil || (claims.Role != "ADMIN" && claims.Role != "SUPER_ADMIN") {
			WriteBrowserError(w, "admin access required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireScope membatasi API key pada scope endpoint yang diminta.
func RequireScope(required string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, scopes := APIKeyFromContext(r.Context())
			for _, scope := range strings.Split(scopes, ",") {
				if strings.TrimSpace(scope) == required {
					next.ServeHTTP(w, r)
					return
				}
			}
			apilog.SetError(r.Context(), "permission_denied")
			WriteV1Error(w, http.StatusForbidden, "api key scope not permitted", "permission_denied")
		})
	}
}

// RequireAPIKey: *** via API key (alternatif JWT).
// Menerima header "Authorization: Bearer ***" atau "x-api-key: ***".
// Validasi hash di database. Jika valid, set userID dan scopes di context.
// Untuk penggunaan di handler, panggil auth.APIKeyFromContext(r.Context()).
func RequireAPIKey(db *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			meta := apilog.From(r.Context())
			rawKey := ""
			h := r.Header.Get("Authorization")
			if strings.HasPrefix(h, "Bearer ") {
				rawKey = strings.TrimPrefix(h, "Bearer ")
			}
			if rawKey == "" {
				rawKey = r.Header.Get("X-API-Key")
			}
			if rawKey == "" {
				apilog.SetError(r.Context(), "missing_api_key")
				WriteV1Error(w, http.StatusUnauthorized, "missing api key", "missing_api_key")
				return
			}

			hash := provider.HashKey(rawKey)
			var userID, scopes, keyID, models string
			var used float64
			var limit *float64
			var rateLimit int
			err := db.QueryRow(r.Context(),
				`SELECT user_id, scopes, id, credit_used, credit_limit, COALESCE(models,''), rate_limit FROM api_key
				  WHERE key_hash = $1 AND is_active = true AND revoked_at IS NULL`,
				hash).Scan(&userID, &scopes, &keyID, &used, &limit, &models, &rateLimit)
			if err != nil {
				var active bool
				var revokedAt sql.NullTime
				stateErr := db.QueryRow(r.Context(), `SELECT is_active, revoked_at FROM api_key WHERE key_hash=$1`, hash).Scan(&active, &revokedAt)
				if stateErr == nil && revokedAt.Valid {
					apilog.SetError(r.Context(), "key_revoked")
					WriteV1Error(w, http.StatusUnauthorized, "api key revoked", "key_revoked")
					return
				}
				if stateErr == nil && !active {
					apilog.SetError(r.Context(), "key_disabled")
					WriteV1Error(w, http.StatusUnauthorized, "api key disabled", "key_disabled")
					return
				}
				apilog.SetError(r.Context(), "invalid_api_key")
				WriteV1Error(w, http.StatusUnauthorized, "invalid api key", "invalid_api_key")
				return
			}
			if meta != nil {
				meta.UserID = userID
				meta.KeyID = keyID
			}
			// Akun yang diblokir tidak bisa pakai API key — periksa is_banned setiap
			// request (bukan cukup di Login), karena key bisa dipakai langsung lewat
			// header tanpa lewat sesi JWT.
			var banned bool
			var bannedReason sql.NullString
			_ = db.QueryRow(r.Context(),
				`SELECT is_banned, banned_reason FROM "user" WHERE id=$1`, userID).Scan(&banned, &bannedReason)
			if banned {
				reason := "akun Anda diblokir"
				if bannedReason.Valid && bannedReason.String != "" {
					reason = bannedReason.String
				}
				apilog.SetError(r.Context(), "account_banned")
				WriteV1Error(w, http.StatusForbidden, reason, "account_banned")
				return
			}
			// Kuota key habis: 402 — sama seperti saldo kurang, bukan 401/403.
			if limit != nil && used >= *limit {
				apilog.SetError(r.Context(), "credit_limit_reached")
				WriteV1Error(w, http.StatusPaymentRequired, "api key credit limit reached", "credit_limit_reached")
				return
			}

			ctx := context.WithValue(r.Context(), apiKeyUserKey{}, userID)
			ctx = context.WithValue(ctx, apiKeyScopesKey{}, scopes)
			ctx = context.WithValue(ctx, apiKeyIDKey{}, keyID)
			ctx = context.WithValue(ctx, apiKeyModelsKey{}, models)
			ctx = context.WithValue(ctx, apiKeyRateKey{}, rateLimit)

			// Update last_used secara async.
			go func() {
				_, _ = db.Exec(context.Background(),
					`UPDATE api_key SET last_used = now() WHERE key_hash = $1`, hash)
			}()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
