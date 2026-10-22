package handler

// Admin: CRUD katalog model (+harga per 1k) dan cek koneksi provider sekali
// (is_healthy + last_checked + baris model_health_log) — port "model management"
// dan "channel test" dari new-api.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"ai-sell/gateway/internal/provider"
	"ai-sell/gateway/internal/secret"
)

var modelTypes = []string{"TEXT", "IMAGE", "VIDEO", "AUDIO", "EMBEDDING", "VISION", "RERANK", "REASONING"}

// normalizeModelTypes keeps the persisted representation backward-compatible
// (a comma-separated TEXT value) while enforcing the UI contract: 1–3 unique
// capabilities from the allowlist.
func normalizeModelTypes(raw string) (string, bool) {
	seen := make(map[string]bool)
	parts := strings.Split(raw, ",")
	if len(parts) < 1 || len(parts) > 3 {
		return "", false
	}
	for i, part := range parts {
		part = strings.ToUpper(strings.TrimSpace(part))
		if part == "" || seen[part] {
			return "", false
		}
		seen[part] = true
		valid := false
		for _, allowed := range modelTypes {
			if allowed == part {
				valid = true
				break
			}
		}
		if !valid {
			return "", false
		}
		parts[i] = part
	}
	return strings.Join(parts, ","), true
}

func validModelType(t string) bool {
	_, ok := normalizeModelTypes(t)
	return ok
}

const modelCols = `SELECT m.id, m.name, m."type", m.is_active, COALESCE(m.description,''),
        COALESCE(m.context_length,0), COALESCE(m.max_output_tokens,0), m.sort_weight,
        m.tier, m.token_multiplier, COALESCE(m.family,''),
        COALESCE(r.provider_id::text,''),
        COALESCE(pr.label,pr.name,''), COALESCE(r.upstream_model,'')
   FROM ai_model m
   LEFT JOIN LATERAL (
     SELECT r.provider_id,r.upstream_model
       FROM model_provider_route r
       JOIN provider route_provider ON route_provider.id=r.provider_id
      WHERE r.model_id=m.id AND r.is_active
      ORDER BY route_provider.priority DESC,route_provider.created_at,r.provider_id
      LIMIT 1
   ) r ON true
   LEFT JOIN provider pr ON pr.id=r.provider_id`

type adminModel struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Type            string  `json:"type"`
	IsActive        bool    `json:"is_active"`
	Description     string  `json:"description"`
	ContextLength   int     `json:"context_length"`
	MaxOutputTokens int     `json:"max_output_tokens"`
	SortWeight      int     `json:"sort_weight"`
	Tier            string  `json:"tier"`
	TokenMultiplier float64 `json:"token_multiplier"`
	Family          string  `json:"family,omitempty"`
	ProviderID      string  `json:"provider_id"`
	ProviderName    string  `json:"provider_name"`
	UpstreamModel   string  `json:"upstream_model"`
}

// AdminModelsList: GET /api/admin/models.
func AdminModelsList(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(r.Context(), modelCols+` ORDER BY m.sort_weight DESC, m.name`)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		out := []adminModel{}
		for rows.Next() {
			var m adminModel
			if err := rows.Scan(&m.ID, &m.Name, &m.Type, &m.IsActive, &m.Description,
				&m.ContextLength, &m.MaxOutputTokens, &m.SortWeight, &m.Tier, &m.TokenMultiplier,
				&m.Family, &m.ProviderID, &m.ProviderName, &m.UpstreamModel); err != nil {
				continue
			}
			out = append(out, m)
		}
		ok(w, map[string]any{"models": out})
	}
}

// modelIn: field yang boleh diisi/diubah. Pointer = "tidak dikirim".
type modelIn struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	Description     string   `json:"description"`
	IsActive        *bool    `json:"is_active"`
	ContextLength   *int     `json:"context_length"`
	MaxOutputTokens *int     `json:"max_output_tokens"`
	SortWeight      *int     `json:"sort_weight"`
	Tier            string   `json:"tier"`             // FREE | PAID
	Family          string   `json:"family"`           // anthropic | openai | google | deepseek | xai | meta | miscralai | moonshot | alibaba | ''=auto
	TokenMultiplier *float64 `json:"token_multiplier"` // 1 = 1:1, 3 = 1 token upstream → 3 token user
	ProviderID      *string  `json:"provider_id"`
	UpstreamModel   *string  `json:"upstream_model"`
}

