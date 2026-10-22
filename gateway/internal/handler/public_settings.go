package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

func GatewayLive(ctx context.Context, db DB) bool {
	var override string
	_ = db.QueryRow(ctx, `SELECT value FROM platform_setting WHERE key='gateway_live_override'`).Scan(&override)
	switch strings.ToLower(strings.TrimSpace(override)) {
	case "true", "1", "live":
		return true
	case "false", "0", "off":
		return false
	}
	var providers, healthy int
	_ = db.QueryRow(ctx, `SELECT COUNT(*) FROM provider WHERE is_active`).Scan(&providers)
	_ = db.QueryRow(ctx, `SELECT COUNT(DISTINCT model_id) FROM model_health_log WHERE checked_at > now() - interval '10 minutes' AND status IN ('ok','up','healthy','operational','slow')`).Scan(&healthy)
	return providers > 0 && healthy > 0
}

var publicCommunitySettingKeys = []string{
	"community_discord_url",
	"community_telegram_url",
	"community_x_url",
	"community_youtube_url",
}

// PublicCommunitySettings returns only explicitly public community links.
// Admin settings can also contain payment/provider configuration and must never
// be exposed through this endpoint.
func PublicCommunitySettings(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		links := map[string]string{}
		for _, key := range publicCommunitySettingKeys {
			var raw string
			if err := db.QueryRow(r.Context(), `SELECT value FROM platform_setting WHERE key=$1`, key).Scan(&raw); err == nil {
				if link := publicHTTPURL(raw); link != "" {
					links[key] = link
				}
			}
		}
		ok(w, map[string]any{"links": links})
	}
}

type marketBanner struct {
	ImageURL string `json:"image_url"`
	LinkURL  string `json:"link_url,omitempty"`
	Alt      string `json:"alt,omitempty"`
	Active   bool   `json:"active"`
}

func PublicMarketBanners(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var raw string
		if err := db.QueryRow(r.Context(), `SELECT value FROM platform_setting WHERE key='market_banners'`).Scan(&raw); err != nil {
			ok(w, map[string]any{"banners": []marketBanner{}})
			return
		}
		var stored []marketBanner
		_ = json.Unmarshal([]byte(raw), &stored)
		out := make([]marketBanner, 0, len(stored))
		for _, banner := range stored {
			banner.ImageURL = publicAssetURL(banner.ImageURL)
			banner.LinkURL = publicLinkURL(banner.LinkURL)
			banner.Alt = strings.TrimSpace(banner.Alt)
			if banner.Active && banner.ImageURL != "" {
				out = append(out, banner)
			}
		}
		ok(w, map[string]any{"banners": out})
	}
}

func publicHTTPURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	return u.String()
}

func publicAssetURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return raw
	}
	return publicHTTPURL(raw)
}

func publicLinkURL(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	return publicAssetURL(raw)
}
