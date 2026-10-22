package handler

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

// uptimekuma.go — Uptime Kuma (louislam/uptime-kuma) sebagai dashboard health.
//
// Kontrak minimal: Kuma jalan sebagai container (lihat docker-compose.yml) dan
// gateway jadi PUSH monitor — gateway mengukur dirinya sendiri lewat /healthz
// lalu melaporkan status/latensi ke Kuma tiap 10 menit. Kuma yang menyimpan
// riwayat, menghitung uptime, dan mengirim notifikasi.
//
// Env (di gateway/.env):
//
//	UPTIME_KUMA_URL    http://uptime-kuma:3001  (kosong/token kosong = fitur mati)
//	UPTIME_KUMA_TOKEN  push token dari monitor bertipe "Push" di UI Kuma
func StartKumaPusher(selfURL string, interval time.Duration) {
	base, token := os.Getenv("UPTIME_KUMA_URL"), os.Getenv("UPTIME_KUMA_TOKEN")
	if base == "" || token == "" {
		return // ponytail: tanpa token, Kuma tetap hidup sebagai monitor pasif
	}
	pushURL := base + "/api/push/" + url.PathEscape(token)
	client := &http.Client{Timeout: 5 * time.Second}

	go func() {
		for {
			status, msg, ping := probe(client, selfURL)
			_, _ = client.Get(pushURL + "?" + pushQuery(status, msg, ping).Encode())
			time.Sleep(interval)
		}
	}()
}

// pushQuery: parameter push Kuma (status, msg, ping).
func pushQuery(status, msg string, pingMS int) url.Values {
	return url.Values{"status": {status}, "msg": {msg}, "ping": {strconv.Itoa(pingMS)}}
}

// StartUptimeScheduler: jalankan pemeriksaan monitor berkala supaya tabel
// heartbeat (dan halaman /status) tidak menunggu klik admin. Memakai
// runUptimeChecks yang sudah ada — tanpa penjadwal baru.
func StartUptimeScheduler(db DB, interval time.Duration) {
	go func() {
		seedTask11Monitors(db)
		sendWelcomeNotification(db)
		for {
			results := runUptimeChecks(context.Background(), db) // langsung sekali, lalu berkala
			notifyUptimeFailures(db, results)
			time.Sleep(interval)
		}
	}()
}

func seedTask11Monitors(db DB) {
	for _, m := range []struct {
		name, url string
		interval  int
	}{
		{"Node2Mint website", "https://www.node2mint.com", 300},
		{"Node2Mint API models", "https://www.node2mint.com/v1/models", 300},
		{"Node2Mint status", "https://www.node2mint.com/en/status", 600},
	} {
		_, _ = db.Exec(context.Background(), `INSERT INTO uptime_monitor (name, kind, url, interval_sec)
			SELECT $1,'HTTP',$2,$3 WHERE NOT EXISTS (SELECT 1 FROM uptime_monitor WHERE name=$1)`, m.name, m.url, m.interval)
	}
}

func uptimeDownNotification(result uptimeResult) (title, message string) {
	return "Uptime monitor down", result.Name + " gagal 3 kali berturut-turut."
}

func notifyUptimeFailures(db DB, results []uptimeResult) {
	ctx := context.Background()
	for _, result := range results {
		var failures int
		if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM (SELECT status FROM uptime_heartbeat WHERE monitor_id=$1 ORDER BY checked_at DESC LIMIT 3) x WHERE status='down'`, result.MonitorID).Scan(&failures); err != nil || failures < 3 {
			continue
		}
		var already bool
		_ = db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM notifications WHERE type='SYSTEM' AND dispatch_id=$1 AND created_at > now()-interval '24 hours')`, "UPTIME_DOWN:"+result.MonitorID).Scan(&already)
		if already {
			continue
		}
		title, message := uptimeDownNotification(result)
		rows, err := db.Query(ctx, `SELECT id FROM "user" WHERE role IN ('ADMIN','SUPER_ADMIN')`)
		if err != nil {
			continue
		}
		for rows.Next() {
			var uid string
			if rows.Scan(&uid) == nil {
				insertNotification(ctx, db, uid, NotifSystem, title, message, nil, "UPTIME_DOWN:"+result.MonitorID)
			}
		}
		rows.Close()
		go sendTelegramAdmin(ctx, title+"\n"+message)
	}
}

func sendWelcomeNotification(db DB) {
	ctx := context.Background()
	rows, err := db.Query(ctx, `SELECT id FROM "user" WHERE role IN ('USER','SELLER')`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var uid string
		if rows.Scan(&uid) != nil {
			continue
		}
		_, _ = db.Exec(ctx, `INSERT INTO notifications (user_id, type, title, message, dispatch_id)
			SELECT $1,'SYSTEM',$2,$3,$4 WHERE NOT EXISTS
			(SELECT 1 FROM notifications WHERE user_id=$1 AND dispatch_id=$4)`, uid, "Selamat datang di Node2Mint", "Akun Node2Mint siap digunakan.", "SYSTEM_WELCOME_2026")
	}
}

// probe: satu pemeriksaan diri sendiri → status Kuma ("up"/"down"), pesan, latensi ms.
func probe(client *http.Client, selfURL string) (status, msg string, pingMS int) {
	status, msg = "up", "OK"
	start := time.Now()
	resp, err := client.Get(selfURL + "/healthz")
	pingMS = int(time.Since(start).Milliseconds())
	switch {
	case err != nil:
		status, msg = "down", err.Error()
	case resp.StatusCode != http.StatusOK:
		status, msg = "down", "HTTP "+strconv.Itoa(resp.StatusCode)
	}
	if resp != nil {
		resp.Body.Close()
	}
	return status, msg, pingMS
}