// validFamily: brand katalog model yang bisa dipilih di admin. ” = auto-detect.
var validFamily = map[string]bool{
	"": true, "anthropic": true, "openai": true, "google": true,
	"deepseek": true, "xai": true, "meta": true, "mistralai": true,
	"moonshot": true, "alibaba": true,
}

// inferFamily: deteksi otomatis brand dari nama model (fallback saat family kosong).
var familyPatterns = []struct{ family, pattern string }{
	{"anthropic", `(?i)claude|anthropic`},
	{"openai", `(?i)\bgpt|openai`},
	{"google", `(?i)gemini|google|palm`},
	{"deepseek", `(?i)deepseek`},
	{"xai", `(?i)grok`},
	{"meta", `(?i)llama|meta`},
	{"mistralai", `(?i)mistral|mixtral`},
	{"moonshot", `(?i)kimi|moonshot`},
	{"alibaba", `(?i)qwen|glm`},
}

func inferFamily(name string) string {
	for _, p := range familyPatterns {
		if regexp.MustCompile(p.pattern).MatchString(name) {
			return p.family
		}
	}
	return "other"
}

// AdminModelsCreate: POST /api/admin/models.
func AdminModelsCreate(db DB, reg *provider.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var in modelIn
		if err := decode(r, &in); err != nil || in.Name == "" {
			fail(w, http.StatusBadRequest, "name wajib")
			return
		}
		if in.Type == "" {
			in.Type = "TEXT"
		}
		normalizedType, valid := normalizeModelTypes(in.Type)
		if !valid {
			fail(w, http.StatusBadRequest, "type tidak dikenal (pilih 1–3 tipe unik)")
			return
		}
		in.Type = normalizedType
		if err := validModelRoute(in.ProviderID, in.UpstreamModel); err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		if in.Tier != "" && in.Tier != "FREE" && in.Tier != "PAID" {
			fail(w, http.StatusBadRequest, "tier harus FREE atau PAID")
			return
		}
		if !validFamily[in.Family] {
			fail(w, http.StatusBadRequest, "family tidak dikenali")
			return
		}
		// family kosong → deteksi otomatis dari nama agar urutan katalog tetap konsisten.
		family := in.Family
		if family == "" {
			family = inferFamily(in.Name)
		}
		if in.TokenMultiplier != nil && (*in.TokenMultiplier < 0 || *in.TokenMultiplier > 1000) {
			fail(w, http.StatusBadRequest, "token_multiplier tidak wajar (0–1000)")
			return
		}
		supported, err := modelInputSupported(r.Context(), db, in.Name, in.ProviderID, in.UpstreamModel)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memeriksa provider model")
			return
		}
		active := supported
		if in.IsActive != nil && !*in.IsActive {
			active = false
		}
		if in.IsActive != nil && *in.IsActive && !supported {
			fail(w, http.StatusConflict, "model aktif wajib memiliki provider aktif dan route yang didukung")
			return
		}
		var id string
		err = db.QueryRow(r.Context(),
			`INSERT INTO ai_model (name, "type", description, context_length, max_output_tokens, sort_weight, tier, family, token_multiplier, is_active)
			 VALUES ($1,$2,$3,$4,$5,COALESCE($6,0),COALESCE(NULLIF($7,''),'PAID'),$8,COALESCE($9,1.0),$10) RETURNING id`,
			in.Name, in.Type, nullStr(in.Description), in.ContextLength, in.MaxOutputTokens, in.SortWeight,
			in.Tier, family, in.TokenMultiplier, active).Scan(&id)
		if err != nil {
			fail(w, http.StatusConflict, "gagal membuat model (nama mungkin sudah ada)")
			return
		}
		if err := upsertModelRoute(r, db, id, in.ProviderID, in.UpstreamModel); err != nil {
			_, _ = db.Exec(r.Context(), `DELETE FROM ai_model WHERE id=$1`, id)
			fail(w, http.StatusBadRequest, "provider atau model upstream tidak valid")
			return
		}
		if err := reg.Reload(r.Context(), db); err != nil {
			fail(w, http.StatusInternalServerError, "model tersimpan tapi routing gagal dimuat")
			return
		}
		audit(r.Context(), db, uid, "MODEL_CREATE", id, nil, in, ipOf(r))
		ok(w, map[string]any{"ok": true, "id": id})
	}
}

