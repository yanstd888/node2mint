package handler

import (
	"encoding/json"
	"testing"
	"time"
)

func TestKYCAdminItemJSONContract(t *testing.T) {
	encoded, err := json.Marshal(kycAdminItem{
		ID:        "kyc-1",
		UserID:    "user-1",
		DocType:   "passport",
		Name:      "qa.pdf",
		Mime:      "application/pdf",
		Status:    "pending",
		CreatedAt: time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC),
		Email:     "qa@example.test",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range []string{"id", "user_id", "doc_type", "name", "mime", "status", "reason", "created_at", "email"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("response missing %q: %s", key, encoded)
		}
	}
}
