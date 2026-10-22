// Responses provider: OpenAI Responses API (/responses). Dipakai upstream yang
// TIDAK menyediakan /chat/completions (mis. node apiType=responses di 9router).
// Format: system → `instructions`, sisanya → array `input`; output teks ada di
// output[].content[].text, dasar streaming di event response.output_text.delta.
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

type ResponsesProvider struct {
	cfg Config
	hc  *http.Client
}

func NewResponses(cfg Config) *ResponsesProvider {
	return &ResponsesProvider{
		cfg: cfg,
		hc:  &http.Client{Timeout: 60 * time.Second},
	}
}

// responsesBody menyusun body Responses API dari RequestChat.
func responsesBody(model string, msgs []Message, maxOut int) map[string]any {
	var instructions []string
	items := make([]map[string]string, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == "system" {
			instructions = append(instructions, m.Content)
			continue
		}
		items = append(items, map[string]string{"role": m.Role, "content": m.Content})
	}
	body := map[string]any{"model": model, "input": items, "max_output_tokens": maxOut}
	if len(instructions) > 0 {
		body["instructions"] = strings.Join(instructions, "\n")
	}
	return body
}

type responsesUsage struct {
	InputTokens        int `json:"input_tokens"`
	OutputTokens       int `json:"output_tokens"`
	TotalTokens        int `json:"total_tokens"`
	InputTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
}

func (u responsesUsage) applyTo(dst *UsageUsage) {
	dst.PromptTokens = u.InputTokens
	dst.CompletionTokens = u.OutputTokens
	dst.TotalTokens = u.TotalTokens
	dst.CacheTokens = u.InputTokensDetails.CachedTokens
}

// outputText: gabungkan semua potongan teks dari output[] Responses API.
func outputText(raw json.RawMessage) string {
	var out []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return ""
	}
	var b strings.Builder
	for _, item := range out {
		for _, c := range item.Content {
			if c.Type == "output_text" || c.Text != "" {
				b.WriteString(c.Text)
			}
		}
	}
	return b.String()
}

func (p *ResponsesProvider) post(req RequestChat, maxOut int) (*http.Response, error) {
	payload, err := json.Marshal(responsesBody(req.Model, req.Messages, maxOut))
	if err != nil {
		return nil, fmt.Errorf("marshal failed: %w", err)
	}
	httpReq, err := http.NewRequest("POST", p.cfg.BaseURL+"/responses", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := p.hc.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("provider request failed: %w", err)
	}
	if resp.StatusCode != 200 {
		defer resp.Body.Close()
		// Format galat sama dengan provider OpenAI-compatible supaya cooldown
		// registry (cooldownFrom) bisa membaca kodenya.
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("provider %d: %s", resp.StatusCode, string(b))
	}
	return resp, nil
}

// Chat memenuhi interface Provider (non-stream).
func (p *ResponsesProvider) Chat(req RequestChat) (UsageUsage, error) {
	maxOut := req.MaxToken
	if maxOut <= 0 {
		maxOut = 1024
	}
	resp, err := p.post(req, maxOut)
	if err != nil {
		return UsageUsage{}, err
	}
	defer resp.Body.Close()

	var result struct {
		Output json.RawMessage `json:"output"`
		Usage  *responsesUsage `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return UsageUsage{}, fmt.Errorf("decode failed: %w", err)
	}
	var usage UsageUsage
	if result.Usage != nil {
		result.Usage.applyTo(&usage)
	}
	usage.Content = outputText(result.Output)
	return usage, nil
}

// StreamChat: SSE Responses API — teks di event output_text.delta, usage di
// response.completed. Usage yang tidak dikirim diestimasi (≈4 char/token).
func (p *ResponsesProvider) StreamChat(req RequestChat, onDelta func(string)) (UsageUsage, error) {
	maxOut := req.MaxToken
	if maxOut <= 0 {
		maxOut = 1024
	}
	payload, err := json.Marshal(responsesBody(req.Model, req.Messages, maxOut))
	if err != nil {
		return UsageUsage{}, fmt.Errorf("marshal failed: %w", err)
	}
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		return UsageUsage{}, fmt.Errorf("marshal failed: %w", err)
	}
	body["stream"] = true
	payload, _ = json.Marshal(body)

	httpReq, err := http.NewRequest("POST", p.cfg.BaseURL+"/responses", bytes.NewReader(payload))
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
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return UsageUsage{}, fmt.Errorf("provider %d: %s", resp.StatusCode, string(b))
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
			Type     string `json:"type"`
			Delta    string `json:"delta"`
			Response *struct {
				Usage *responsesUsage `json:"usage"`
			} `json:"response"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "response.output_text.delta":
			if ev.Delta == "" {
				continue
			}
			full.WriteString(ev.Delta)
			if onDelta != nil {
				onDelta(ev.Delta)
			}
		case "response.completed", "response.done":
			if ev.Response != nil && ev.Response.Usage != nil {
				ev.Response.Usage.applyTo(&usage)
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
