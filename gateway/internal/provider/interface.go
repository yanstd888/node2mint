// Provider interface: satu kontrak untuk semua AI provider (OpenAI, Anthropic,
// dst). Stream = token demi token; usage = biaya/token usage setelahnya.
// Ponytail: 1 contract only — embedding/rerank/moderation ditambah saat dibutuhkan,
// cukup implementasikan interface yang sama.
package provider

// Message adalah satu pesan dalam request chat.
type Message struct {
	Role    string `json:"role"` // "user" | "assistant" | "system"
	Content string `json:"content"`
}

// RequestChat ke provider.
type RequestChat struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	MaxToken int       `json:"max_tokens,omitempty"`
	Temp     float64   `json:"temperature,omitempty"`
}

// UsageUsage adalah biaya usage yang dikembalikan provider.
type UsageUsage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CacheTokens      int     `json:"cache_tokens"`
	Cost             float64 `json:"cost"`
	// Balasan asli dari provider; tanpa ini chat hanya mengembalikan "...".
	Content string `json:"-"`
}

// Provider adalah kontrak semua provider AI.
type Provider interface {
	// Chat memulai request ke provider. Jika req.Stream, chunks dikirim via callback,
	// UsageUsage dikembalikan di callback terakhir.
	Chat(req RequestChat) (UsageUsage, error)
	// StreamChat: SSE upstream — tiap delta content lewat onDelta, usage
	// dikembalikan setelah stream selesai.
	StreamChat(req RequestChat, onDelta func(string)) (UsageUsage, error)
}

// Config berisi kredensial. Implementasi provider baca dari sini.
type Config struct {
	APIKey  string `json:"-"`
	BaseURL string `json:"base_url"`
}
