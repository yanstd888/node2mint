package handler

// Analytics: GET /api/analytics?range=24h|7d|30d&scope=user|seller|platform.
// Port dari web lama (server/src/lib/analytics.ts) ke SQL murni — data yang
// SAMA dibaca dari usage_log / credit_transaction / market_order, tanpa
// infra baru. Scope platform hanya untuk ADMIN/SUPER_ADMIN; scope user lain
// ditolak (cakupan ditentukan peran di server, bukan query string).
//
// ponytail: agregasi jalan di SQL per permintaan; pindah ke rollup harian
// (tabel daily_stats) kalau usage_log jutaan baris.

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

func analyticsRange(r *http.Request) (since time.Time, days int) {
	switch r.URL.Query().Get("range") {
	case "24h":
		days = 1
	case "30d":
		days = 30
	default:
		days = 7
	}
	return time.Now().AddDate(0, 0, -days), days
}

// fillDays mengisi hari kosong dengan 0 — grafik tidak boleh bolong.
func fillDays(rows map[string]float64, since time.Time, days int) []map[string]any {
	out := []map[string]any{}
	for i := days; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i)
		k := d.Format("2006-01-02")
		out = append(out, map[string]any{"day": k, "value": rows[k]})
	}
	return out
}

// dayRows membaca agregat per hari (kolom pertama hari, kedua angka).
func dayRows(rows pgx.Rows) (map[string]float64, error) {
	out := map[string]float64{}
	defer rows.Close()
	for rows.Next() {
		var d time.Time
		var v float64
		if err := rows.Scan(&d, &v); err != nil {
			return nil, err
		}
		out[d.Format("2006-01-02")] += v
	}
	return out, rows.Err()
}

func userOverview(ctx context.Context, db DB, uid string, since time.Time, days int) (map[string]any, error) {
	var requests, inT, outT int64
	var cost, topup float64
	if err := db.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0), COALESCE(SUM(cost),0)
		   FROM usage_log WHERE user_id=$1 AND created_at >= $2`, uid, since).
		Scan(&requests, &inT, &outT, &cost); err != nil {
		return nil, err
	}
	if err := db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount),0) FROM credit_transaction
		  WHERE user_id=$1 AND type='TOPUP' AND status='COMPLETED' AND created_at >= $2`, uid, since).
		Scan(&topup); err != nil {
		return nil, err
	}
	seriesRows, err := db.Query(ctx,
		`SELECT date_trunc('day', created_at), COUNT(*),
		        COALESCE(SUM(input_tokens+output_tokens),0)
		   FROM usage_log
		  WHERE user_id=$1 AND created_at >= $2 GROUP BY 1 ORDER BY 1`, uid, since)
	if err != nil {
		return nil, err
	}
	requestMap := map[string]float64{}
	tokenMap := map[string]float64{}
	for seriesRows.Next() {
		var day time.Time
		var requests, tokens float64
		if err := seriesRows.Scan(&day, &requests, &tokens); err != nil {
			seriesRows.Close()
			return nil, err
		}
		key := day.Format("2006-01-02")
		requestMap[key] = requests
		tokenMap[key] = tokens
	}
	seriesRows.Close()
	if err := seriesRows.Err(); err != nil {
		return nil, err
	}
	modelRows, err := db.Query(ctx,
		`SELECT COALESCE(model,'?'), COUNT(*), COALESCE(SUM(cost),0),
		        COALESCE(SUM(input_tokens+output_tokens),0)
		   FROM usage_log
		  WHERE user_id=$1 AND created_at >= $2 GROUP BY 1 ORDER BY 2 DESC LIMIT 5`, uid, since)
	if err != nil {
		return nil, err
	}
	defer modelRows.Close()
	models := []map[string]any{}
	for modelRows.Next() {
		var m string
		var n int64
		var c, tokens float64
		if err := modelRows.Scan(&m, &n, &c, &tokens); err != nil {
			continue
		}
		models = append(models, map[string]any{"model": m, "requests": n, "cost": c, "tokens": tokens})
	}
	return map[string]any{
		"requests": requests, "inputTokens": inT, "outputTokens": outT,
		"cost": cost, "topup": topup,
		"series": map[string]any{
			"requests": fillDays(requestMap, since, days),
			"tokens":   fillDays(tokenMap, since, days),
		},
		"models": models,
	}, nil
}

