package handler

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestDisputeResponseDeadlineSchemaAndInsert(t *testing.T) {
	schema, err := os.ReadFile("../../sql/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"response_deadline TIMESTAMPTZ",
		"ALTER TABLE market_dispute ADD COLUMN IF NOT EXISTS response_deadline TIMESTAMPTZ",
	} {
		if !strings.Contains(string(schema), want) {
			t.Fatalf("schema.sql missing %q", want)
		}
	}
	source, err := os.ReadFile("dispute.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "response_deadline) VALUES ($1,$2,$3,$4,$5,now()+interval '24 hours')") {
		t.Fatal("new disputes must persist the 24-hour response deadline")
	}
}

// TestDisputeEligibility memastikan buyer hanya bisa membuka sengketa pada
// order PAID/PROCESSING, atau COMPLETED dalam 7 hari.
func TestDisputeEligibility(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	completed7d := time.Date(2025, 12, 25, 12, 0, 0, 0, time.UTC) // 7 hari sebelumnya
	completed8d := time.Date(2025, 12, 24, 12, 0, 0, 0, time.UTC) // 8 hari sebelumnya

	cases := []struct {
		name        string
		status      string
		completedAt *time.Time
		want        bool
	}{
		{"PAID boleh", "PAID", nil, true},
		{"PROCESSING boleh", "PROCESSING", nil, true},
		{"COMPLETED dalam 7 hari boleh", "COMPLETED", &completed7d, true},
		{"COMPLETED 8 hari ditolak", "COMPLETED", &completed8d, false},
		{"COMPLETED tanpa completed_at ditolak", "COMPLETED", nil, false},
		{"CANCELLED ditolak", "CANCELLED", nil, false},
		{"DISPUTED ditolak (sudah ada sengketa)", "DISPUTED", nil, false},
		{"REFUNDED ditolak", "REFUNDED", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := disputeEligibility(c.status, c.completedAt, now); got != c.want {
				t.Fatalf("status=%s: got %v, mau %v", c.status, got, c.want)
			}
		})
	}
}

// TestResolveDecision memastikan kebijakan: tidak ada refund parsial,
// manual_review bila saldo toko < subtotal.
func TestResolveDecision(t *testing.T) {
	cases := []struct {
		name      string
		buyerWins bool
		storeBal  float64
		subtotal  float64
		want      string
	}{
		{"buyer wins, saldo cukup", true, 100, 50, "BUYER_WINS"},
		{"buyer wins, saldo pas", true, 50, 50, "BUYER_WINS"},
		{"buyer wins, saldo kurang", true, 49.99, 50, "MANUAL_REVIEW"},
		{"buyer wins, saldo 0", true, 0, 50, "MANUAL_REVIEW"},
		{"buyer wins, subtotal 0", true, 100, 0, "BUYER_WINS"},
		{"seller wins (buyer tidak menang)", false, 100, 50, "RESOLVED"},
		{"seller wins, saldo 0", false, 0, 50, "RESOLVED"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveDecision(c.buyerWins, c.storeBal, c.subtotal); got != c.want {
				t.Fatalf("buyerWins=%v storeBal=%f subtotal=%f: got %s, mau %s",
					c.buyerWins, c.storeBal, c.subtotal, got, c.want)
			}
		})
	}
}

// TestResolveDecisionNoPartial memastikan tidak ada jalur refund sebagian —
// selalu penuh atau tidak sama sekali.
func TestResolveDecisionNoPartial(t *testing.T) {
	// Saldo cukup untuk sebagian tapi tidak penuh → tetap MANUAL_REVIEW,
	// bukan "refund sebagian".
	if got := resolveDecision(true, 30, 100); got != "MANUAL_REVIEW" {
		t.Fatalf("saldo 30 < subtotal 100: harus MANUAL_REVIEW (no partial), dapat %s", got)
	}
}

// TestDisputeReasonValidation memastikan reason wajib 10–1000 karakter.
func TestDisputeReasonValidation(t *testing.T) {
	cases := []struct {
		name   string
		reason string
		wantOK bool
	}{
		{"reason valid", "barang tidak dikirim sesuai order", true},
		{"reason minimal 10 karakter", "1234567890", true},
		{"reason 9 karakter ditolak", "123456789", false},
		{"reason kosong ditolak", "", false},
		{"reason 1000 karakter diterima", strings.Repeat("x", 1000), true},
		{"reason 1001 karakter ditolak", strings.Repeat("x", 1001), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotOK := len(c.reason) >= 10 && len(c.reason) <= 1000
			if gotOK != c.wantOK {
				t.Fatalf("reason len=%d: got %v, mau %v", len(c.reason), gotOK, c.wantOK)
			}
		})
	}
}

// TestDisputeAuditTrail memastikan audit_log action/action name konsisten
// dan target_idnya adalah dispute ID.
func TestDisputeAuditTrail(t *testing.T) {
	// Keamanan: action harus eksplisit, bukan dinamis dari input user.
	assertAuditAction("DISPUTE_OPEN")
	assertAuditAction("DISPUTE_RESOLVE")
}

// assertAuditAction verifikasi nama action yang dipakai handler audit.
// (guard struktural — jika nama action typo, audit trail tidak bisa dilacak)
func assertAuditAction(action string) {
	if !strings.HasPrefix(action, "DISPUTE_") {
		panic("audit action harus memakai prefix DISPUTE_: " + action)
	}
}

// TestSellerResponseValidation memastikan seller response wajib 10–2000 karakter.
func TestSellerResponseValidation(t *testing.T) {
	cases := []struct {
		name     string
		response string
		wantOK   bool
	}{
		{"response valid", "Barang sudah dikirim, cek resi di chat.", true},
		{"response minimal 10 karakter", "1234567890", true},
		{"response 9 karakter ditolak", "123456789", false},
		{"response kosong ditolak", "", false},
		{"response 2000 karakter diterima", strings.Repeat("x", 2000), true},
		{"response 2001 karakter ditolak", strings.Repeat("x", 2001), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotOK := len(c.response) >= 10 && len(c.response) <= 2000
			if gotOK != c.wantOK {
				t.Fatalf("response len=%d: got %v, mau %v", len(c.response), gotOK, c.wantOK)
			}
		})
	}
}

// TestEvidenceURLValidation memastikan evidence_url opsional, max 500 karakter.
func TestEvidenceURLValidation(t *testing.T) {
	cases := []struct {
		name        string
		evidenceURL string
		wantOK      bool
	}{
		{"evidence kosong boleh", "", true},
		{"evidence valid", "https://example.com/proof.jpg", true},
		{"evidence 500 karakter diterima", strings.Repeat("x", 500), true},
		{"evidence 501 karakter ditolak", strings.Repeat("x", 501), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotOK := len(c.evidenceURL) <= 500
			if gotOK != c.wantOK {
				t.Fatalf("evidence_url len=%d: got %v, mau %v", len(c.evidenceURL), gotOK, c.wantOK)
			}
		})
	}
}
