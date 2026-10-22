package handler

// Endpoint kompatibel OpenAI + Anthropic di /v1/* (auth API key, scope "chat").
// Port dari new-api: surface yang sama untuk drop-in client, tapi billing-nya
// saldo rupiah (bukan sistem kuota token new-api).

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"ai-sell/gateway/internal/auth"
	"ai-sell/gateway/internal/provider"
)

// ListModelsV1: GET /v1/models — format OpenAI, sumbernya katalog ai_model.
func ListModelsV1(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(r.Context(),
			`SELECT m.name, m.created_at FROM ai_model m WHERE m.is_active ORDER BY `+familyOrderSQL+`, m.sort_weight DESC, m.name`)
		if err != nil {
			failV1(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type model struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
			Name    string `json:"name"` // ekstra non-standar, sama seperti new-api
		}
		data := []model{}
		for rows.Next() {
			var name string
			var created time.Time
			if err := rows.Scan(&name, &created); err != nil {
				continue
			}
			data = append(data, model{ID: name, Object: "model", Created: created.Unix(), OwnedBy: "ai-sell", Name: name})
		}
		ok(w, map[string]any{"object": "list", "data": data})
	}
}

// AnthropicMessages: POST /v1/messages — format Anthropic (system terpisah,
// content blocks). Diterjemahkan ke jalur chat internal yang sama.
// ponytail: tanpa streaming (stream:true ditolak 400) — tambah SSE Anthropic
// (event message_start/content_block_delta) saat klien Anthropic SDK memintanya.
func AnthropicMessages(db DB, reg *provider.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := auth.APIKeyFromContext(r.Context())
		if uid == "" {
			failV1(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		var in struct {
			Model     string          `json:"model"`
			System    json.RawMessage `json:"system"`
			Messages  json.RawMessage `json:"messages"`
			MaxTokens *int            `json:"max_tokens"`
			Stream    bool            `json:"stream"`
		}
		if err := decode(r, &in); err != nil || in.Model == "" || len(in.Messages) == 0 {
			failV1(w, http.StatusBadRequest, "bad json: model dan messages wajib")
			return
		}
		if in.Stream {
			failV1(w, http.StatusBadRequest, "streaming belum didukung di /v1/messages")
			return
		}
		var arr []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(in.Messages, &arr); err != nil {
			failV1(w, http.StatusBadRequest, "bad json: messages")
			return
		}
		var msgs []provider.Message
		if s := anthropicText(in.System); s != "" {
			msgs = append(msgs, provider.Message{Role: "system", Content: s})
		}
		for _, m := range arr {
			role := m.Role
			if role == "" {
				role = "user"
			}
			msgs = append(msgs, provider.Message{Role: role, Content: anthropicText(m.Content)})
		}

		usage, _, err := runChat(r, w, db, reg, uid, auth.APIKeyIDFromContext(r.Context()),
			in.Model, msgs, false, in.MaxTokens)
		if err != nil {
			return // sudah dijawab runChat
		}
		inTok, outTok := usage.PromptTokens, usage.CompletionTokens
		if inTok == 0 {
			inTok = estimateTokens(msgs)
		}
		if outTok == 0 {
			outTok = len(usage.Content)/4 + 1
		}
		ok(w, map[string]any{
			"id":            "msg_" + randHex(12),
			"type":          "message",
			"role":          "assistant",
			"model":         in.Model,
			"content":       []map[string]any{{"type": "text", "text": usage.Content}},
			"stop_reason":   "end_turn",
			"stop_sequence": nil,
			"usage":         map[string]any{"input_tokens": inTok, "output_tokens": outTok},
		})
	}
}

// anthropicText: content Anthropic bisa string atau [{type,text}]. Ambil teksnya
// saja; blok image/tool diabaikan (ponytail: tambah saat ada yang memakainya).
func anthropicText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	var b strings.Builder
	for _, x := range blocks {
		if x.Type == "text" {
			b.WriteString(x.Text)
		}
	}
	return b.String()
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
