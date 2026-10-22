package handler

import (
	"context"
	"log"
	"strings"
	"time"

	"ai-sell/gateway/internal/secret"
)

// modelhealth.go — status per MODEL (beda dari Uptime Kuma: Kuma memantau
// endpoint HTTP, tidak tahu apa itu model/TTFT).
//
// Sumber data: model_health_log. Diisi dua jalur:
//  1. tombol CHECK di /admin/models (satu kali, manual);
//  2. StartModelHealthProbe — probe berkala tiap provider aktif.
//
// Trafik nyata pengguna TIDAK ikut (usage_log tidak menyimpan latensi) —
// availability di sini murni dari probe, sama seperti arti "sampel" di Kuma.
func StartModelHealthProbe(db DB, interval time.Duration) {
	go func() {
		for {
			probeAllModels(context.Background(), db)
			time.Sleep(interval)
		}
	}()
}

type probeModel struct {
	upstream string
	modelID  *string
}

type probeTarget struct {
	providerID, providerName, baseURL, key, apiType string
	models                                          []probeModel
}

func probeAllModels(ctx context.Context, db DB) {
	// Katalog model (dipakai kalau provider tidak membatasi `models`).
	catalog, ids, err := modelNames(ctx, db)
	if err != nil {
		log.Printf("[WARN] probe model: katalog gagal dibaca: %v", err)
		return
	}
	routeRows, err := db.Query(ctx,
		`SELECT r.provider_id,m.id,m.name,r.upstream_model FROM model_provider_route r
		   JOIN ai_model m ON m.id=r.model_id WHERE r.is_active AND m.is_active`)
	if err != nil {
		log.Printf("[WARN] probe model: route gagal dibaca: %v", err)
		return
	}
	type route struct{ modelID, public, upstream string }
	routes := map[string][]route{}
	for routeRows.Next() {
		var providerID string
		var x route
		if routeRows.Scan(&providerID, &x.modelID, &x.public, &x.upstream) == nil {
			routes[providerID] = append(routes[providerID], x)
		}
	}
	routeRows.Close()

	rows, err := db.Query(ctx,
		`SELECT id,name,base_url,api_key,COALESCE(models,''),COALESCE(api_type,'chat_completions') FROM provider WHERE is_active ORDER BY priority DESC`)
	if err != nil {
		log.Printf("[WARN] probe model: provider gagal dibaca: %v", err)
		return
	}
	var targets []probeTarget
	for rows.Next() {
		var t probeTarget
		var stored, csv string
		if err := rows.Scan(&t.providerID, &t.providerName, &t.baseURL, &stored, &csv, &t.apiType); err != nil {
			continue
		}
		key, err := secret.Decrypt(stored)
		if err != nil {
			continue // kunci rusak: provider ini sudah dilewati registry juga
		}
		t.key = key
		if strings.TrimSpace(csv) == "" {
			for _, public := range catalog {
				upstream, id := public, ids[public]
				for _, mapped := range routes[t.providerID] {
					if mapped.public == public {
						upstream, id = mapped.upstream, mapped.modelID
						break
					}
				}
				t.models = append(t.models, probeModel{upstream: upstream, modelID: &id})
			}
		} else {
			for _, upstream := range strings.Split(csv, ",") {
				upstream = strings.TrimSpace(upstream)
				if upstream == "" {
					continue
				}
				var modelID *string
				for _, mapped := range routes[t.providerID] {
					if mapped.upstream == upstream {
						id := mapped.modelID
						modelID = &id
						break
					}
				}
				if modelID == nil {
					if id, ok := ids[upstream]; ok {
						modelID = &id
					}
				}
				t.models = append(t.models, probeModel{upstream: upstream, modelID: modelID})
			}
		}
		targets = append(targets, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		log.Printf("[WARN] probe model: rows error: %v", err)
		return
	}

	for _, t := range targets {
		for _, model := range t.models {
			var status string
			var latency, code int
			var healthy bool
			if t.apiType == "responses" {
				start := time.Now()
				code, healthy, _ = pingResponses(strings.TrimRight(t.baseURL, "/"), t.key, model.upstream)
				latency = int(time.Since(start).Milliseconds())
				if healthy {
					status = "ok"
				} else if code >= 500 || code == 0 {
					status = "down"
				} else {
					status = "error"
				}
			} else {
				status, latency, code = probeOne(strings.TrimRight(t.baseURL, "/")+"/chat/completions", t.key, model.upstream)
				healthy = status == "ok" || status == "slow"
			}
			// model_id menunjuk alias publik walau identifier upstream berbeda.
			modelID := model.modelID
			var codeArg any
			if code > 0 {
				codeArg = code
			}
			if _, err := db.Exec(ctx,
				`INSERT INTO model_health_log (model_id, provider_id, status, latency_ms, status_code)
				 VALUES ($1,$2,$3,$4,$5)`, modelID, t.providerID, status, latency, codeArg); err != nil {
				log.Printf("[WARN] simpan health log gagal: provider=%s model=%s err=%v", t.providerName, model.upstream, err)
			}
			if _, err := db.Exec(ctx,
				`UPDATE provider SET is_healthy=$1, last_checked=now(), last_check_status=$2, last_latency_ms=$3, updated_at=now() WHERE id=$4`,
				healthy, status, latency, t.providerID); err != nil {
				log.Printf("[WARN] update status provider gagal: provider=%s err=%v", t.providerName, err)
			}
		}
	}
}

// probeOne: satu ping chat (max_tokens=16, lihat pingProvider) → status +
// latensi ms + kode HTTP.
// Ambang "slow" sama dengan runUptimeChecks supaya satu bahasa status.
func probeOne(url, key, model string) (status string, latencyMS, statusCode int) {
	start := time.Now()
	code, healthy, _ := pingProvider(url, key, model)
	latencyMS = int(time.Since(start).Milliseconds())
	switch {
	case healthy && latencyMS > 3000:
		return "slow", latencyMS, code
	case healthy:
		return "ok", latencyMS, code
	case code >= 500 || code == 0:
		return "down", latencyMS, code
	default:
		return "error", latencyMS, code
	}
}

// modelNames: katalog model aktif → daftar nama + peta nama→UUID (untuk model_id
// di health log). Nilai dikembalikan, bukan disimpan di variabel global —
// probe jalan di goroutine, jadi state bersama = race.
func modelNames(ctx context.Context, db DB) ([]string, map[string]string, error) {
	// ponytail: EMBEDDING/RERANK tidak diuji lewat /chat/completions — model itu
	// butuh endpoint lain; tambahkan saat katalog punya model embedding nyata.
	rows, err := db.Query(ctx, `SELECT id, name FROM ai_model WHERE is_active AND "type" NOT IN ('EMBEDDING','RERANK') ORDER BY sort_weight DESC, name`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	ids := map[string]string{}
	var out []string
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		ids[name] = id
		out = append(out, name)
	}
	return out, ids, rows.Err()
}
