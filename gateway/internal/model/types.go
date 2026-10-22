//go:build ignore

// Model data — satu file untuk seluruh tabel (pgx tidak butuh ORM).
// Schema SQL: sql/schema.sql
package model

import "time"

// User: role = USER | SELLER | ADMIN | SUPER_ADMIN
type User struct {
	ID            string    `json:"id" db:"id"`
	Email         string    `json:"email" db:"email"`
	Name          *string   `json:"name" db:"name"`
	Role          string    `json:"role" db:"role"`
	WalletBalance float64   `json:"wallet_balance" db:"wallet_balance"`
	CreatedAt     time.Time `json:"-" db:"created_at"`
	UpdatedAt     time.Time `json:"-" db:"updated_at"`
}

// API key: hanya hash yang disimpan (SHA256).
type APIKey struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"-" db:"user_id"`
	Name      string    `json:"name" db:"name"`
	Prefix    string    `json:"prefix" db:"prefix"` // "sk-abcd1234"
	KeyHash   string    `json:"-" db:"key_hash"`    // SHA256
	Scopes    string    `json:"scopes" db:"scopes"` // CSV
	RateLimit int       `json:"rate_limit" db:"rate_limit"`
	IsActive  bool      `json:"is_active" db:"is_active"`
	LastUsed  time.Time `json:"last_used" db:"last_used"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	RevokedAt time.Time `json:"revoked_at" db:"revoked_at"` // NULL = aktif
}

// CreditTransaction: semua perubahan saldo (atomik).
type CreditTransaction struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"-" db:"user_id"`
	Amount      float64   `json:"amount" db:"amount"` // + atau - (usage)
	Type        string    `json:"type" db:"type"`     // TOPUP | USAGE | REFERRAL | etc
	Status      string    `json:"status" db:"status"` // COMPLETED | PENDING | FAILED
	Description string    `json:"description" db:"description"`
	ReferenceID string    `json:"reference_id,omitempty" db:"reference_id"`
	CreatedAt   time.Time `json:"created_at,omitempty" db:"created_at,omitempty"`
}

// UsageLog: setiap request API yang menghabiskan kredit.
type UsageLog struct {
	ID           string    `json:"id" db:"id"`
	UserID       string    `json:"-" db:"user_id"`
	APIKeyID     string    `json:"-" db:"api_key_id"`
	Model        string    `json:"model" db:"model"`
	InputTokens  int       `json:"input_tokens" db:"input_tokens"`
	OutputTokens int       `json:"output_tokens" db:"output_tokens"`
	Cost         float64   `json:"cost" db:"cost"`
	Status       string    `json:"status" db:"status"` // OK | RATE_LIMITED | INSUFFICIENT_CREDIT
	CreatedAt    time.Time `json:"created_at,omitempty" db:"created_at,omitempty"`
}