// AdminModelsPatch: PATCH /api/admin/models/{id} (id boleh di body juga).
func AdminModelsPatch(db DB, reg *provider.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var in modelIn
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		id := r.PathValue("id")
		if id == "" {
			id = in.ID
		}
		if id == "" {
			fail(w, http.StatusBadRequest, "id wajib")
			return
		}
		if in.Type != "" {
			normalizedType, valid := normalizeModelTypes(in.Type)
			if !valid {
				fail(w, http.StatusBadRequest, "type tidak dikenal (pilih 1–3 tipe unik)")
				return
			}
			in.Type = normalizedType
		}
		if err := validModelRoute(in.ProviderID, in.UpstreamModel); err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		routeChanged := in.ProviderID != nil || in.UpstreamModel != nil
		var currentName string
		var currentActive bool
		if err := db.QueryRow(r.Context(), `SELECT name, is_active FROM ai_model WHERE id=$1`, id).Scan(&currentName, &currentActive); err != nil {
			fail(w, http.StatusNotFound, "model tidak ditemukan")
			return
		}
		activationRequested := in.IsActive != nil && *in.IsActive
		if activationRequested || (currentActive && routeChanged) {
			var supported bool
			var err error
			if routeChanged {
				supported, err = modelInputSupported(r.Context(), db, currentName, in.ProviderID, in.UpstreamModel)
			} else {
				supported, err = modelProviderSupported(r.Context(), db, id)
			}
			if err != nil {
				fail(w, http.StatusInternalServerError, "gagal memeriksa provider model")
				return
			}
			if !supported {
				fail(w, http.StatusConflict, "model aktif wajib memiliki provider aktif dan route yang didukung")
				return
			}
		}

		sets := []string{"updated_at=now()"}
		args := []any{}
		add := func(col string, v any) {
			args = append(args, v)
			sets = append(sets, col+"=$"+itoa(len(args)))
		}
		if in.Name != "" {
			add("name", in.Name)
		}
		if in.Type != "" {
			add(`"type"`, in.Type)
		}
		if in.Description != "" {
			add("description", in.Description)
		}
		if in.IsActive != nil {
			add("is_active", *in.IsActive)
		}
		if in.ContextLength != nil {
			add("context_length", *in.ContextLength)
		}
		if in.MaxOutputTokens != nil {
			add("max_output_tokens", *in.MaxOutputTokens)
		}
		if in.SortWeight != nil {
			add("sort_weight", *in.SortWeight)
		}
		if in.Tier != "" {
			if in.Tier != "FREE" && in.Tier != "PAID" {
				fail(w, http.StatusBadRequest, "tier harus FREE atau PAID")
				return
			}
			add("tier", in.Tier)
		}
		if in.Family != "" {
			if !validFamily[in.Family] {
				fail(w, http.StatusBadRequest, "family tidak dikenal")
				return
			}
			add("family", in.Family)
		}
		if in.TokenMultiplier != nil {
			if *in.TokenMultiplier < 0 || *in.TokenMultiplier > 1000 {
				fail(w, http.StatusBadRequest, "token_multiplier tidak wajar (0–1000)")
				return
			}
			add("token_multiplier", *in.TokenMultiplier)
		}
		args = append(args, id)
		res, err := db.Exec(r.Context(),
			`UPDATE ai_model SET `+strings.Join(sets, ", ")+` WHERE id=$`+itoa(len(args)), args...)
		if err != nil {
			fail(w, http.StatusInternalServerError, "update failed")
			return
		}
		if res.RowsAffected() == 0 {
			fail(w, http.StatusNotFound, "model tidak ditemukan")
			return
		}
		if err := upsertModelRoute(r, db, id, in.ProviderID, in.UpstreamModel); err != nil {
			fail(w, http.StatusBadRequest, "provider atau model upstream tidak valid")
			return
		}
		if err := reg.Reload(r.Context(), db); err != nil {
			fail(w, http.StatusInternalServerError, "model tersimpan tapi routing gagal dimuat")
			return
		}
		audit(r.Context(), db, uid, "MODEL_UPDATE", id, nil, in, ipOf(r))
		ok(w, map[string]any{"ok": true})
	}
}

