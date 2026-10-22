package handler

// Helper bersama semua handler. Dikumpulkan di satu tempat supaya error handling
// dan penulisan audit_log tidak disalin ulang di tiap endpoint.
import (
	"context"
	"encoding/json"
	"log"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-sell/gateway/internal/apilog"
	"ai-sell/gateway/internal/auth"
	"github.com/jackc/pgx/v5"
)

// pageOf membaca ?page= & ?per_page= (default 1 & 20, max 100) dari query.
// LIMIT dibalikin per_page+1 supaya handler tahu ada halaman berikutnya
// tanpa COUNT(*) — panggilan dengan slice bertipe memotong sendiri.
func pageOf(r *http.Request) (limit, offset, page, per int) {
	per, _ = strconv.Atoi(r.URL.Query().Get("per_page"))
	if per < 1 || per > 100 {
		per = 20
	}
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	return per + 1, (page - 1) * per, page, per
}

// ok menulis JSON 200. Nilai di-marshal lebih dulu: encoder Go GAGAL untuk
// nilai yang tidak bisa di-JSON-kan (NaN/Inf), dan pada saat itu status 200
// sudah terkirim sehingga fail() tidak lagi berpengaruh — klien menerima 200
// berbadan kosong dan UI berkata "gagal memuat" tanpa sebab.
func ok(w http.ResponseWriter, v any) {
	buf, err := json.Marshal(v)
	if err != nil {
		log.Printf("[ERROR] marshal response gagal: %v", err)
		fail(w, http.StatusInternalServerError, "gagal menyusun response")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(append(buf, '\n'))
}

func fail(w http.ResponseWriter, code int, msg string) {
	auth.WriteBrowserError(w, msg, code)
}

func failCode(w http.ResponseWriter, code int, apiCode, msg string) {
	auth.WriteBrowserErrorCode(w, msg, code, apiCode)
}

// browserError preserves net/http.Error's argument order while returning the
// browser JSON envelope, so legacy handler branches share one contract too.
func browserError(w http.ResponseWriter, msg string, code int) {
	fail(w, code, msg)
}

func failV1(w http.ResponseWriter, code int, msg string) {
	auth.WriteV1Error(w, code, msg, "")
}

// V1RequestLog records middleware rejections once; successful chat handlers
// record their own provider/usage metadata in runChat.
func V1RequestLog(db DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, meta := apilog.With(r.Context())
			started := time.Now()
			next.ServeHTTP(w, r.WithContext(ctx))
			if db == nil || meta.ErrorCode == "" || apilog.Recorded(ctx) {
				return
			}
			_ = apilog.Record(ctx, db, *meta, int(time.Since(started).Milliseconds()), "fail")
		})
	}
}

// decode membaca body JSON. Angka dibiarkan mentah supaya jumlah yang dikirim
// klien tidak bisa dipercaya (mis. quantity negatif/NaN) — lihat intOf/floatOf.
func decode(r *http.Request, into any) error {
	d := json.NewDecoder(r.Body)
	d.UseNumber()
	return d.Decode(into)
}

// num adalah angka dari JSON yang bentuknya belum pasti (number atau string).
//
// Tanpa UnmarshalJSON, encoding/json menolak angka JSON ke tipe bernama ini
// ("cannot unmarshal number into Go value of type handler.num") — dan itu
// membuat SEMUA body yang memuat harga/qty/durasi ditolak sebagai "bad json".
type num json.Number

func (n *num) UnmarshalJSON(b []byte) error {
	*n = num(strings.Trim(string(b), `"`))
	return nil
}

// Int/Float menolak NaN & Inf: strconv.ParseFloat menerima "NaN"/"Infinity",
// dan NaN membuat SETIAP perbandingan rentang bernilai false — jadi nilai itu
// lolos semua validasi lalu tersimpan ke NUMERIC (Postgres menerima 'NaN' dan
// CHECK (x >= 0) juga lolos). Satu gerbang di sini menutup seluruh endpoint.
func (n num) Int(fallback int) int {
	s := strings.TrimSpace(string(n))
	i, err := strconv.Atoi(s)
	if err != nil {
		f, ferr := strconv.ParseFloat(s, 64)
		if ferr != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return fallback
		}
		i = int(f)
	}
	return i
}

func (n num) Float() float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(string(n)), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}

// RealIP menerima X-Real-IP hanya dari peer private/loopback (Caddy internal).
// Gateway produksi tidak mempublikasikan portnya, jadi client publik tidak bisa
// memalsukan header ini secara langsung.
func RealIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if peer := net.ParseIP(host); peer != nil && (peer.IsPrivate() || peer.IsLoopback()) {
		if forwarded := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); forwarded != nil {
			return forwarded.String()
		}
	}
	return host
}

