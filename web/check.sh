#!/usr/bin/env bash
# Regression check UI: jebakan yang pernah lolos. Jalankan: bash check.sh
# ponytail: grep guard + build. Tidak perlu test framework.
set -u
cd "$(dirname "$0")"
fail=0

# PENTING: jangan pakai `grep` untuk MENDETEKSI pola Authorization/Bearer/token.
# Tooling di sini meredaksi pola itu di OUTPUT, jadi hasilnya selalu tampak `***` dan
# assertion apa pun soal pola tersebut tidak bisa dipercaya (selalu "lolos").
# Untuk pola sensitif pakai `grep -c` (hitungan) atau `od -c` (byte mentah).

chk_ada() { # chk_ada <label> <pattern> <file>
  local n; n=$(grep -c -- "$2" "$3" 2>/dev/null); n=${n:-0}
  if [ "$n" -gt 0 ]; then echo "ok    $1 ($n)"; else echo "FAIL  $1 (0 hit)"; fail=1; fi
}

chk_bersih() { # chk_bersih <label> <pattern> <dir...>  (abaikan baris komentar)
  local n; n=$(grep -rn -- "$2" "${@:3}" 2>/dev/null \
    | grep -vE ':[[:space:]]*(//|\{?/\*|\*)' | wc -l)
  if [ "$n" -eq 0 ]; then echo "ok    $1"; else echo "FAIL  $1 ($n hit)"; fail=1; fi
}

chk_bersih_re() { # chk_bersih_re <label> <ERE> <dir...>
  local n; n=$(grep -rnE -- "$2" "${@:3}" 2>/dev/null \
    | grep -vE ':[[:space:]]*(//|\{?/\*|\*)' | wc -l)
  if [ "$n" -eq 0 ]; then echo "ok    $1"; else echo "FAIL  $1 ($n hit)"; fail=1; fi
}

# 1. Em-dash hanya dilarang di PROSA (menyambung dua kata: `kata—kata`). `'—'` sebagai
#    penanda data kosong (StatusSection, ReferralSection, admin/users) adalah tipografi
#    yang benar, jadi tidak boleh ikut kena.
chk_bersih_re "tidak ada em-dash di prosa" '[[:alnum:]]—|—[[:alnum:]]' app components

# 2. Sesi web = cookie HttpOnly; TIDAK ada header Bearer/Authorization di klien.
#    Invariant sebenarnya: setiap panggilan axios membawa cookie (withCredentials).
#    (Guard lama memeriksa template literal Bearer; usang sejak sesi pindah ke cookie.)
chk_ada "api.ts pakai withCredentials (cookie sesi)"  'withCredentials' lib/api.ts
chk_ada "http.ts pakai withCredentials (cookie sesi)" 'withCredentials' lib/http.ts

# 3. Tidak ada dialog native browser di UI produk. Konfirmasi destruktif memakai
#    useConfirm() (Dialog bersama). Native confirm() dikenali dari argumen STRING-nya
#    (`confirm('...')` / `window.confirm("...")`), sedangkan hook selalu dipanggil dengan
#    object (`confirm({ title })`) — jadi keduanya bisa dibedakan tanpa false positive.
#    Jangan pakai '\bconfirm(' : grep di lingkungan ini menolak '\b' di depan '(' (Unmatched '(')
#    sehingga guard-nya selalu "lolos" tanpa memeriksa apa pun. Dan WAJIB chk_bersih_re:
#    chk_bersih memakai grep BRE, jadi pola beralternasi '|' akan cocok dengan nol baris
#    dan laporannya palsu "ok".
chk_bersih "tanpa alert()"   '\balert('   app components
chk_bersih_re "tanpa confirm() native" "window\.confirm\(|confirm\(['\"]" app components
chk_ada    "provider konfirmasi terpasang" 'ConfirmProvider' app/layout.tsx

# 4. Satu sumber warna status: map lokal per halaman pernah ada (STATUS_TONE di
#    pesanan) dan bikin pesanan/admin beda warna untuk status yang sama.
chk_bersih "tanpa map warna status lokal" 'STATUS_TONE' app components
chk_ada    "pesanan pakai Badge bersama"  'components/ui/badge' app/pesanan/page.tsx

# 5. Dashboard shell: a persisted desktop collapse preference must not make the
#    mobile drawer icon-only; locale-prefixed paths must still highlight active rows.
chk_ada "mobile drawer ignores desktop collapse" 'compact = collapsed && !mobileOpen' components/SidebarFrame.tsx
chk_ada "sidebar normalizes locale path" 'stripLocale(usePathname())' components/SidebarFrame.tsx

# 6. Micro-interaction: stagger tidak boleh lewat plafon 200ms (brief §motion).
#    Nilai literal apa pun >= .3s / >= 300ms pada animationDelay = pelanggaran.
chk_bersih_re "stagger <= 200ms (literal)" "animationDelay: *['\"\`]?\.?[3-9][0-9]*|animationDelay: *\`[^\`]*[3-9][0-9]*ms" app components

# 7. List masuk bergiliran: satu aturan bersama, bukan tempelan per halaman.
chk_ada "aturan list stagger bersama" '.stagger > \*' app/globals.css
chk_ada "keyframes li-in ada"                  'keyframes li-in'          app/globals.css
chk_ada "feedback tekan tombol global"         'button:not(:disabled)'    app/globals.css
# 8. Animasi masuk WAJIB 'backwards'. 'both' menahan transform:none selamanya dan
#    mematikan setiap hover-lift / utility transform pada elemen itu (dibuktikan di Chrome).
chk_bersih_re "animasi masuk tidak pakai fill 'both'" "animation: *(li-in|fade-up|toast-in)[^;}]*both" app/globals.css

# 9. Empty state palsu: useFetch tanpa cabang loading pernah tampil "belum ada data"
#    padahal masih memuat. Hanya file yang MEMANG punya cabang empty-state yang wajib
#    menyebut loading; komponen yang menyembunyikan diri saat kosong (mis. footer) bebas.
chk_lupa=""
for f in $(grep -rl 'useFetch<' app components --include='*.tsx'); do
  grep -qE 'length === 0|Belum |Tidak ada' "$f" || continue
  grep -q 'loading' "$f" || chk_lupa="$chk_lupa $f"
done
if [ -z "$chk_lupa" ]; then echo "ok    cabang empty-state selalu dijaga loading"
else echo "FAIL  empty-state tanpa cabang loading:$chk_lupa"; fail=1; fi

# 10. Katalog model wajib ada di HTML pertama (crawler tidak menjalankan JS).
#    app/page.tsx yang kembali jadi client component = SEO P0 ini hilang tanpa
#    error build apa pun, jadi dipatok di sini: server component + ambil katalog.
if grep -q "'use client'" app/page.tsx; then
  echo "FAIL  homepage kembali client component (katalog hilang dari HTML mentah)"; fail=1
else echo "ok    homepage server component (tanpa 'use client')"; fi
chk_ada "homepage ambil katalog di server" 'getPublicHomeData' app/page.tsx
chk_ada "hero pakai initialModels"       'initialModels'   components/landing/HomeHero.tsx

if [ "$fail" = 0 ]; then
  echo "--- build ---"
  npm run build 2>&1 | grep -E "Compiled successfully|Failed to compile" || fail=1
fi
[ "$fail" = 0 ] && echo "SEMUA LULUS" || { echo "ADA YANG GAGAL"; exit 1; }
