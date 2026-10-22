package handler

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func sendTelegramAdmin(ctx context.Context, text string) {
	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	chatID := strings.TrimSpace(os.Getenv("TELEGRAM_ADMIN_CHAT_ID"))
	if chatID == "" {
		chatID = telegramChatID() // fallback ke TELEGRAM_CHAT_ID (dev/test)
	}
	if token == "" || chatID == "" || text == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	form := url.Values{"chat_id": {chatID}, "text": {text}, "disable_web_page_preview": {"true"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+token+"/sendMessage", strings.NewReader(form.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}
