package handler

// Fraud: velocity check on order creation + denylist/allowlist middlewares.
//
// Order velocity: a user placing > N orders within a rolling window triggers a
// fraud_event row + admin notification. The order itself still succeeds (the
// payment logic is unchanged); the flag is purely for admin review. This is
// safe because it never blocks legitimate single orders — only patterns that
// look like scripted abuse.

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"ai-sell/gateway/internal/apilog"
	"ai-sell/gateway/internal/auth"
)

// CheckOrderVelocity: called at the end of checkoutOrder after the order
// succeeds. Reads the velocity window/threshold from platform_setting so admin
// can tune without a new deploy. Inserts a fraud_event if exceeded.
//
// ponytail: scan is a single COUNT over market_order in the window — cheap at
// this volume. If order volume grows 100x, switch to a Redis sliding-window
// counter keyed by user_id.
func CheckOrderVelocity(ctx context.Context, db DB, userID string, ip string) {
	windowSec := settingFloat(ctx, db, "fraud_order_velocity_window", 60)
	maxOrders := int(settingFloat(ctx, db, "fraud_order_velocity_max", 3))
	if maxOrders < 1 {
		maxOrders = 3
	}
	var count int
	err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM market_order WHERE buyer_id=$1 AND created_at > now() - interval '1 second' * $2`,
		userID, int(windowSec)).Scan(&count)
	if err != nil {
		log.Printf("[WARN] fraud velocity check failed: user=%s err=%v", userID, err)
		return
	}
	if count > maxOrders {
		// Flag the user so RequireAuth / RequireAdmin routes can surface it.
		if _, err := db.Exec(ctx,
			`UPDATE "user" SET fraud_flag=true, fraud_reason=$1 WHERE id=$2`,
			strconv.Itoa(count)+" orders in "+strconv.Itoa(int(windowSec))+"s", userID); err != nil {
			log.Printf("[WARN] fraud_flag update failed: user=%s err=%v", userID, err)
		}
		if _, err := db.Exec(ctx,
			`INSERT INTO fraud_event (user_id, kind, detail, ip) VALUES ($1,'ORDER_VELOCITY',$2,$3)`,
			userID, strconv.Itoa(count)+" orders in "+strconv.Itoa(int(windowSec))+"s", ip); err != nil {
			log.Printf("[WARN] fraud_event insert failed: user=%s err=%v", userID, err)
		}
		genAdminFraudAlert(ctx, db, userID, count, int(windowSec))
	}
}

// genAdminFraudAlert: system notification to every admin account.
func genAdminFraudAlert(ctx context.Context, db DB, userID string, count, window int) {
	var admins []string
	rows, err := db.Query(ctx, `SELECT id FROM "user" WHERE role IN ('ADMIN','SUPER_ADMIN')`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		admins = append(admins, id)
	}
	for _, aid := range admins {
		insertNotification(ctx, db, aid, NotifSystem, "Fraud Alert",
			"Pengguna baru mencurigakan: "+strconv.Itoa(count)+" order dalam "+strconv.Itoa(window)+"s. Lihat /admin/users.", &userID, "")
	}
}

// ------------------------------------------------------------------ V1 denylist

// V1OriginDeny: rejects /v1/* requests whose Origin or Referer header matches
// the configured denylist. The denylist lives in platform_setting
// 'v1_origin_denylist' (comma-separated origins or substrings to match).
// This is a denylist (not allowlist): a matching Origin OR Referer is rejected;
// no Origin/Referer header (typical of API clients and curl) is allowed.
func V1OriginDeny(db DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			dl := strings.ToLower(settingStr(r.Context(), db, "v1_origin_denylist", ""))
			if dl == "" {
				next.ServeHTTP(w, r)
				return
			}
			checks := strings.Split(dl, ",")
			origin := strings.ToLower(r.Header.Get("Origin"))
			referer := strings.ToLower(r.Header.Get("Referer"))
			for _, bad := range checks {
				bad = strings.TrimSpace(bad)
				if bad == "" {
					continue
				}
				if (origin != "" && strings.Contains(origin, bad)) ||
					(referer != "" && strings.Contains(referer, bad)) {
					log.Printf("[WARN] v1 denylist reject: origin=%s referer=%s bad=%s", origin, referer, bad)
					apilog.SetError(r.Context(), "origin_not_allowed")
					auth.WriteV1Error(w, http.StatusForbidden, "origin or referer not allowed", "origin_not_allowed")
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ------------------------------------------------------------------ IP gates

func parseIPAllowlist(raw, name string) []*net.IPNet {
	var allow []*net.IPNet
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !strings.Contains(s, "/") {
			ip := net.ParseIP(s)
			if ip == nil {
				log.Printf("[WARN] %s entry %q is not a valid IP/CIDR", name, s)
				continue
			}
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			s += "/" + strconv.Itoa(bits)
		}
		_, cidr, err := net.ParseCIDR(s)
		if err != nil {
			log.Printf("[WARN] %s entry %q is not a valid IP/CIDR", name, s)
			continue
		}
		allow = append(allow, cidr)
	}
	return allow
}

func ipAllowlist(envName string, failClosed bool, label string) func(http.Handler) http.Handler {
	allow := parseIPAllowlist(os.Getenv(envName), envName)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(allow) == 0 {
				if !failClosed {
					next.ServeHTTP(w, r)
					return
				}
				browserError(w, "website masih dalam pembangunan", http.StatusForbidden)
				return
			}
			addr := net.ParseIP(ipOf(r))
			for _, c := range allow {
				if addr != nil && c.Contains(addr) {
					next.ServeHTTP(w, r)
					return
				}
			}
			log.Printf("[WARN] %s IP allowlist reject: ip=%s", label, ipOf(r))
			browserError(w, "website masih dalam pembangunan", http.StatusForbidden)
		})
	}
}

// AdminIPAllowlist: optional IP/CIDR gate for /admin/*. Empty = allow all so
// local development keeps the existing role-only guard.
func AdminIPAllowlist() func(http.Handler) http.Handler {
	return ipAllowlist("ADMIN_IP_ALLOWLIST", false, "admin")
}

// PublicIPAllowlist gates login, signup, reset, and social-auth routes. In
// production an empty/invalid allowlist fails closed; local development stays open.
func PublicIPAllowlist() func(http.Handler) http.Handler {
	return ipAllowlist("SITE_IP_ALLOWLIST", strings.EqualFold(os.Getenv("NODE_ENV"), "production"), "site")
}
