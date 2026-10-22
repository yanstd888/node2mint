// Provider registry: load aktif dari DB, failover urut priority.
// Ponytail: semua provider gunakan OpenAI-compatible Chat interface.
package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"ai-sell/gateway/internal/secret"

	"github.com/jackc/pgx/v5/pgxpool"
)

// entry: provider + daftar model yang dilayaninya (CSV; kosong = semua).
// Pool key = beberapa baris provider dengan base_url sama; tiap baris punya
// cooldown sendiri — key yang kena 429/kuota habis disingkirkan sementara,
// saudaranya di base_url yang sama tetap dilayani (pola 9router).
type entry struct {
	id             string
	p              Provider
	models         string
	routes         map[string]string // model publik -> model upstream
	name           string
	degraded       bool      // is_healthy=false dari Check admin: dicoba paling akhir
	unhealthyUntil time.Time // cooldown setelah kegagalan; nol = siap
}

func (e entry) cooling(now time.Time) bool { return now.Before(e.unhealthyUntil) }

// cooldownFrom: lama cooldown dari pesan galat provider ("provider <code>: …").
func cooldownFrom(err error) time.Duration {
	var code int
	if n, _ := fmt.Sscanf(err.Error(), "provider %d:", &code); n == 1 {
		switch {
		case code == 429:
			return time.Minute // rate limit: sekejap lagi
		case code == 401 || code == 402 || code == 403:
			return 10 * time.Minute // kunci/kuota: kecil kemungkinan pulih cepat
		default:
			return 30 * time.Second // 400 dkk bisa sekadar nama model salah
		}
	}
	return 30 * time.Second // jaringan/timeout
}

// Registry: urutan provider (priority tinggi dulu).
type Registry struct {
	mu        sync.RWMutex
	providers []entry
}

// CallMeta identifies the provider that handled a request and whether a
// previous candidate failed before this provider succeeded.
type CallMeta struct {
	Provider string
	Fallback bool
}