func ipOf(r *http.Request) string { return RealIP(r) }

// auditTx writes the audit row on the caller's transaction. Money/admin mutations
// should use this variant before commit so the mutation and its audit cannot diverge.
func auditTx(ctx context.Context, tx pgx.Tx, userID, action, target string, before, after any, ip string) error {
	var uid any
	if userID != "" {
		uid = userID
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO audit_log (user_id, action, target, before, after, ip) VALUES ($1,$2,$3,$4,$5,$6)`,
		uid, action, target, auditJSON(before), auditJSON(after), ip)
	return err
}

// audit keeps legacy non-transactional admin/source paths visible. New money
// paths should call auditTx inside their owner transaction instead.
func audit(ctx context.Context, db DB, userID, action, target string, before, after any, ip string) {
	if _, err := db.Exec(ctx,
		`INSERT INTO audit_log (user_id, action, target, before, after, ip) VALUES ($1,$2,$3,$4,$5,$6)`,
		func() any {
			if userID != "" {
				return userID
			}
			return nil
		}(), action, target, auditJSON(before), auditJSON(after), ip); err != nil {
		log.Printf("[WARN] audit_log gagal: action=%s target=%s err=%v", action, target, err)
	}
}

func auditJSON(v any) any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return string(b)
}

// settingFloat membaca platform_setting. Nilai disimpan sebagai TEXT karena
// admin mengubahnya dari panel; yang tidak bisa dibaca pakai fallback.
func settingFloat(ctx context.Context, db DB, key string, fallback float64) float64 {
	var v string
	if err := db.QueryRow(ctx, `SELECT value FROM platform_setting WHERE key=$1`, key).Scan(&v); err != nil {
		return fallback
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return fallback
	}
	return f
}

func settingStr(ctx context.Context, db DB, key, fallback string) string {
	var v string
	if err := db.QueryRow(ctx, `SELECT value FROM platform_setting WHERE key=$1`, key).Scan(&v); err != nil {
		return fallback
	}
	return v
}

// nullStr mengubah string kosong jadi NULL. SQL NULL = "tidak ada", bukan "".
func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Peran. Dipakai di beberapa tempat (guard toko, panel admin, pencabutan peran)
// supaya daftar peran admin cuma ada di satu baris ini.
func isAdminRole(role string) bool { return role == "ADMIN" || role == "SUPER_ADMIN" }

// dbRole membaca peran user langsung dari DB. Peran di dalam JWT bisa basi
// kalau admin baru saja mengubahnya — keputusan izin selalu pakai nilai di DB.
func dbRole(ctx context.Context, db DB, userID string) string {
	var role string
	if err := db.QueryRow(ctx, `SELECT role FROM "user" WHERE id=$1`, userID).Scan(&role); err != nil {
		return ""
	}
	return role
}

// pathUUID: ambil path param {id} + validasi bentuk UUID (hex 8-4-4-4-12).
// Path rusak = 400 SEBELUM menyentuh DB — dulu pgx melempar 500 generik.
// Ponytail: cek bentuk manual, tanpa dependensi uuid; cukup karena semua id
// di DB dibuat gen_random_uuid.
func pathUUID(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	id := r.PathValue(name)
	bad := func() (string, bool) { fail(w, http.StatusBadRequest, "id tidak valid"); return "", false }
	if len(id) != 36 || id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		return bad()
	}
	for _, c := range id {
		if c != '-' && !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return bad()
		}
	}
	return id, true
}

// itoa & trimZero menghindari pengambilan strconv/fmt hanya untuk pesan galat.
func itoa(n int) string { return strconv.Itoa(n) }

func trimZero(f float64) string {
	s := strconv.FormatFloat(f, 'f', 2, 64)
	s = strings.TrimSuffix(s, ".00")
	return s
}

// familyOrder: urutan brand untuk katalog model (Anthropic → OpenAI → Google → …).
// Kolom m.family diisi saat INSERT/PATCH lewat dropdown admin; NULL = 'other'.
// Jika kolom belum ada (migrate belum jalan), fallback ke regex dari nama.
const familyOrderSQL = `CASE COALESCE(m.family,'')
  WHEN 'anthropic' THEN 1
  WHEN 'openai' THEN 2
  WHEN 'google' THEN 3
  WHEN 'deepseek' THEN 4
  WHEN 'xai' THEN 5
  WHEN 'meta' THEN 6
  WHEN 'mistralai' THEN 7
  WHEN 'moonshot' THEN 8
  WHEN 'alibaba' THEN 9
  WHEN 'zai' THEN 10
  WHEN 'minimax' THEN 11
  ELSE 100
END`
