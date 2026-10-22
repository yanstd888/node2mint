package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const telegramNotifyMaxAttempts = 3

var errSellerTelegramUnavailable = errors.New("seller telegram tidak terhubung")

type sellerTelegramPayload struct {
	Store            string  `json:"store,omitempty"`
	Title            string  `json:"title,omitempty"`
	Quantity         int     `json:"quantity,omitempty"`
	Amount           float64 `json:"amount,omitempty"`
	Stock            int     `json:"stock,omitempty"`
	Threshold        int     `json:"threshold,omitempty"`
	OrderID          string  `json:"order_id,omitempty"`
	ProductID        string  `json:"product_id,omitempty"`
	DisputeID        string  `json:"dispute_id,omitempty"`
	Reason           string  `json:"reason,omitempty"`
	OccurredAt       string  `json:"occurred_at,omitempty"`
	ResponseDeadline string  `json:"response_deadline,omitempty"`
}

var sellerTelegramWIB = time.FixedZone("WIB", 7*60*60)
var sellerTelegramMonths = [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

func sellerTelegramTime(at time.Time) string {
	at = at.In(sellerTelegramWIB)
	return fmt.Sprintf("%02d %s %d %02d:%02d WIB", at.Day(), sellerTelegramMonths[at.Month()-1], at.Year(), at.Hour(), at.Minute())
}

func sellerPublicURL(path string) string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_WEB_URL")), "/")
	if !strings.HasPrefix(base, "https://") {
		return ""
	}
	return base + path
}

func sellerTelegramMessage(eventType string, p sellerTelegramPayload) (text, buttonLabel, buttonURL string) {
	store := html.EscapeString(p.Store)
	title := html.EscapeString(p.Title)
	orderID := html.EscapeString(p.OrderID)
	productID := html.EscapeString(p.ProductID)
	disputeID := html.EscapeString(p.DisputeID)
	occurredAt := html.EscapeString(strings.TrimSpace(p.OccurredAt))
	if occurredAt == "" {
		occurredAt = "-"
	}
	responseDeadline := html.EscapeString(strings.TrimSpace(p.ResponseDeadline))
	if responseDeadline == "" {
		responseDeadline = "-"
	}
	amount := fmt.Sprintf("Rp %.0f", p.Amount)
	switch eventType {
	case "NEW_ORDER":
		text = fmt.Sprintf("🛒 <b>PESANAN BARU — PERLU DIPROSES</b>\nToko: %s\nOrder: <code>%s</code>\nWaktu: %s\nProduk: %s × %d\nTotal: <b>%s</b>\nPembayaran: Saldo wallet — LUNAS\nTindakan: proses pesanan dalam <b>1×24 jam</b>.", store, orderID, occurredAt, title, p.Quantity, amount)
		buttonLabel, buttonURL = "Buka Pesanan", sellerPublicURL("/toko?tab=pesanan")
	case "DISPUTE":
		reason := html.EscapeString(p.Reason)
		text = fmt.Sprintf("⚠️ <b>SENGKETA BARU — PERLU RESPONS</b>\nSengketa: <code>%s</code>\nOrder: <code>%s</code>\nWaktu: %s\nProduk: %s\nNilai: <b>%s</b>\nJenis: Sengketa pembeli\nAlasan: %s\nTenggat respons: <b>%s</b>", disputeID, orderID, occurredAt, title, amount, reason, responseDeadline)
		buttonLabel, buttonURL = "Buka Dispute", sellerPublicURL("/toko/disputes")
	case "OUT_OF_STOCK":
		text = fmt.Sprintf("🚨 <b>STOK HABIS</b>\nProduk: %s\nID Produk: <code>%s</code>\nWaktu: %s\nSisa stok: <b>0</b>\nOrder pemicu: <code>%s</code>\nNilai order: <b>%s</b>", title, productID, occurredAt, orderID, amount)
		buttonLabel, buttonURL = "Kelola Produk", sellerPublicURL("/toko?tab=produk")
	case "LOW_STOCK":
		text = fmt.Sprintf("⚠️ <b>STOK MENIPIS</b>\nProduk: %s\nID Produk: <code>%s</code>\nWaktu: %s\nSisa stok: <b>%d</b> (ambang %d)\nOrder pemicu: <code>%s</code>\nNilai order: <b>%s</b>", title, productID, occurredAt, p.Stock, p.Threshold, orderID, amount)
		buttonLabel, buttonURL = "Kelola Produk", sellerPublicURL("/toko?tab=produk")
	default:
		return "", "", ""
	}
	if buttonURL == "" {
		buttonLabel = ""
	}
	return text, buttonLabel, buttonURL
}