// LoadFromDB: load provider aktif, urutkan priority desc (ORDER BY sudah urut).
func LoadFromDB(ctx context.Context, db *pgxpool.Pool) (*Registry, error) {
	rows, err := db.Query(ctx,
		`SELECT provider_id,name,base_url,api_key,models,is_healthy,api_type,entry_name
		   FROM (
		     SELECT p.id provider_id,p.name,p.base_url,p.api_key,COALESCE(p.models,'') models,
		            p.is_healthy,COALESCE(p.api_type,'chat_completions') api_type,
		            p.name||':primary' entry_name,p.priority,p.created_at
		       FROM provider p WHERE p.is_active
		     UNION ALL
		     SELECT p.id,p.name,p.base_url,k.api_key,COALESCE(p.models,''),p.is_healthy,
		            COALESCE(p.api_type,'chat_completions'),p.name||':'||k.id::text,p.priority,k.created_at
		       FROM provider p JOIN provider_api_key k ON k.provider_id=p.id AND k.is_active
		      WHERE p.is_active
		   ) pooled ORDER BY priority DESC,created_at`)
	if err != nil {
		return nil, err
	}
	var provs []entry
	for rows.Next() {
		var id, name, baseURL, stored, models, apiType, entryName string
		var healthy bool
		if err := rows.Scan(&id, &name, &baseURL, &stored, &models, &healthy, &apiType, &entryName); err != nil {
			rows.Close()
			return nil, err
		}
		// Kolom api_key berisi ciphertext (AES-256-GCM, lihat internal/secret).
		// Baris lama yang belum terenkripsi diteruskan apa adanya oleh Decrypt,
		// jadi provider yang dikonfigurasi sebelum enkripsi dipasang tidak mati.
		key, err := secret.Decrypt(stored)
		if err != nil {
			// Gagal dekripsi = kunci tidak bisa dipakai. Provider ini JANGAN
			// dimasukkan: failover akan mencobanya dan gagal dengan 401 yang
			// membingungkan.
			log.Printf("[ERROR] provider %s: kunci API tidak bisa didekripsi: %v", name, err)
			continue
		}
		cfg := Config{APIKey: key, BaseURL: baseURL}
		// Dua bentuk upstream: chat_completions (mayoritas) dan responses
		// (OpenAI Responses API — dipakai sebagian agregator). api_type kosong
		// dianggap chat_completions supaya baris lama tetap jalan.
		p := Provider(NewOpenAI(cfg))
		if apiType == "responses" {
			p = NewResponses(cfg)
		}
		provs = append(provs, entry{id: id, p: p, models: models, routes: map[string]string{}, name: entryName, degraded: !healthy})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	routeRows, err := db.Query(ctx,
		`SELECT r.provider_id, m.name, r.upstream_model
		   FROM model_provider_route r
		   JOIN ai_model m ON m.id=r.model_id
		  WHERE r.is_active AND m.is_active`)
	if err != nil {
		return nil, err
	}
	defer routeRows.Close()
	byID := make(map[string][]int, len(provs))
	for i := range provs {
		byID[provs[i].id] = append(byID[provs[i].id], i)
	}
	routed := make(map[string]struct{})
	for routeRows.Next() {
		var providerID, publicModel, upstreamModel string
		if err := routeRows.Scan(&providerID, &publicModel, &upstreamModel); err != nil {
			return nil, err
		}
		mergeRoute(provs, byID, routed, providerID, publicModel, upstreamModel)
	}
	if err := routeRows.Err(); err != nil {
		return nil, err
	}
	// Provider yang ditandai tidak sehat oleh tombol Check admin tetap dipakai,
	// tapi dipindah ke belakang: dicoba hanya kalau semua yang sehat gagal.
	// ponytail: urutan ini ikut nilai is_healthy saat load/reload terakhir —
	// reload ulang setelah Check admin kalau butuh langsung terurut.
	sort.SliceStable(provs, func(i, j int) bool { return !provs[i].degraded && provs[j].degraded })
	return &Registry{providers: provs}, nil
}

// Reload applies provider admin changes without a gateway restart.
func (r *Registry) Reload(ctx context.Context, db *pgxpool.Pool) error {
	fresh, err := LoadFromDB(ctx, db)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.providers = fresh.providers
	r.mu.Unlock()
	return nil
}

// mergeRoute makes every provider opt out of an explicit public alias once,
// then assigns that alias to the provider owning each route row.
func mergeRoute(provs []entry, byID map[string][]int, initialized map[string]struct{}, providerID, publicModel, upstreamModel string) {
	if _, ok := initialized[publicModel]; !ok {
		for i := range provs {
			provs[i].routes[publicModel] = ""
		}
		initialized[publicModel] = struct{}{}
	}
	for _, i := range byID[providerID] {
		provs[i].routes[publicModel] = upstreamModel
	}
}

// routeModel resolves a public alias into its upstream model identifier.
func routeModel(e entry, model string) (string, bool) {
	if upstream, routed := e.routes[model]; routed {
		return upstream, upstream != ""
	}
	if e.models == "" {
		return model, true
	}
	for _, m := range strings.Split(e.models, ",") {
		if strings.TrimSpace(m) == model {
			return model, true
		}
	}
	return "", false
}

// pick: failover candidates yang melayani `model` (model kosong = semua).
func (r *Registry) pick() ([]entry, error) {
	r.mu.RLock()
	provs := append([]entry(nil), r.providers...)
	r.mu.RUnlock()
	if len(provs) == 0 {
		return nil, ErrNoProvider
	}
	return provs, nil
}

// cool: set/bersihkan cooldown satu provider (dicocokkan lewat nama).
// ponytail: pencarian O(n) — provider aktif jumlahnya puluhan, bukan ribuan.
func (r *Registry) cool(name string, until time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.providers {
		if r.providers[i].name == name {
			r.providers[i].unhealthyUntil = until
			return
		}
	}
}

// attemptAll: jalankan call ke provider yang melayani model, urut prioritas,
// lewati yang sedang cooldown. Kalau SEMUA sedang cooldown, yang cooldown tetap
// dicoba — gagal total lebih buruk daripada memakai key yang mungkin pulih.
func (r *Registry) attemptAll(model string, call func(entry, string) (UsageUsage, error)) (CallMeta, error) {
	provs, err := r.pick()
	if err != nil {
		return CallMeta{}, err
	}
	now := time.Now()
	var cooling []entry
	serving := 0
	failed := false
	var meta CallMeta
	var lastErr error

	attempt := func(e entry, upstreamModel string) (bool, error) {
		serving++
		meta.Provider = e.name
		meta.Fallback = failed
		_, err := call(e, upstreamModel)
		if err == nil {
			r.cool(e.name, time.Time{})
			return true, nil
		}
		failed = true
		cooldown := cooldownFrom(err)
		r.cool(e.name, now.Add(cooldown))
		log.Printf("[WARN] provider %s gagal (cooldown %s): %v", e.name, cooldown, err)
		return false, err
	}

	for _, e := range provs {
		upstreamModel, ok := routeModel(e, model)
		if !ok {
			continue
		}
		if e.cooling(now) {
			cooling = append(cooling, e)
			continue
		}
		if done, err := attempt(e, upstreamModel); done {
			return meta, nil
		} else {
			lastErr = err
		}
	}
	for _, e := range cooling {
		upstreamModel, _ := routeModel(e, model)
		if served, err := attempt(e, upstreamModel); served {
			return meta, nil
		} else {
			lastErr = err
		}
	}
	if serving == 0 {
		return meta, fmt.Errorf("tidak ada provider yang melayani model %q", model)
	}
	return meta, lastErr
}

// ChatWithFailover: coba provider pertama, ganti provider lain kalau gagal.
// Provider yang tidak melayani model ini (kolom models CSV) dilewati, begitu
// juga yang sedang cooldown akibat kegagalan sebelumnya.
func (r *Registry) ChatWithMeta(req RequestChat) (UsageUsage, CallMeta, error) {
	var usage UsageUsage
	meta, err := r.attemptAll(req.Model, func(e entry, upstreamModel string) (UsageUsage, error) {
		upstreamReq := req
		upstreamReq.Model = upstreamModel
		u, err := e.p.Chat(upstreamReq)
		if err == nil {
			usage = u
		}
		return u, err
	})
	return usage, meta, err
}

func (r *Registry) Chat(req RequestChat) (UsageUsage, error) {
	usage, _, err := r.ChatWithMeta(req)
	return usage, err
}

// StreamChat: sama seperti Chat, tapi SSE — tiap delta lewat onDelta.
func (r *Registry) StreamChatWithMeta(req RequestChat, onDelta func(string)) (UsageUsage, CallMeta, error) {
	var usage UsageUsage
	meta, err := r.attemptAll(req.Model, func(e entry, upstreamModel string) (UsageUsage, error) {
		upstreamReq := req
		upstreamReq.Model = upstreamModel
		u, err := e.p.StreamChat(upstreamReq, onDelta)
		if err == nil {
			usage = u
		}
		return u, err
	})
	return usage, meta, err
}

func (r *Registry) StreamChat(req RequestChat, onDelta func(string)) (UsageUsage, error) {
	usage, _, err := r.StreamChatWithMeta(req, onDelta)
	return usage, err
}

// HashKey untuk lookup API key.
func HashKey(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

var ErrNoProvider = errors.New("no active provider available")