// AdminProviderImportModels: GET upstream /models and register only models that
// are not already in the catalog. Imported models stay inactive until an admin
// supplies owner cost/multiplier data and explicitly enables them.
func AdminProviderImportModels(db DB, reg *provider.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerID := r.PathValue("id")
		if providerID == "" {
			fail(w, http.StatusBadRequest, "id wajib")
			return
		}
		var baseURL, stored string
		if err := db.QueryRow(r.Context(), `SELECT base_url, api_key FROM provider WHERE id=$1`, providerID).Scan(&baseURL, &stored); err != nil {
			fail(w, http.StatusNotFound, "provider tidak ditemukan")
			return
		}
		key, err := secret.Decrypt(stored)
		if err != nil {
			fail(w, http.StatusBadRequest, "kunci provider tidak bisa dipakai")
			return
		}
		models, err := fetchProviderModels(r.Context(), baseURL, key)
		if err != nil {
			fail(w, http.StatusBadGateway, "gagal mengambil model upstream")
			return
		}
		if len(models) == 0 {
			fail(w, http.StatusBadRequest, "provider tidak mengembalikan model")
			return
		}

		tx, err := db.Begin(r.Context())
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memulai import")
			return
		}
		defer tx.Rollback(r.Context())

		var current string
		if err := tx.QueryRow(r.Context(), `SELECT COALESCE(models,'') FROM provider WHERE id=$1 FOR UPDATE`, providerID).Scan(&current); err != nil {
			fail(w, http.StatusNotFound, "provider tidak ditemukan")
			return
		}
		available := mergeModelNames(current, models)
		imported := 0
		for _, name := range models {
			var modelID string
			err := tx.QueryRow(r.Context(),
				`INSERT INTO ai_model (name,"type",is_active,tier,family,token_multiplier)
				 VALUES ($1,'TEXT',false,'PAID',$2,1.0)
				 ON CONFLICT (name) DO NOTHING RETURNING id`, name, inferFamily(name)).Scan(&modelID)
			if err != nil {
				continue // existing catalog name: never overwrite routing, pricing, or activation.
			}
			if _, err := tx.Exec(r.Context(),
				`INSERT INTO model_provider_route (model_id,provider_id,upstream_model,is_active,updated_at)
				 VALUES ($1,$2,$3,true,now())`, modelID, providerID, name); err != nil {
				fail(w, http.StatusInternalServerError, "gagal menyimpan routing model")
				return
			}
			imported++
		}
		if _, err := tx.Exec(r.Context(), `UPDATE provider SET models=$1, updated_at=now() WHERE id=$2`, strings.Join(available, ","), providerID); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menyimpan daftar model")
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menyimpan import")
			return
		}
		if err := reg.Reload(r.Context(), db); err != nil {
			fail(w, http.StatusInternalServerError, "model diimpor tapi routing gagal dimuat")
			return
		}
		audit(r.Context(), db, uidFromCtx(r.Context()), "PROVIDER_MODELS_IMPORT", providerID, nil,
			map[string]any{"discovered": len(models), "imported": imported}, ipOf(r))
		ok(w, map[string]any{"ok": true, "discovered": len(models), "imported": imported, "models": available})
	}
}

// AdminModelsDelete: DELETE /api/admin/models/{id} — harga/badge/health ikut
// terhapus lewat ON DELETE CASCADE.
func AdminModelsDelete(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		id := r.PathValue("id")
		res, err := db.Exec(r.Context(), `DELETE FROM ai_model WHERE id=$1`, id)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal menghapus model")
			return
		}
		if res.RowsAffected() == 0 {
			fail(w, http.StatusNotFound, "model tidak ditemukan")
			return
		}
		audit(r.Context(), db, uid, "MODEL_DELETE", id, nil, map[string]any{"id": id}, ipOf(r))
		ok(w, map[string]any{"ok": true})
	}
}

