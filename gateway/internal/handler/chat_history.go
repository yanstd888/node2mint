package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type chatHistoryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatHistoryItem struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Model     string          `json:"model"`
	Messages  json.RawMessage `json:"messages"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func ChatHistoryList(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		rows, err := db.Query(r.Context(), `SELECT id,title,model,messages,created_at,updated_at FROM chat_history WHERE user_id=$1 ORDER BY updated_at DESC LIMIT 50`, uid)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		out := []chatHistoryItem{}
		for rows.Next() {
			var x chatHistoryItem
			var messages []byte
			if err := rows.Scan(&x.ID, &x.Title, &x.Model, &messages, &x.CreatedAt, &x.UpdatedAt); err != nil {
				fail(w, http.StatusInternalServerError, "query failed")
				return
			}
			x.Messages = json.RawMessage(messages)
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		ok(w, map[string]any{"histories": out})
	}
}

func ChatHistoryCreate(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var in struct {
			Title, Model string
			Messages     []chatHistoryMessage
		}
		if err := decode(r, &in); err != nil || len(in.Messages) == 0 || len(in.Messages) > 100 {
			fail(w, http.StatusBadRequest, "invalid history")
			return
		}
		for _, m := range in.Messages {
			if (m.Role != "user" && m.Role != "assistant") || len(m.Content) > 20000 {
				fail(w, http.StatusBadRequest, "invalid history")
				return
			}
		}
		title := strings.TrimSpace(in.Title)
		if title == "" {
			title = strings.TrimSpace(in.Messages[0].Content)
			if len(title) > 80 {
				title = title[:80]
			}
		}
		if len(title) > 120 || len(in.Model) > 200 {
			fail(w, http.StatusBadRequest, "invalid history")
			return
		}
		var id string
		payload, _ := json.Marshal(in.Messages)
		if err := db.QueryRow(r.Context(), `INSERT INTO chat_history(user_id,title,model,messages) VALUES($1,$2,$3,$4::jsonb) RETURNING id`, uid, title, strings.TrimSpace(in.Model), payload).Scan(&id); err != nil {
			fail(w, http.StatusInternalServerError, "save failed")
			return
		}
		ok(w, map[string]any{"id": id})
	}
}
