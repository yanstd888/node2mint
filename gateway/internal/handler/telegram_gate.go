package handler

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"ai-sell/gateway/internal/auth"

	"github.com/jackc/pgx/v5"
)

const (
	telegramLinkTTL       = 10 * time.Minute
	telegramMembershipTTL = 15 * time.Minute
	telegramFeatureDaily  = "daily"
	telegramFeatureRedeem = "redeem"
	telegramFeatureAPI    = "api_free"
)

var telegramBotAPIBase = "https://api.telegram.org"

var errTelegramMembershipRequired = errors.New("telegram_membership_required")

type telegramGateState string

const (
	telegramUnlocked            telegramGateState = "unlocked"
	telegramLockedNeverVerified telegramGateState = "locked_never_verified"
	telegramLockedLeftGroup     telegramGateState = "locked_left_group"
	telegramLockedTechnical     telegramGateState = "locked_technical"
	telegramLockedTelegramTaken telegramGateState = "locked_tg_taken"
)

type telegramAPIUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type telegramChatMemberResult struct {
	Status string          `json:"status"`
	User   telegramAPIUser `json:"user"`
}

type telegramAPIResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
}

func telegramBotToken() string      { return strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")) }
func telegramChatID() string        { return strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID")) }
func telegramWebhookSecret() string { return strings.TrimSpace(os.Getenv("TELEGRAM_WEBHOOK_SECRET")) }
func telegramConfigured() bool {
	return telegramBotToken() != "" && telegramChatID() != "" && telegramWebhookSecret() != ""
}

func telegramActiveStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "creator", "administrator", "member":
		return true
	default:
		return false
	}
}

func telegramHash(raw string) string {
	s := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(s[:])
}

func newTelegramLinkToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func telegramAPI(ctx context.Context, method string, q url.Values, out any) error {
	token := telegramBotToken()
	if token == "" {
		return errors.New("telegram bot belum dikonfigurasi")
	}
	u := telegramBotAPIBase + "/bot" + token + "/" + method
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	res, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var body telegramAPIResponse
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&body); err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK || !body.OK {
		return fmt.Errorf("telegram %s gagal: %s", method, body.Description)
	}
	if out != nil && len(body.Result) > 0 {
		if err := json.Unmarshal(body.Result, out); err != nil {
			return err
		}
	}
	return nil
}

func telegramMember(ctx context.Context, telegramUserID int64) (telegramChatMemberResult, error) {
	var member telegramChatMemberResult
	err := telegramAPI(ctx, "getChatMember", url.Values{
		"chat_id": {telegramChatID()},
		"user_id": {strconv.FormatInt(telegramUserID, 10)},
	}, &member)
	return member, err
}

func telegramGateWriteError(w http.ResponseWriter, state telegramGateState, status int) {
	message := "Kamu belum verifikasi keanggotaan grup Telegram."
	code := "TELEGRAM_MEMBERSHIP_REQUIRED"
	switch state {
	case telegramLockedLeftGroup:
		message = "Kamu keluar dari grup Telegram. Join ulang & verifikasi untuk membuka lagi."
	case telegramLockedTelegramTaken:
		code = "TELEGRAM_ACCOUNT_TAKEN"
		message = "Akun Telegram ini sudah pernah dipakai verifikasi di akun lain."
	case telegramLockedTechnical:
		code = "TELEGRAM_GATE_UNAVAILABLE"
		message = "Verifikasi gagal, coba lagi dalam beberapa menit."
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{
		"code": code, "state": strings.ToUpper(string(state)), "message": message,
	}})
}