// validModelRoute requires provider and upstream model as one atomic mapping.
func validModelRoute(providerID, upstreamModel *string) error {
	if providerID == nil && upstreamModel == nil {
		return nil
	}
	if providerID == nil || upstreamModel == nil {
		return fmt.Errorf("provider dan model upstream wajib dikirim bersama")
	}
	provider := strings.TrimSpace(*providerID)
	upstream := strings.TrimSpace(*upstreamModel)
	if (provider == "") != (upstream == "") {
		return fmt.Errorf("provider dan model upstream wajib dipilih bersama")
	}
	if len(upstream) > 255 {
		return fmt.Errorf("nama model upstream terlalu panjang")
	}
	return nil
}

// modelProviderSupported reports whether a model has an active explicit route or
// an active legacy provider whose model list includes the public model name.
func modelProviderSupported(ctx context.Context, db DB, modelID string) (bool, error) {
	var supported bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			  FROM model_provider_route r
			  JOIN provider p ON p.id=r.provider_id
			 WHERE r.model_id=$1 AND r.is_active AND p.is_active
			   AND (btrim(COALESCE(p.models,''))='' OR EXISTS (
				 SELECT 1 FROM unnest(string_to_array(p.models,',')) AS candidate
				  WHERE btrim(candidate)=r.upstream_model
			   ))
		) OR EXISTS (
			SELECT 1
			  FROM ai_model m
			  JOIN provider p ON p.is_active
			 WHERE m.id=$1
			   AND (btrim(COALESCE(p.models,''))='' OR EXISTS (
				 SELECT 1 FROM unnest(string_to_array(p.models,',')) AS candidate
				  WHERE btrim(candidate)=m.name
			   ))
		)`, modelID).Scan(&supported)
	return supported, err
}

func providerSupportsModel(ctx context.Context, db DB, providerID, upstream string) (bool, error) {
	var supported bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM provider p
			 WHERE p.id=$1 AND p.is_active
			   AND (btrim(COALESCE(p.models,''))='' OR EXISTS (
				 SELECT 1 FROM unnest(string_to_array(p.models,',')) AS candidate
				  WHERE btrim(candidate)=$2
			   ))
		)`, providerID, upstream).Scan(&supported)
	return supported, err
}

func legacyProviderSupportsModel(ctx context.Context, db DB, modelName string) (bool, error) {
	var supported bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM provider p
			 WHERE p.is_active
			   AND (btrim(COALESCE(p.models,''))='' OR EXISTS (
				 SELECT 1 FROM unnest(string_to_array(p.models,',')) AS candidate
				  WHERE btrim(candidate)=$1
			   ))
		)`, modelName).Scan(&supported)
	return supported, err
}

func modelInputSupported(ctx context.Context, db DB, modelName string, providerID, upstreamModel *string) (bool, error) {
	if providerID != nil || upstreamModel != nil {
		if providerID == nil || upstreamModel == nil {
			return false, nil
		}
		provider := strings.TrimSpace(*providerID)
		upstream := strings.TrimSpace(*upstreamModel)
		if provider == "" || upstream == "" {
			return false, nil
		}
		return providerSupportsModel(ctx, db, provider, upstream)
	}
	return legacyProviderSupportsModel(ctx, db, modelName)
}

func upsertModelRoute(r *http.Request, db DB, modelID string, providerID, upstreamModel *string) error {
	if providerID == nil && upstreamModel == nil {
		return nil
	}
	provider := strings.TrimSpace(*providerID)
	upstream := strings.TrimSpace(*upstreamModel)
	if provider == "" && upstream == "" {
		_, err := db.Exec(r.Context(), `DELETE FROM model_provider_route WHERE model_id=$1`, modelID)
		return err
	}
	res, err := db.Exec(r.Context(),
		`INSERT INTO model_provider_route (model_id,provider_id,upstream_model,is_active,updated_at)
		 SELECT $1,p.id,$3,true,now() FROM provider p
		  WHERE p.id=$2 AND (btrim(COALESCE(p.models,''))='' OR EXISTS (
		    SELECT 1 FROM unnest(string_to_array(p.models,',')) AS candidate
		     WHERE btrim(candidate)=$3))
		 ON CONFLICT (model_id,provider_id) DO UPDATE SET
		 upstream_model=EXCLUDED.upstream_model,is_active=true,updated_at=now()`,
		modelID, provider, upstream)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("provider tidak ditemukan")
	}
	return nil
}

func providerCheckModel(modelID, models string) string {
	if modelID = strings.TrimSpace(modelID); modelID != "" {
		return modelID
	}
	return strings.TrimSpace(strings.Split(models, ",")[0])
}

// providerTestModel keeps a valid requested model, otherwise uses the first
// model advertised by the upstream. The bool tells the UI to refresh its CSV.
func providerTestModel(requested string, available []string) (string, bool) {
	requested = strings.TrimSpace(requested)
	if len(available) == 0 {
		return requested, false
	}
	for _, model := range available {
		if model == requested {
			return requested, false
		}
	}
	return available[0], true
}

// CheckProviderDraft tests an unsaved provider from its supplied base URL and key.
// It never persists or audits the raw key.
func CheckProviderDraft(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			BaseURL string `json:"base_url"`
			APIKey  string `json:"api_key"`
			Model   string `json:"model"`
			APIType string `json:"api_type"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		u, err := url.Parse(strings.TrimSpace(in.BaseURL))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || strings.TrimSpace(in.APIKey) == "" {
			fail(w, http.StatusBadRequest, "base_url dan api_key wajib valid")
			return
		}
		base := strings.TrimRight(u.String(), "/")
		list := listProviderModels(base, in.APIKey)
		model, replaced := providerTestModel(in.Model, list)
		if model == "" {
			_ = db.QueryRow(r.Context(), `SELECT name FROM ai_model WHERE is_active ORDER BY sort_weight DESC, name LIMIT 1`).Scan(&model)
		}
		if model == "" {
			fail(w, http.StatusBadRequest, "tidak ada model untuk diuji")
			return
		}
		var code int
		var ok2 bool
		var detail string
		if in.APIType == "responses" {
			code, ok2, detail = pingResponses(base, in.APIKey, model)
		} else {
			code, ok2, detail = pingProvider(base+"/chat/completions", in.APIKey, model)
		}
		ok(w, map[string]any{"ok": ok2, "status_code": code, "model": model, "model_replaced": replaced, "models": list, "error": detail})
	}
}

