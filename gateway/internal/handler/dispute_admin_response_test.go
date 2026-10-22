package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"ai-sell/gateway/internal/store"
)

// TestDisputeListIncludesSellerResponseAndTimestamp covers the admin response
// contract with a disposable DB only; it never creates a production dispute.
func TestDisputeListIncludesSellerResponseAndTimestamp(t *testing.T) {
	if os.Getenv("DISPUTE_ADMIN_RESPONSE_TEST") != "1" || os.Getenv("DATABASE_URL") == "" {
		t.Skip("DISPUTE_ADMIN_RESPONSE_TEST=1 dan DATABASE_URL disposable diperlukan")
	}
	ctx := context.Background()
	db, err := store.Connect(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, _, orderID := seedEscrowFixture(t, ctx, db, false, 0)
	id := disputeID(t, ctx, db, orderID)
	response := "Barang sudah dikirim beserta bukti pengiriman."
	respondedAt := time.Date(2026, 10, 11, 8, 30, 0, 0, time.UTC)
	if _, err := db.Exec(ctx,
		`UPDATE market_dispute SET seller_response=$1, seller_response_at=$2 WHERE id=$3`,
		response, respondedAt, id); err != nil {
		t.Fatalf("seed seller response: %v", err)
	}

	rec := httptest.NewRecorder()
	DisputeList(db).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/admin/disputes?status=OPEN", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, mau 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Disputes []struct {
			ID               string `json:"id"`
			SellerResponse   string `json:"seller_response"`
			SellerResponseAt string `json:"seller_response_at"`
		} `json:"disputes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rec.Body.String())
	}
	for _, got := range body.Disputes {
		if got.ID != id {
			continue
		}
		if got.SellerResponse != response {
			t.Fatalf("seller response=%q, mau %q", got.SellerResponse, response)
		}
		gotRespondedAt, err := time.Parse(time.RFC3339, got.SellerResponseAt)
		if err != nil || !gotRespondedAt.Equal(respondedAt) {
			t.Fatalf("seller response at=%q, mau instant %s (err=%v)", got.SellerResponseAt, respondedAt.Format(time.RFC3339), err)
		}
		return
	}
	t.Fatalf("dispute %s tidak ditemukan dalam response admin", id)
}
