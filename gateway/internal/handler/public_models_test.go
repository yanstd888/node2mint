package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"ai-sell/gateway/internal/store"
)

// Kontrak katalog publik: harga model yang tidak dipakai settlement tidak
// boleh muncul sebagai tarif. Sebagai gantinya UI menerima aturan token yang
// benar-benar dipakai saat billing.
func TestPublicModelsExposeTokenBillingRules(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL tidak diset — lewati uji kontrak katalog publik")
	}
	db, err := store.Connect(context.Background())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()

	var modelID string
	if err := db.QueryRow(context.Background(), `SELECT id::text FROM ai_model WHERE name='gpt-4o' AND is_active`).Scan(&modelID); err != nil {
		t.Fatalf("seed model route: %v", err)
	}
	providerName := "a01-public-models-" + randCode(8)
	var providerID string
	if err := db.QueryRow(context.Background(),
		`INSERT INTO provider (name, label, "type", base_url, api_key, is_active, priority, models)
		 VALUES ($1,'A-01 test-only provider','openai','https://example.invalid','TEST-ONLY-SENTINEL',true,0,'gpt-4o') RETURNING id`, providerName).Scan(&providerID); err != nil {
		t.Fatalf("seed provider route: %v", err)
	}
	if _, err := db.Exec(context.Background(),
		`INSERT INTO model_provider_route (model_id, provider_id, upstream_model, is_active) VALUES ($1,$2,'gpt-4o',true)`, modelID, providerID); err != nil {
		t.Fatalf("seed model route: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), `DELETE FROM model_provider_route WHERE provider_id=$1`, providerID)
		_, _ = db.Exec(context.Background(), `DELETE FROM provider WHERE id=$1`, providerID)
	})

	rec := httptest.NewRecorder()
	PublicModels(db)(rec, httptest.NewRequest(http.MethodGet, "/api/models", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Models  []map[string]json.RawMessage `json:"models"`
		Billing struct {
			Unit                string  `json:"unit"`
			FreeDailyTokens     int64   `json:"free_daily_tokens"`
			CachedTokenDiscount float64 `json:"cached_token_discount"`
			UsageSurchargeRate  float64 `json:"usage_surcharge_rate"`
		} `json:"billing"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rec.Body.String())
	}
	if len(body.Models) == 0 {
		t.Fatal("catalog kosong")
	}
	unsupported := map[string]bool{}
	rows, err := db.Query(context.Background(), `
		SELECT m.name
		  FROM ai_model m
		 WHERE m.is_active AND NOT (
			EXISTS (
				SELECT 1 FROM model_provider_route r
				JOIN provider p ON p.id=r.provider_id
				 WHERE r.model_id=m.id AND r.is_active AND p.is_active
				   AND (btrim(COALESCE(p.models,''))='' OR EXISTS (
					 SELECT 1 FROM unnest(string_to_array(p.models,',')) AS candidate
					  WHERE btrim(candidate)=r.upstream_model
				   ))
			) OR EXISTS (
				SELECT 1 FROM provider p
				 WHERE p.is_active AND (btrim(COALESCE(p.models,''))='' OR EXISTS (
					 SELECT 1 FROM unnest(string_to_array(p.models,',')) AS candidate
					  WHERE btrim(candidate)=m.name
				   ))
			)
		 )`)
	if err != nil {
		t.Fatalf("query unsupported models: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan unsupported model: %v", err)
		}
		unsupported[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows unsupported models: %v", err)
	}
	for _, model := range body.Models {
		var name string
		if err := json.Unmarshal(model["name"], &name); err != nil {
			t.Fatalf("model name tidak valid: %s", rec.Body.String())
		}
		if unsupported[name] {
			t.Fatalf("model tanpa provider masih tampil: %s", name)
		}
	}
	for _, model := range body.Models {
		if _, hasLegacyMoneyRate := model["pricing"]; hasLegacyMoneyRate {
			t.Fatalf("model masih mengekspos tarif legacy: %s", rec.Body.String())
		}
		if raw, hasLastChecked := model["last_checked"]; hasLastChecked {
			var value string
			if err := json.Unmarshal(raw, &value); err != nil || value == "" {
				t.Fatalf("last_checked tidak valid: %s", rec.Body.String())
			}
		}
	}
	if body.Billing.Unit != "api_token" || body.Billing.FreeDailyTokens <= 0 || body.Billing.CachedTokenDiscount < 0 || body.Billing.CachedTokenDiscount > 1 || body.Billing.UsageSurchargeRate < 0 {
		t.Fatalf("billing tidak valid: %#v", body.Billing)
	}
}