// AdminProviderCheck: POST /api/admin/provider/{id}/check — satu panggilan uji
// ke provider, lalu update is_healthy/last_checked + tulis model_health_log.
// ponytail: cek on-demand, bukan monitor periodik — cron kalau perlu riwayat.
func AdminProviderCheck(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		id := r.PathValue("id")
		var name, baseURL, stored, modelID, models, apiType string
		if err := db.QueryRow(r.Context(),
			`SELECT name, base_url, api_key, COALESCE(model_id,''), COALESCE(models,''), COALESCE(api_type,'chat_completions') FROM provider WHERE id=$1`, id).
			Scan(&name, &baseURL, &stored, &modelID, &models, &apiType); err != nil {
			fail(w, http.StatusNotFound, "provider tidak ditemukan")
			return
		}

		// Model uji: model_id eksplisit, lalu model pertama CSV provider, lalu katalog.
		modelID = providerCheckModel(modelID, models)
		if modelID == "" {
			_ = db.QueryRow(r.Context(),
				`SELECT name FROM ai_model WHERE is_active ORDER BY sort_weight DESC, name LIMIT 1`).Scan(&modelID)
		}
		if modelID == "" {
			fail(w, http.StatusBadRequest, "tidak ada model untuk diuji dan provider.model_id kosong")
			return
		}
		// Baris health perlu FK ai_model: nama UUID/name dipetakan, kalau tidak ada → NULL.
		var aiModelID *string
		var found string
		if err := db.QueryRow(r.Context(),
			`SELECT id FROM ai_model WHERE name=$1 OR id::text=$1 LIMIT 1`, modelID).Scan(&found); err == nil {
			aiModelID = &found
		}

		key, keyErr := secret.Decrypt(stored)
		start := time.Now()
		statusCode, ok2, detail := 0, false, ""
		if keyErr != nil {
			detail = "kunci provider tidak bisa didekripsi: " + keyErr.Error()
		} else if apiType == "responses" {
			statusCode, ok2, detail = pingResponses(baseURL, key, modelID)
		} else {
			statusCode, ok2, detail = pingProvider(baseURL+"/chat/completions", key, modelID)
		}
		latency := int(time.Since(start).Milliseconds())

		status := "ok"
		switch {
		case keyErr != nil:
			status = "down"
		case ok2 && latency > 3000:
			status = "slow"
		case !ok2 && statusCode >= 500:
			status = "down"
		case !ok2:
			status = "error"
		}
		var code any
		if statusCode > 0 {
			code = statusCode
		}
		if _, err := db.Exec(r.Context(),
			`UPDATE provider SET is_healthy=$1, last_checked=now(), last_check_status=$2, last_latency_ms=$3, updated_at=now() WHERE id=$4`,
			status == "ok" || status == "slow", status, latency, id); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menyimpan status")
			return
		}
		if _, err := db.Exec(r.Context(),
			`INSERT INTO model_health_log (model_id, provider_id, status, latency_ms, status_code)
			 VALUES ($1,$2,$3,$4,$5)`, aiModelID, id, status, latency, code); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menulis health log")
			return
		}
		audit(r.Context(), db, uid, "PROVIDER_CHECK", id, nil,
			map[string]any{"provider": name, "status": status, "latency_ms": latency}, ipOf(r))
		ok(w, map[string]any{"ok": ok2, "status": status, "latency_ms": latency, "status_code": statusCode, "model": modelID, "error": detail})
	}
}

