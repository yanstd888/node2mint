# DESIGN PASS - Node2Mint Modern (frontend)

Rangkaian perbaikan UI/UX + 1 blocker compile. Semua perubahan di `web/` saja,
tanpa dependency baru (Tailwind v3 + komponen yang sudah ada).

## Blocker yang ditemukan

`Authorization: *** ${token}` masih ada di 3 file (`lib/api.ts`, `lib/auth.tsx`,
`app/dashboard/page.tsx`). Kata `Bearer` dan backtick pembuka hilang, jadi
TypeScript gagal parse. FIX BY ARGUS.MD menandai ini sebagai CRITICAL-06 FIXED,
tetapi di disk belum. Ini penyebab utama app tidak bisa build.

## Yang diubah

### Struktur & layout
- `components/AppShell.tsx`: topbar mobile (hamburger + brand), sidebar off-canvas
  di bawah `md`, guard login tunggal (route selain `/masuk` wajib login), skeleton shell.
- `components/Sidebar.tsx`: blok brand, hapus `pt-14` mati (dulu ada ruang kosong
  karena tidak ada topbar), auto-tutup drawer saat pindah halaman, `aria-current`.
- `app/page.tsx`: root route arahkan ke `/dashboard` bila sudah login, `/masuk` bila belum.
- Semua `h-screen` -> `min-h-dvh` (anti-jump di Safari iOS).

### Sistem visual
- `tailwind.config.js` + `globals.css`: token warna dipakai konsisten (`bg-bg`,
  `text-text`, `border-border`, `text-mute`), sebelumnya hex hardcode tersebar.
  Utility `.card`, `.label`, `.skel` untuk keseragaman.
- Satu aksen (cyan) + warna status saja. Rainbow border di kartu dashboard
  (cyan/lilac/yellow/mint/gold sekaligus) dihapus.
- Satu skala radius (`rounded-xl`) di semua elemen.

### Halaman
- `/masuk`: split layout (panel brand + form), mode Masuk/Daftar, label di atas
  input (`<label htmlFor>`), error jadi banner `role="alert"`.
- `/dashboard`: kartu stat seragam, skeleton loading, `tabular-nums`, data via
  wrapper `api` (bukan axios manual), tombol logout ganda dihapus.
- `/api-keys`: revoke dua-langkah inline (bukan `confirm()`), feedback "Tersalin"
  (bukan `alert()`), dialog key mentah pakai nuansa peringatan (gold, bukan merah),
  skeleton + empty state rapi. Tombol copy prefix yang tidak berguna dihapus.
- `/market`: banner hasil pembelian inline (bukan `alert()`), skeleton grid,
  empty state, `loading="lazy"` pada gambar stok.
- `/admin/users` dan `/admin/providers`: BARU. Sebelumnya link sidebar menuju 404
  (tercatat sebagai INFO-05). Provider page bisa toggle aktif/nonaktif.

### Aksesibilitas
- `components/ui/dialog.tsx`: `role="dialog"`, `aria-modal`, tutup dengan ESC,
  fokus otomatis ke elemen pertama.
- `components/ui/button.tsx`: focus ring terlihat (`focus-visible:ring-*`),
  feedback tekan `active:scale-[0.98]`.
- Ikon dekoratif diberi `aria-hidden`, tombol ikon punya `aria-label`.
- Label form pakai teks nyata, bukan placeholder sebagai label.

## Verifikasi
- `npm run build`: compile sukses, 8 route, 0 error.
- `bash web/check.sh`: cek kode korup, em-dash, template literal Bearer, lalu build.
- CSS hasil build dicek memuat semua token (`bg-bg`, `text-mute`, `.skel`, dll).
- Render SSR `/masuk` mengembalikan form nyata (sebelumnya blank karena
  AuthProvider memblokir render - sudah diperbaiki).

## Belum diverifikasi
- ~~Build Go gateway~~ **SUDAH diverifikasi**: Go 1.22.12 ada di `/c/Users/YM/go/bin` (bukan di PATH).
  Dijalankan di `gateway/`: `go vet ./...` exit 0, `go build ./...` exit 0, `go test ./...` exit 0
  (`internal/auth` ok, sisanya tanpa test). ARGUS juga mengonfirmasi hal yang sama.
- Uji interaktif di browser nyata (login -> dashboard -> API key) belum dijalankan
  end-to-end, butuh PostgreSQL + Redis + gateway hidup.

## KOREKSI setelah cross-check dengan ARGUS
Klaim awal saya soal "kode korup `***` di 3 file" SALAH. `***` adalah artefak redaksi pada
tampilan tool (grep/read), bukan isi file. Bukti: `od -c` pada file contoh menunjukkan kata
`Bearer` ada di disk sementara grep menampilkannya teredaksi. Implikasi:
- Yang punya header bearer: `lib/api.ts` dan `lib/auth.tsx` saja. `app/dashboard/page.tsx` tidak.
- Frontend sudah bisa build sebelum saya menyentuhnya.
- Guard `***` di `check.sh` sudah dihapus karena secara logika tidak bisa gagal (grep selalu
  meredaksi), diganti assertion yang benar-benar menguji template literal Bearer.

