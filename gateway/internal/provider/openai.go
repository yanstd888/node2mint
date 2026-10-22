// Provider: OpenAI-compatible chat (dipakai GPT, DeepSeek, dll).
// Kontrak: implementasikan provider.Provider.
package provider

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIProvider: implementasi Provider untuk OpenAI-compatible API.
type OpenAIProvider struct {
	cfg Config
	hc  *http.Client
}

func NewOpenAI(cfg Config) *OpenAIProvider {
	return &OpenAIProvider{
		cfg: cfg,
		hc:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Chat memenuhi interface Provider.
func (p *OpenAIProvider) Chat(req RequestChat) (UsageUsage, error) {
	// Ponytail: stream flag support sederhana — jika true, pakai SSE ke downstream.
	// Untuk POC, pakai request non-stream (response penuh), usage di-parse dari akhir.
	url := p.cfg.BaseURL + "/chat/completions"
	body := map[string]any{
		"model":       req.Model,
		"messages":    req.Messages,
		"max_tokens":  req.MaxToken,
		"temperature": req.Temp,
	}
	if req.Stream {
		body["stream"] = true
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return UsageUsage{}, fmt.Errorf("marshal failed: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return UsageUsage{}, fmt.Errorf("create request failed: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.hc.Do(httpReq)
	if err != nil {
		return UsageUsage{}, fmt.Errorf("provider request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return UsageUsage{}, fmt.Errorf("provider %d: %s", resp.StatusCode, string(b))
	}

	if req.Stream {
		// Ponytail: stream passthrough ke caller — caller (gateway handler) yang
		// translate ke SSE. Usage tidak tersedia di mode stream OpenAI, estimasi.
		return UsageUsage{}, fmt.Errorf("stream usage tidak tersedia di v1")
	}

	var result struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			PromptTokens         int `json:"prompt_tokens"`
			CompletionTokens     int `json:"completion_tokens"`
			TotalTokens          int `json:"total_tokens"`
			CacheTokens          int `json:"cached_tokens"`
			CacheReadInputTokens int `json:"cache_read_input_tokens"`
			// Cache dihitung penuh sebagai token biasa; angkanya hanya untuk
			// catatan/transparansi di usage_log.cache_tokens.
			PromptTokensDetails struct {
				CacheTokens int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
			InputTokensDetails struct {
				CacheTokens int `json:"cached_tokens"`
			} `json:"input_tokens_details"`
		} `json:"usage"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return UsageUsage{}, fmt.Errorf("decode failed: %w", err)
	}
	// Cost: dihapus — harga per model kini di ai_model.token_multiplier,
	// dihitung di handler (settleChat), bukan di provider.
	usage := UsageUsage{}
	if result.Usage != nil {
		usage.PromptTokens = result.Usage.PromptTokens
		usage.CompletionTokens = result.Usage.CompletionTokens
		usage.TotalTokens = result.Usage.TotalTokens
		usage.CacheTokens = max(result.Usage.CacheTokens, result.Usage.CacheReadInputTokens,
			result.Usage.PromptTokensDetails.CacheTokens, result.Usage.InputTokensDetails.CacheTokens)
	}
	if len(result.Choices) > 0 {
		usage.Content = result.Choices[0].Message.Content
	}
	return usage, nil
}

// StreamChat: SSE upstream OpenAI-compatible. Tiap delta content dipanggil ke
// onDelta; usage diambil dari blok "usage" kalau upstream mengirimnya, kalau
// tidak diestimasi dari panjang teks (≈4 char/token).
// ponytail: tanpa retry/keepalive — failover ada di Registry.StreamChat.
func (p *OpenAIProvider) StreamChat(req RequestChat, onDelta func(string)) (UsageUsage, error) {
	url := p.cfg.BaseURL + "/chat/completions"
	body := map[string]any{
		"model":       req.Model,
		"messages":    req.Messages,
		"max_tokens":  req.MaxToken,
		"temperature": req.Temp,
		"stream":      true,
		// OpenAI-compatible upstream biasanya hanya mengirim blok usage pada
		// chunk terakhir bila ini diminta. Tanpanya prompt/cache selalu nol dan
		// gateway terpaksa mengestimasi input, sehingga cache tidak pernah tercatat.
		"stream_options": map[string]any{"include_usage": true},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return UsageUsage{}, fmt.Errorf("marshal failed: %w", err)
	}
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return UsageUsage{}, fmt.Errorf("create request failed: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.hc.Do(httpReq)
	if err != nil {
		return UsageUsage{}, fmt.Errorf("provider request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return UsageUsage{}, fmt.Errorf("provider %d: %s", resp.StatusCode, string(b))
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return UsageUsage{}, fmt.Errorf("provider stream bukan SSE (%s): %s", resp.Header.Get("Content-Type"), string(b))
	}

	var usage UsageUsage
	var full strings.Builder
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var ev struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens         int `json:"prompt_tokens"`
				CompletionTokens     int `json:"completion_tokens"`
				TotalTokens          int `json:"total_tokens"`
				CacheTokens          int `json:"cached_tokens"`
				CacheReadInputTokens int `json:"cache_read_input_tokens"`
				PromptTokensDetails  struct {
					CacheTokens int `json:"cached_tokens"`
				} `json:"prompt_tokens_details"`
				InputTokensDetails struct {
					CacheTokens int `json:"cached_tokens"`
				} `json:"input_tokens_details"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}
		if ev.Usage != nil {
			usage.PromptTokens = ev.Usage.PromptTokens
			usage.CompletionTokens = ev.Usage.CompletionTokens
			usage.TotalTokens = ev.Usage.TotalTokens
			usage.CacheTokens = max(ev.Usage.CacheTokens, ev.Usage.CacheReadInputTokens,
				ev.Usage.PromptTokensDetails.CacheTokens, ev.Usage.InputTokensDetails.CacheTokens)
		}
		for _, c := range ev.Choices {
			if c.Delta.Content == "" {
				continue
			}
			full.WriteString(c.Delta.Content)
			if onDelta != nil {
				onDelta(c.Delta.Content)
			}
		}
	}
	usage.Content = full.String()
	if usage.TotalTokens == 0 && usage.Content != "" {
		usage.CompletionTokens = full.Len()/4 + 1
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return usage, nil
}

// estimateCost dihapus: harga per model = ai_model.token_multiplier (di DB,
// dihitung di handler/settleChat). Harga hardcoded di sini bohong setelah
// tagihan jadi token murni.