func telegramVerificationState(ctx context.Context, db DB, userID string, force bool) (telegramGateState, string) {
	var telegramID int64
	var status string
	var checked *time.Time
	var username string
	err := db.QueryRow(ctx, `SELECT telegram_user_id,status,last_checked_at,COALESCE(telegram_username,'')
		FROM telegram_verifications WHERE user_id=$1`, userID).Scan(&telegramID, &status, &checked, &username)
	if errors.Is(err, pgx.ErrNoRows) {
		var result string
		_ = db.QueryRow(ctx, `SELECT COALESCE(result_code,'') FROM telegram_link_token WHERE user_id=$1 AND purpose='verify' ORDER BY created_at DESC LIMIT 1`, userID).Scan(&result)
		if result == string(telegramLockedTelegramTaken) {
			return telegramLockedTelegramTaken, ""
		}
		return telegramLockedNeverVerified, ""
	}
	if err != nil {
		return telegramLockedTechnical, ""
	}
	if !force && status == "active" && checked != nil && checked.After(time.Now().Add(-telegramMembershipTTL)) {
		return telegramUnlocked, username
	}
	if !telegramConfigured() {
		if status == "revoked" {
			return telegramLockedLeftGroup, username
		}
		return telegramLockedTechnical, username
	}
	member, err := telegramMember(ctx, telegramID)
	if err != nil {
		return telegramLockedTechnical, username
	}
	if !telegramActiveStatus(member.Status) {
		_, _ = db.Exec(ctx, `UPDATE telegram_verifications SET status='revoked', revoked_at=now(), last_checked_at=now() WHERE user_id=$1`, userID)
		return telegramLockedLeftGroup, username
	}
	if member.User.Username != "" {
		username = member.User.Username
	}
	_, _ = db.Exec(ctx, `UPDATE telegram_verifications SET status='active', telegram_username=NULLIF($2,''), verified_at=now(), revoked_at=NULL, last_checked_at=now() WHERE user_id=$1`, userID, username)
	return telegramUnlocked, username
}

func telegramFeatureRequired(ctx context.Context, db DB, userID string, feature string) (telegramGateState, error) {
	state, _ := telegramVerificationState(ctx, db, userID, false)
	if state != telegramUnlocked {
		return state, errTelegramMembershipRequired
	}
	return telegramUnlocked, nil
}

// TelegramLink creates an opaque, one-time Telegram /start link. The raw token
// is never persisted; a database hash is enough to consume it atomically.
func TelegramLink(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		token, err := newTelegramLinkToken()
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal membuat tautan Telegram")
			return
		}
		if _, err := db.Exec(r.Context(), `UPDATE telegram_link_token SET consumed_at=now() WHERE user_id=$1 AND purpose='verify' AND consumed_at IS NULL`, uid); err != nil {
			fail(w, http.StatusInternalServerError, "gagal memperbarui tautan Telegram")
			return
		}
		if _, err := db.Exec(r.Context(), `INSERT INTO telegram_link_token(user_id,purpose,token_hash,expires_at) VALUES($1,'verify',$2,now()+interval '10 minutes')`, uid, telegramHash(token)); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menyimpan tautan Telegram")
			return
		}
		ok(w, map[string]any{"url": "https://t.me/node2mintbot?start=" + token, "expires_in_seconds": int(telegramLinkTTL.Seconds())})
	}
}

// TelegramVerify rechecks the Telegram account previously bound through /start.
func TelegramVerify(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, username := telegramVerificationState(r.Context(), db, uidFromCtx(r.Context()), true)
		if state != telegramUnlocked {
			telegramGateWriteError(w, state, http.StatusForbidden)
			return
		}
		ok(w, map[string]any{"verified": true, "telegram_username": username})
	}
}

func TelegramStatus(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, username := telegramVerificationState(r.Context(), db, uidFromCtx(r.Context()), false)
		out := map[string]any{"verified": state == telegramUnlocked, "state": state, "locked_features": []string{}}
		if username != "" {
			out["telegram_username"] = username
		}
		if state != telegramUnlocked {
			out["locked_features"] = []string{telegramFeatureAPI, telegramFeatureRedeem, telegramFeatureDaily, "referral_bonus"}
		}
		ok(w, out)
	}
}

func TelegramGateCheck(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		feature := strings.TrimSpace(r.URL.Query().Get("feature"))
		switch feature {
		case telegramFeatureAPI, telegramFeatureRedeem, telegramFeatureDaily, "referral_bonus":
		default:
			fail(w, http.StatusBadRequest, "fitur gate tidak dikenal")
			return
		}
		state, _ := telegramVerificationState(r.Context(), db, uidFromCtx(r.Context()), false)
		ok(w, map[string]any{"allowed": state == telegramUnlocked, "reason": state})
	}
}