func sellerOverview(ctx context.Context, db DB, storeID string, since time.Time, days int) (map[string]any, error) {
	rows, err := db.Query(ctx,
		`SELECT p.id, p.title, p.view_count, p.price,
		        (SELECT COUNT(*) FROM market_order o
		          WHERE o.product_id=p.id AND o.created_at >= $2
		            AND o.status IN ('PAID','PROCESSING','COMPLETED','DISPUTED')),
		        (SELECT COALESCE(SUM(o.subtotal),0) FROM market_order o
		          WHERE o.product_id=p.id AND o.created_at >= $2
		            AND o.status IN ('PAID','PROCESSING','COMPLETED','DISPUTED')),
		        (SELECT COUNT(*) FROM cart_item c WHERE c.product_id=p.id)
		   FROM market_product p
		  WHERE p.store_id = $1
		  ORDER BY 6 DESC, 5 DESC LIMIT 10`, storeID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	products := []map[string]any{}
	var views, sales, revenue, cartAdds float64
	for rows.Next() {
		var id, title string
		var viewCount, units, carts int64
		var price, rev float64
		if err := rows.Scan(&id, &title, &viewCount, &price, &units, &rev, &carts); err != nil {
			continue
		}
		products = append(products, map[string]any{
			"id": id, "title": title, "views": viewCount, "price": price,
			"units": units, "revenue": rev, "cart_adds": carts,
		})
		views += float64(viewCount)
		sales += float64(units)
		revenue += rev
		cartAdds += float64(carts)
	}
	var conversion float64
	if views > 0 {
		conversion = sales / views * 100
	}
	return map[string]any{
		"products": products, "revenue": revenue, "views": views,
		"sales": sales, "cart_adds": cartAdds, "conversion": conversion,
	}, nil
}

func platformOverview(ctx context.Context, db DB, since time.Time, days int) (map[string]any, error) {
	prev := since.AddDate(0, 0, -days)
	var signups, activeUsers, requests, inT, outT int64
	var aiCost, revenue, orderRevenue float64
	var revenueCount, orderCount int64
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM "user" WHERE created_at >= $1`, since).Scan(&signups); err != nil {
		return nil, err
	}
	if err := db.QueryRow(ctx,
		`SELECT COUNT(DISTINCT user_id), COUNT(*), COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0), COALESCE(SUM(cost),0)
		   FROM usage_log WHERE created_at >= $1`, since).
		Scan(&activeUsers, &requests, &inT, &outT, &aiCost); err != nil {
		return nil, err
	}
	if err := db.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(SUM(amount),0) FROM credit_transaction
		  WHERE type='TOPUP' AND status='COMPLETED' AND created_at >= $1`, since).
		Scan(&revenueCount, &revenue); err != nil {
		return nil, err
	}
	if err := db.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(SUM(subtotal),0) FROM market_order WHERE created_at >= $1`, since).
		Scan(&orderCount, &orderRevenue); err != nil {
		return nil, err
	}
	var prevCost, prevRevenue float64
	if err := db.QueryRow(ctx,
		`SELECT COALESCE(SUM(cost),0) FROM usage_log WHERE created_at >= $1 AND created_at < $2`, prev, since).Scan(&prevCost); err != nil {
		return nil, err
	}
	if err := db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount),0) FROM credit_transaction
		  WHERE type='TOPUP' AND status='COMPLETED' AND created_at >= $1 AND created_at < $2`, prev, since).Scan(&prevRevenue); err != nil {
		return nil, err
	}
	signupRows, err := db.Query(ctx,
		`SELECT date_trunc('day', created_at), COUNT(*) FROM "user" WHERE created_at >= $1 GROUP BY 1 ORDER BY 1`, since)
	if err != nil {
		return nil, err
	}
	signupMap, err := dayRows(signupRows)
	if err != nil {
		return nil, err
	}
	usageRows, err := db.Query(ctx,
		`SELECT date_trunc('day', created_at), COUNT(*) FROM usage_log WHERE created_at >= $1 GROUP BY 1 ORDER BY 1`, since)
	if err != nil {
		return nil, err
	}
	usageMap, err := dayRows(usageRows)
	if err != nil {
		return nil, err
	}
	// Funnel: daftar -> top-up -> pakai API (dalam rentang).
	var topped, used int64
	if err := db.QueryRow(ctx,
		`SELECT COUNT(DISTINCT user_id) FROM credit_transaction WHERE type='TOPUP' AND status='COMPLETED' AND created_at >= $1`, since).Scan(&topped); err != nil {
		return nil, err
	}
	if err := db.QueryRow(ctx,
		`SELECT COUNT(DISTINCT user_id) FROM usage_log WHERE created_at >= $1`, since).Scan(&used); err != nil {
		return nil, err
	}
	modelRows, err := db.Query(ctx,
		`SELECT COALESCE(model,'?'), COUNT(*), COALESCE(SUM(cost),0), COALESCE(SUM(input_tokens+output_tokens),0)
		   FROM usage_log WHERE created_at >= $1 GROUP BY 1 ORDER BY 2 DESC LIMIT 5`, since)
	if err != nil {
		return nil, err
	}
	defer modelRows.Close()
	models := []map[string]any{}
	for modelRows.Next() {
		var m string
		var n int64
		var c, t float64
		if err := modelRows.Scan(&m, &n, &c, &t); err != nil {
			continue
		}
		models = append(models, map[string]any{"model": m, "requests": n, "cost": c, "tokens": t})
	}
	return map[string]any{
		"totals": map[string]any{
			"signups": signups, "activeUsers": activeUsers, "requests": requests,
			"inputTokens": inT, "outputTokens": outT, "aiCost": aiCost,
			"revenue": revenue, "revenueCount": revenueCount,
			"orderRevenue": orderRevenue, "orderCount": orderCount,
		},
		"prev":   map[string]any{"aiCost": prevCost, "revenue": prevRevenue},
		"series": map[string]any{"signups": fillDays(signupMap, since, days), "requests": fillDays(usageMap, since, days)},
		"funnel": []map[string]any{{"step": "Daftar", "count": signups}, {"step": "Top-up", "count": topped}, {"step": "Pakai API", "count": used}},
		"models": models,
	}, nil
}

func Analytics(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		since, days := analyticsRange(r)
		scope := r.URL.Query().Get("scope")
		out := map[string]any{"range": r.URL.Query().Get("range")}
		var err error
		switch scope {
		case "platform":
			role := dbRole(r.Context(), db, uid)
			if !isAdminRole(role) {
				fail(w, http.StatusForbidden, "hanya admin")
				return
			}
			out["scope"] = "platform"
			out["overview"], err = platformOverview(r.Context(), db, since, days)
		case "seller":
			s, found := loadStore(r.Context(), db, uid)
			if !found {
				fail(w, http.StatusNotFound, "toko belum terdaftar")
				return
			}
			out["scope"] = "seller"
			out["overview"], err = sellerOverview(r.Context(), db, s.ID, since, days)
		default:
			out["scope"] = "user"
			out["overview"], err = userOverview(r.Context(), db, uid, since, days)
		}
		if err != nil {
			log.Printf("[ERROR] analytics: %v", err)
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		ok(w, out)
	}
}