// listProviderModels is the non-fatal lookup used by the provider check flow.
func listProviderModels(base, key string) []string {
	models, err := fetchProviderModels(context.Background(), base, key)
	if err != nil {
		return nil
	}
	if len(models) > 100 {
		return models[:100]
	}
	return models
}

// fetchProviderModels reads the OpenAI-compatible /models list without ever
// returning or logging the credential used to authenticate the request.
func fetchProviderModels(ctx context.Context, base, key string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream returned HTTP %d", resp.StatusCode)
	}
	return parseUpstreamModels(io.LimitReader(resp.Body, 1<<20))
}

// parseUpstreamModels accepts the usual OpenAI {data:[{id}]} response, with a
// name fallback for compatible gateways, and removes duplicate IDs/names.
func parseUpstreamModels(r io.Reader) ([]string, error) {
	var payload struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(r).Decode(&payload); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(payload.Data))
	seen := make(map[string]bool, len(payload.Data))
	for _, item := range payload.Data {
		name := strings.TrimSpace(item.ID)
		if name == "" {
			name = strings.TrimSpace(item.Name)
		}
		if name != "" && !seen[name] {
			seen[name] = true
			models = append(models, name)
		}
	}
	return models, nil
}

func mergeModelNames(current string, additions []string) []string {
	models := make([]string, 0, len(additions)+1)
	seen := make(map[string]bool)
	for _, name := range append(strings.Split(current, ","), additions...) {
		name = strings.TrimSpace(name)
		if name != "" && !seen[name] {
			seen[name] = true
			models = append(models, name)
		}
	}
	return models
}

// pingResponses: satu request kecil ke Responses API. Jalur Check untuk
// provider api_type=responses — endpoint & bentuk beda dari chat/completions.
func pingResponses(base, key, model string) (int, bool, string) {
	body, _ := json.Marshal(map[string]any{
		"model":             model,
		"input":             "ping",
		"max_output_tokens": 16, // router menolak nilai < 16 (lihat pingProvider)
	})
	req, err := http.NewRequest("POST", base+"/responses", bytes.NewReader(body))
	if err != nil {
		return 0, false, err.Error()
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return 0, false, err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if resp.StatusCode < 300 {
		return resp.StatusCode, true, ""
	}
	return resp.StatusCode, false, string(b)
}

// pingProvider: satu request chat kecil. Balik status code + ok + pesan galat.
func pingProvider(url, key, model string) (int, bool, string) {
	body, _ := json.Marshal(map[string]any{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": "ping"}},
		"max_tokens": 16, // batas output terkecil yang diterima router: 1 ditolak (invalid_request)
	})
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, false, err.Error()
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	// ponytail: timeout tetap 10s — jadikan kolom provider.timeout_ms kalau perlu.
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return 0, false, err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if resp.StatusCode < 300 {
		return resp.StatusCode, true, "" // sukses: body provider bukan pesan galat
	}
	return resp.StatusCode, false, string(b)
}