func SellerTelegramLink(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var exists bool
		if err := db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM market_store WHERE user_id=$1)`, uid).Scan(&exists); err != nil {
			fail(w, http.StatusInternalServerError, "gagal membaca toko")
			return
		}
		if !exists {
			fail(w, http.StatusForbidden, "buat toko terlebih dahulu")
			return
		}
		token, err := newTelegramLinkToken()
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal membuat tautan Telegram seller")
			return
		}
		ctx := r.Context()
		if _, err := db.Exec(ctx, `UPDATE telegram_link_token SET consumed_at=now() WHERE user_id=$1 AND purpose='seller_connect' AND consumed_at IS NULL`, uid); err != nil {
			fail(w, http.StatusInternalServerError, "gagal memperbarui tautan Telegram seller")
			return
		}
		if _, err := db.Exec(ctx, `INSERT INTO telegram_link_token(user_id,purpose,token_hash,expires_at) VALUES($1,'seller_connect',$2,now()+interval '10 minutes')`, uid, telegramHash(token)); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menyimpan tautan Telegram seller")
			return
		}
		ok(w, map[string]any{"url": "https://t.me/node2mintbot?start=" + token, "expires_in_seconds": int(telegramLinkTTL.Seconds())})
	}
}

func SellerTelegramStatus(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var chatID int64
		var username string
		var enabled bool
		err := db.QueryRow(r.Context(), `SELECT telegram_chat_id,COALESCE(telegram_username,''),notifications_enabled FROM seller_telegram WHERE seller_id=$1`, uidFromCtx(r.Context())).Scan(&chatID, &username, &enabled)
		if errors.Is(err, pgx.ErrNoRows) {
			ok(w, map[string]any{"connected": false, "notifications_enabled": false})
			return
		}
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal membaca status Telegram seller")
			return
		}
		// A blocked bot is not a usable connection. Returning connected=false
		// makes the UI offer a fresh deep-link instead of showing a dead badge.
		ok(w, map[string]any{"connected": enabled, "notifications_enabled": enabled, "telegram_username": username, "telegram_chat_id": strconv.FormatInt(chatID, 10)})
	}
}

func SellerTelegramUnlink(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := db.Exec(r.Context(), `DELETE FROM seller_telegram WHERE seller_id=$1`, uidFromCtx(r.Context())); err != nil {
			fail(w, http.StatusInternalServerError, "gagal memutus Telegram seller")
			return
		}
		ok(w, map[string]any{"ok": true})
	}
}

func enqueueSellerTelegram(ctx context.Context, db execer, eventKey, eventType, sellerID, orderID, productID string, payload sellerTelegramPayload) error {
	if strings.TrimSpace(payload.OccurredAt) == "" {
		payload.OccurredAt = sellerTelegramTime(time.Now())
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `INSERT INTO telegram_notify_log(event_key,type,seller_id,order_id,product_id,payload)
		VALUES($1,$2,$3,NULLIF($4,'')::uuid,NULLIF($5,'')::uuid,$6::jsonb) ON CONFLICT(event_key) DO NOTHING`, eventKey, eventType, sellerID, orderID, productID, string(data))
	return err
}

func queueDisputeSellerNotification(ctx context.Context, db DB, disputeID, orderID, storeID, reason string, amount float64) {
	var sellerID, title string
	var responseDeadline *time.Time
	if err := db.QueryRow(ctx, `SELECT s.user_id::text,p.title,d.response_deadline
		FROM market_dispute d
		JOIN market_order o ON o.id=d.order_id
		JOIN market_store s ON s.id=o.store_id
		JOIN market_product p ON p.id=o.product_id
		WHERE d.id=$1 AND o.id=$2 AND s.id=$3`, disputeID, orderID, storeID).Scan(&sellerID, &title, &responseDeadline); err != nil {
		log.Printf("[WARN] seller dispute notification lookup gagal: %v", err)
		return
	}
	payload := sellerTelegramPayload{Title: title, Amount: amount, OrderID: orderID, DisputeID: disputeID, Reason: reason}
	if responseDeadline != nil {
		payload.ResponseDeadline = sellerTelegramTime(*responseDeadline)
	}
	if err := enqueueSellerTelegram(ctx, db, "dispute:"+disputeID, "DISPUTE", sellerID, orderID, "", payload); err != nil {
		log.Printf("[WARN] enqueue seller dispute notification gagal: %v", err)
	}
}

type telegramNotifyRow struct {
	ID        string
	EventKey  string
	EventType string
	SellerID  string
	Payload   []byte
	Attempts  int
}

func claimTelegramNotify(ctx context.Context, db DB) (*telegramNotifyRow, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE telegram_notify_log SET status='FAILED',next_attempt_at=now(),updated_at=now() WHERE status='SENDING' AND updated_at < now()-interval '10 minutes'`); err != nil {
		return nil, err
	}
	var row telegramNotifyRow
	err = tx.QueryRow(ctx, `SELECT id,event_key,type,seller_id,payload,attempts FROM telegram_notify_log
		WHERE status IN ('PENDING','FAILED') AND next_attempt_at<=now() AND attempts<$1
		ORDER BY created_at LIMIT 1 FOR UPDATE SKIP LOCKED`, telegramNotifyMaxAttempts).
		Scan(&row.ID, &row.EventKey, &row.EventType, &row.SellerID, &row.Payload, &row.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE telegram_notify_log SET status='SENDING',attempts=attempts+1,updated_at=now() WHERE id=$1`, row.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	row.Attempts++
	return &row, nil
}

func sellerTelegramChat(ctx context.Context, db DB, sellerID string) (int64, error) {
	var chatID int64
	var enabled bool
	err := db.QueryRow(ctx, `SELECT telegram_chat_id,notifications_enabled FROM seller_telegram WHERE seller_id=$1`, sellerID).Scan(&chatID, &enabled)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !enabled) {
		return 0, errSellerTelegramUnavailable
	}
	return chatID, err
}

func sendTelegramSellerMessage(ctx context.Context, chatID int64, text, buttonLabel, buttonURL string) error {
	token := telegramBotToken()
	if token == "" {
		return errSellerTelegramUnavailable
	}
	form := url.Values{"chat_id": {strconv.FormatInt(chatID, 10)}, "text": {text}, "parse_mode": {"HTML"}, "disable_web_page_preview": {"true"}}
	if buttonURL != "" {
		if buttonLabel == "" {
			buttonLabel = "Buka Node2Mint"
		}
		markup, _ := json.Marshal(map[string]any{"inline_keyboard": [][]map[string]string{{{"text": buttonLabel, "url": buttonURL}}}})
		form.Set("reply_markup", string(markup))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, telegramBotAPIBase+"/bot"+token+"/sendMessage", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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
		return fmt.Errorf("telegram sendMessage http %d: %s", res.StatusCode, body.Description)
	}
	return nil
}

func telegramNotifyFallback(ctx context.Context, db DB, row telegramNotifyRow, p sellerTelegramPayload) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	message := "Notifikasi Telegram seller gagal dikirim."
	if row.EventType == "NEW_ORDER" && p.OrderID != "" {
		message = "Notifikasi pesanan baru gagal dikirim ke Telegram; buka halaman pesanan untuk memeriksa."
	} else if row.EventType == "DISPUTE" {
		message = "Notifikasi dispute gagal dikirim ke Telegram; periksa tab pesanan."
	} else if row.EventType == "OUT_OF_STOCK" || row.EventType == "LOW_STOCK" {
		message = "Notifikasi stok gagal dikirim ke Telegram; periksa produk toko."
	}
	if _, err := tx.Exec(ctx, `INSERT INTO notifications(user_id,type,title,message,order_id,dispatch_id)
		VALUES($1,'SYSTEM','Telegram seller tidak tersedia',$2,NULLIF($3,'')::uuid,$4)
		ON CONFLICT (dispatch_id,user_id) WHERE dispatch_id IS NOT NULL DO NOTHING`, row.SellerID, message, p.OrderID, "telegram-fallback:"+row.EventKey); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func markTelegramNotifySkipped(ctx context.Context, db DB, row telegramNotifyRow, p sellerTelegramPayload, sendErr error) error {
	message := "seller Telegram tidak terhubung"
	if sendErr != nil && strings.TrimSpace(sendErr.Error()) != "" {
		message = sendErr.Error()
	}
	if len(message) > 500 {
		message = message[:500]
	}
	if _, err := db.Exec(ctx, `UPDATE telegram_notify_log SET status='FAILED',attempts=$2,next_attempt_at=now()+interval '365 days',last_error=$3,updated_at=now() WHERE id=$1 AND status='SENDING'`, row.ID, telegramNotifyMaxAttempts, message); err != nil {
		return err
	}
	return telegramNotifyFallback(ctx, db, row, p)
}

func disableSellerTelegram(ctx context.Context, db DB, sellerID string) error {
	_, err := db.Exec(ctx, `UPDATE seller_telegram SET notifications_enabled=false,updated_at=now() WHERE seller_id=$1`, sellerID)
	return err
}

func markTelegramNotifyFailed(ctx context.Context, db DB, row telegramNotifyRow, p sellerTelegramPayload, permanent bool, sendErr error) error {
	if permanent {
		row.Attempts = telegramNotifyMaxAttempts
	}
	message := "telegram notification failed"
	if sendErr != nil && strings.TrimSpace(sendErr.Error()) != "" {
		message = sendErr.Error()
	}
	if len(message) > 500 {
		message = message[:500]
	}
	var next string
	if row.Attempts >= telegramNotifyMaxAttempts {
		next = "now()+interval '365 days'"
	} else {
		next = fmt.Sprintf("now()+interval '%d minutes'", 1<<min(row.Attempts, 4))
	}
	if _, err := db.Exec(ctx, `UPDATE telegram_notify_log SET status='FAILED',next_attempt_at=`+next+`,last_error=$2,updated_at=now() WHERE id=$1 AND status='SENDING'`, row.ID, message); err != nil {
		return err
	}
	if row.Attempts >= telegramNotifyMaxAttempts {
		if strings.Contains(message, "http 403") {
			if err := disableSellerTelegram(ctx, db, row.SellerID); err != nil {
				log.Printf("[WARN] disable blocked seller Telegram failed: %v", err)
			}
		}
		return telegramNotifyFallback(ctx, db, row, p)
	}
	return nil
}

func dispatchTelegramNotify(ctx context.Context, db DB) bool {
	row, err := claimTelegramNotify(ctx, db)
	if err != nil {
		log.Printf("[WARN] telegram notify claim failed: %v", err)
		return false
	}
	if row == nil {
		return false
	}
	var p sellerTelegramPayload
	if err := json.Unmarshal(row.Payload, &p); err != nil {
		_ = markTelegramNotifyFailed(ctx, db, *row, p, true, err)
		return true
	}
	chatID, err := sellerTelegramChat(ctx, db, row.SellerID)
	if errors.Is(err, errSellerTelegramUnavailable) {
		if markErr := markTelegramNotifySkipped(ctx, db, *row, p, err); markErr != nil {
			log.Printf("[WARN] telegram notify skip update: %v", markErr)
		}
		return true
	}
	permanent := false
	if err == nil {
		var text, buttonLabel, buttonURL string
		text, buttonLabel, buttonURL = sellerTelegramMessage(row.EventType, p)
		err = sendTelegramSellerMessage(ctx, chatID, text, buttonLabel, buttonURL)
		permanent = strings.Contains(telegramErrString(err), "http 403")
	}
	if err != nil {
		if markErr := markTelegramNotifyFailed(ctx, db, *row, p, permanent, err); markErr != nil {
			log.Printf("[WARN] telegram notify failure update: %v", markErr)
		}
		return true
	}
	if _, err := db.Exec(ctx, `UPDATE telegram_notify_log SET status='SENT',sent_at=now(),last_error=NULL,updated_at=now() WHERE id=$1 AND status='SENDING'`, row.ID); err != nil {
		log.Printf("[WARN] telegram notify success update: %v", err)
	}
	return true
}

func telegramErrString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func StartSellerTelegramDispatcher(db DB, interval time.Duration) {
	if telegramBotToken() == "" {
		return
	}
	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			for dispatchTelegramNotify(ctx, db) {
				// ponytail: global serial throttle is conservative across sellers; shard by chat only if backlog proves it necessary.
				time.Sleep(time.Second)
			}
			cancel()
			time.Sleep(interval)
		}
	}()
}