func RequireTelegramFeatureHTTP(w http.ResponseWriter, r *http.Request, db DB, feature string) bool {
	state, err := telegramFeatureRequired(r.Context(), db, uidFromCtx(r.Context()), feature)
	if err == nil {
		return true
	}
	status := http.StatusForbidden
	if state == telegramLockedTechnical {
		status = http.StatusServiceUnavailable
	}
	telegramGateWriteError(w, state, status)
	return false
}

func TelegramWebhook(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secret := telegramWebhookSecret()
		if secret == "" || !hmac.Equal([]byte(r.Header.Get("X-Telegram-Bot-Api-Secret-Token")), []byte(secret)) {
			browserError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			fail(w, http.StatusBadRequest, "bad body")
			return
		}
		var update struct {
			Message *struct {
				Text string `json:"text"`
				Chat struct {
					ID   int64  `json:"id"`
					Type string `json:"type"`
				} `json:"chat"`
				From telegramAPIUser `json:"from"`
			} `json:"message"`
			ChatMember *struct {
				Chat struct {
					ID       int64  `json:"id"`
					Username string `json:"username"`
				} `json:"chat"`
				NewChatMember telegramChatMemberResult `json:"new_chat_member"`
			} `json:"chat_member"`
		}
		if err := json.Unmarshal(body, &update); err != nil {
			fail(w, http.StatusBadRequest, "bad update")
			return
		}
		if update.Message != nil && update.Message.Chat.Type == "private" {
			parts := strings.Fields(update.Message.Text)
			if len(parts) == 2 && strings.HasPrefix(parts[0], "/start") {
				handleTelegramStart(r.Context(), db, parts[1], update.Message.From, update.Message.Chat.ID)
			}
		}
		if update.ChatMember != nil && telegramWebhookChatMatches(update.ChatMember.Chat.ID, update.ChatMember.Chat.Username) {
			status := strings.ToLower(update.ChatMember.NewChatMember.Status)
			if status == "left" || status == "kicked" {
				_, _ = db.Exec(r.Context(), `UPDATE telegram_verifications SET status='revoked', revoked_at=now(), last_checked_at=now() WHERE telegram_user_id=$1 AND status='active'`, update.ChatMember.NewChatMember.User.ID)
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}

func telegramWebhookChatMatches(chatID int64, username string) bool {
	configured := strings.TrimSpace(telegramChatID())
	if configured == "" {
		return false
	}
	if n, err := strconv.ParseInt(configured, 10, 64); err == nil {
		return n == chatID
	}
	return strings.EqualFold(strings.TrimPrefix(configured, "@"), strings.TrimPrefix(username, "@"))
}

func handleTelegramStart(ctx context.Context, db DB, raw string, tg telegramAPIUser, chatID int64) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	var linkID, userID, purpose string
	err = tx.QueryRow(ctx, `UPDATE telegram_link_token SET consumed_at=now() WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at>now() RETURNING id,user_id,purpose`, telegramHash(raw)).Scan(&linkID, &userID, &purpose)
	if err != nil || tg.ID == 0 {
		return
	}
	setResult := func(code string) {
		_, _ = tx.Exec(ctx, `UPDATE telegram_link_token SET result_code=$1 WHERE id=$2`, code, linkID)
	}
	if purpose == "seller_connect" {
		if chatID == 0 || chatID != tg.ID {
			setResult("invalid_private_chat")
			_ = tx.Commit(ctx)
			return
		}
		var owner string
		err = tx.QueryRow(ctx, `SELECT seller_id::text FROM seller_telegram WHERE telegram_chat_id=$1 FOR UPDATE`, chatID).Scan(&owner)
		if err == nil && owner != userID {
			setResult(string(telegramLockedTelegramTaken))
			_ = tx.Commit(ctx)
			return
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return
		}
		if _, err := tx.Exec(ctx, `INSERT INTO seller_telegram(seller_id,telegram_chat_id,telegram_username)
			VALUES($1,$2,NULLIF($3,''))
			ON CONFLICT (seller_id) DO UPDATE SET telegram_chat_id=EXCLUDED.telegram_chat_id,telegram_username=EXCLUDED.telegram_username,notifications_enabled=true,updated_at=now()`, userID, chatID, tg.Username); err != nil {
			return
		}
		setResult("active")
		if err := tx.Commit(ctx); err == nil {
			_ = sendTelegramSellerMessage(ctx, chatID, "✅ <b>Telegram seller terhubung.</b> Notifikasi pesanan, dispute, dan stok akan dikirim ke chat ini.", "Buka Toko", sellerPublicURL("/toko"))
		}
		return
	}
	if purpose != "verify" {
		return
	}
	var owner string
	err = tx.QueryRow(ctx, `SELECT user_id FROM telegram_verifications WHERE telegram_user_id=$1 FOR UPDATE`, tg.ID).Scan(&owner)
	if err == nil && owner != userID {
		setResult(string(telegramLockedTelegramTaken))
		_ = tx.Commit(ctx)
		return
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return
	}
	var bound int64
	err = tx.QueryRow(ctx, `SELECT telegram_user_id FROM telegram_verifications WHERE user_id=$1 FOR UPDATE`, userID).Scan(&bound)
	if err == nil && bound != tg.ID {
		setResult(string(telegramLockedTelegramTaken))
		_ = tx.Commit(ctx)
		return
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return
	}
	member, err := telegramMember(ctx, tg.ID)
	if err != nil || !telegramActiveStatus(member.Status) {
		return
	}
	if member.User.Username != "" {
		tg.Username = member.User.Username
	}
	_, err = tx.Exec(ctx, `INSERT INTO telegram_verifications(user_id,telegram_user_id,telegram_username,status,verified_at,last_checked_at)
		VALUES($1,$2,NULLIF($3,''),'active',now(),now())
		ON CONFLICT (user_id) DO UPDATE SET status='active',telegram_username=EXCLUDED.telegram_username,verified_at=now(),revoked_at=NULL,last_checked_at=now()
		WHERE telegram_verifications.telegram_user_id=EXCLUDED.telegram_user_id`, userID, tg.ID, tg.Username)
	if err != nil {
		return
	}
	setResult("active")
	_ = tx.Commit(ctx)
}

func TelegramFreeTokensNeeded(ctx context.Context, db DB, userID string, units int64) (bool, error) {
	if units <= 0 {
		return false, nil
	}
	var paid, free int64
	if err := db.QueryRow(ctx, `SELECT
		COALESCE(SUM(remaining) FILTER (WHERE NOT requires_telegram),0),
		COALESCE(SUM(remaining) FILTER (WHERE requires_telegram),0)
		FROM user_token_package WHERE user_id=$1 AND expires_at>now() AND remaining>0`, userID).Scan(&paid, &free); err != nil {
		return false, err
	}
	return paid < units && free > 0, nil
}

func RequireTelegramForFreeTokens(ctx context.Context, db DB, userID string, units int64) (telegramGateState, error) {
	need, err := TelegramFreeTokensNeeded(ctx, db, userID, units)
	if err != nil || !need {
		return telegramUnlocked, err
	}
	return telegramFeatureRequired(ctx, db, userID, telegramFeatureAPI)
}

func StartTelegramMembershipRecheck(db DB, every time.Duration) {
	if !telegramConfigured() {
		return
	}
	go func() {
		for {
			rows, err := db.Query(context.Background(), `SELECT user_id FROM telegram_verifications WHERE status='active' ORDER BY last_checked_at NULLS FIRST LIMIT 500`)
			if err == nil {
				for rows.Next() {
					var uid string
					if rows.Scan(&uid) == nil {
						_, _ = telegramVerificationState(context.Background(), db, uid, true)
					}
				}
				rows.Close()
			}
			time.Sleep(every)
		}
	}()
}

func telegramV1Gate(w http.ResponseWriter, state telegramGateState) {
	auth.WriteV1Error(w, http.StatusForbidden, "telegram membership required", "TELEGRAM_MEMBERSHIP_REQUIRED")
}
