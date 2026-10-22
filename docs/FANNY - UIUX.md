# FANNY - UIUX.md

Catatan design pass frontend Node2Mint Modern. Semua perubahan ada di folder `web/`,
tanpa dependency baru (tetap Tailwind v3 + komponen yang sudah ada).

Tanggal: design pass pertama oleh Fanny.

---

## 1. Design read & keputusan dasar

**Reading as:** internal SaaS dashboard (marketplace + API gateway) untuk user developer/bisnis,
dark-first, dengan brand navy + cyan yang sudah ada.

**Mode:** redesign **preserve**. Brand, struktur route, dan label navigasi dipertahankan.
Yang diubah adalah konsistensi visual, perilaku mobile, aksesibilitas, dan state feedback.

**Dials:** VARIANCE 3 (grid prediktabel, ini dashboard bukan landing page),
MOTION 2 (transisi CSS saja, tidak ada animasi scroll), DENSITY 6 (rapi, bukan cockpit).

**Kenapa bukan redesign total:** struktur info (sidebar, route, label) sudah benar.
Masalahnya bukan arah visual, tapi eksekusi: warna tidak konsisten, mobile rusak,
dan interaksi pakai dialog native browser.

---

## 2. KOREKSI: soal kode "korup" (penting dibaca)

Bagian ini awalnya menyatakan `Authorization: *** ${token}` di 3 file sebagai blocker compile.
**Itu diagnosis saya yang salah.** Setelah koreksi dari ARGUS dan pengujian ulang, yang benar:

- Yang muncul sebagai `***` di output `grep` / alat baca adalah **artefak redaksi pada lapisan
  tampilan tool**, bukan isi file. Kata `Bearer` sebenarnya ada di disk.
- Isi file diuji langsung dengan `od -c` (byte mentah) pada file contoh: file berisi
  `` `Bearer ${token}` `` lengkap, sementara `grep` menampilkannya sebagai `***`.
  Jadi `grep` tidak bisa dipakai sebagai bukti untuk pola ini.
- Sumber yang tidak teredaksi dan tetap akurat: `od -c`, `wc -c`, dan **hitungan** `grep -c`
  (yang menghitung, bukan menampilkan).

Konsekuensinya:
- Berkas yang benar-benar menyimpan header bearer hanya **dua**: `lib/api.ts` dan `lib/auth.tsx`.
  `app/dashboard/page.tsx` tidak pernah memuat header Authorization (memakai wrapper `api`).
- Frontend **sudah bisa build** sebelum saya menyentuhnya. Perbaikan saya di dua file itu
  tidak mengubah perilaku, karena isinya memang sudah benar.
- Klaim lama "FIX BY ARGUS.MD tidak sinkron dengan disk" **tidak berlaku**. Log itu benar;
  yang salah adalah cara saya membaca file.

**Pelajaran:** untuk pola yang disamarkan tool (Authorization/Bearer/token), jangan
menyimpulkan dari `grep` atau dari tampilan file. Pakai `od -c` atau `grep -c`.

---

## 3. Sistem visual

### Warna
**Sebelum:** hex hardcode tersebar (`bg-[#0b0f17]`, `text-[#e4e9f4]`, `border-[#273250]`)
walaupun token sudah ada di `tailwind.config.js` dan `globals.css`. Kartu stat dashboard
memakai 5 border berbeda sekaligus (cyan, lilac, yellow, mint, gold).

**Sesudah:**
- Semua komponen pakai token: `bg-bg`, `bg-surface`, `text-text`, `text-mute`,
  `border-border`, `text-cyan-300`, `text-danger`.
- **Satu aksen: cyan.** Warna lain hanya untuk status nyata (merah untuk error/habis,
  gold untuk peringatan kredensial). Rainbow border dihapus.
- Nilai penting (Saldo, Pendapatan) memakai `text-cyan-300` sebagai penekanan, sisanya `text-text`.

### Radius
Satu skala saja: `rounded-xl` untuk kartu, dialog, tombol, dan input.
Sebelumnya campur (`rounded-lg`, `rounded-xl`, `rounded`, `rounded-full` tanpa aturan).

### Tipografi
- Angka memakai `tabular-nums` supaya saldo dan statistik tidak bergeser saat nilainya berubah.
- Judul memakai `tracking-tight` + `font-semibold`, bukan `font-bold` seragam.
- Hierarki section dibangun dari ukuran + warna, bukan ukuran saja.

### Utility baru di `globals.css`
| Class | Fungsi |
|---|---|
| `.card` | `bg-surface border border-border rounded-xl` |
| `.label` | label kecil di atas nilai (stat & form) |
| `.skel` | skeleton loading `bg-surface-hi/70 animate-pulse` |

`tailwind.config.js`: `fontFamily.sans` diarahkan ke `var(--font-sans)` supaya font Inter
dari `next/font` benar-benar terpakai, bukan fallback sistem.

---

## 4. Perubahan per file

### `components/AppShell.tsx`
- Topbar mobile baru (hamburger + brand), tinggi 56px, `backdrop-blur`.
- Sidebar jadi off-canvas di bawah `md`: tertutup default + overlay gelap, bukan `w-64` paksa.
- Konten pakai `md:ml-64` dan `pt-14 md:pt-0` (padding mobile untuk topbar).
- **Guard login tunggal:** semua route selain `/masuk` wajib login, redirect ke `/masuk`.
- `LoadingShell` berbentuk skeleton, bukan layar kosong.

### `components/Sidebar.tsx`
- Blok brand baru (ikon + "Node2Mint") di header sidebar.
- `pt-14` mati dihapus. Dulu ada ruang kosong karena topbar yang direncanakan tidak ada.
- Drawer auto-tutup saat pindah halaman.
- `aria-current="page"` di link aktif, `aria-label` di tombol, `aria-hidden` di ikon dekoratif.
- Link admin (`/admin/providers`, `/admin/users`) tetap hanya untuk role ADMIN.

### `app/layout.tsx`
- `min-h-screen` di body. Metadata jadi `title.template` + deskripsi yang menjelaskan produk.
- Body memakai token (`bg-bg text-text`) menggantikan hex inline.

### `app/page.tsx` (root)
- Dulu selalu redirect ke `/masuk`. Sekarang: sudah login ke `/dashboard`, belum ke `/masuk`.
- Menunggu status `loading` selesai dulu supaya tidak salah arah saat verifikasi token.
- Loading memakai skeleton bulat, bukan teks "Redirect…".

### `app/masuk/page.tsx`
- Layout split: panel brand di kiri (desktop), form di kanan. Mobile hanya form + logo.
- **Mode Masuk / Daftar** dengan toggle. Sebelumnya tidak ada jalan daftar sama sekali,
  padahal endpoint `POST /api/auth/register` sudah ada di backend.
- Label nyata di atas input (`<label htmlFor>`), bukan placeholder sebagai label.
- Error jadi banner `role="alert"`, bukan teks polos.
- `autoComplete` dan `minLength` diatur untuk autofill dan validasi browser.
- Copy ditulis ulang tanpa em-dash.

### `app/dashboard/page.tsx`
- Kartu stat diseragamkan lewat satu komponen `Stat`.
- Skeleton loading untuk kartu user dan kartu admin.
- Data diambil lewat wrapper `api` (`lib/api.ts`), bukan axios manual dengan header rakitan sendiri.
- Flag `alive` untuk mencegah setState setelah komponen unmount.
- Tombol logout ganda dihapus (cukup satu, di sidebar).
- Tanggal pakai format Indonesia lengkap (`1 Januari 2026`), bukan angka mentah.

### `app/api-keys/page.tsx`
- **Revoke dua langkah inline** ("Ya, hapus" / "Batal"), menggantikan `confirm()` native.
- Feedback copy jadi ikon centang + teks "Tersalin", menggantikan `alert()`.
- Tombol copy prefix per baris dihapus (tidak ada gunanya, key mentah hanya tampil sekali).
- Dialog key mentah memakai nuansa peringatan (gold), bukan merah yang menyiratkan error.
- Skeleton list + empty state yang menjelaskan cara mengisi.
- Badge status "Aktif/Nonaktif", key nonaktif ditandai `line-through`.
- `ul`/`li` semantik, bukan `div` bertumpuk.

### `app/market/page.tsx`
- Hasil pembelian jadi **banner inline** yang hilang sendiri setelah 4 detik, menggantikan `alert()`.
- Skeleton grid saat memuat, empty state saat kosong.
- `loading="lazy"` pada gambar produk.
- Harga dan stok pakai warna status yang benar (cyan untuk tersedia, merah untuk habis).
- Tombol menampilkan "Stok Habis" saat `stock === 0`, bukan sekadar disabled tanpa alasan.

### `components/ui/dialog.tsx`
- `role="dialog"` + `aria-modal` + `aria-label`.
- Tutup dengan tombol ESC.
- Fokus otomatis ke input atau tombol pertama di dalam dialog.
- Klik luar tetap menutup, klik dalam tidak.

### `components/ui/button.tsx`
- Focus ring terlihat: `focus-visible:ring-2 ring-cyan-400/60` dengan offset.
- Feedback tekan `active:scale-[0.98]`.
- Gap default supaya ikon + teks tidak perlu dibungkus `span` manual.
- Varian tetap sama (`default`, `destructive`, `outline`, `ghost`), dipetakan ke token baru.

### `components/ui/input.tsx`
- Border, background, dan focus state dipetakan ke token.
- Placeholder lebih terang supaya lolos kontras minimum.

### `app/admin/users/page.tsx` (BARU)
- Daftar user: email, role, saldo. Pakai `GET /api/admin/users`.
- Skeleton, empty state, dan banner error.

### `app/admin/providers/page.tsx` (BARU)
- Daftar provider: label, badge aktif/nonaktif, prioritas, tipe + base URL.
- Tombol toggle memanggil `POST /api/admin/provider/toggle`.
- Dua halaman ini menutup temuan INFO-05 (link sidebar yang menuju 404).

---

## 5. Responsif

| Area | Desktop | Mobile (< 768px) |
|---|---|---|
| Navigasi | Sidebar tetap 256px | Drawer off-canvas + topbar hamburger |
| Konten utama | `md:ml-64` | Full width, `pt-14` untuk topbar |
| Grid stat | 3 kolom (4 untuk admin) | 1 kolom (2 untuk admin) |
| Grid produk | 3 kolom | 1 kolom |
| Panel brand login | Tampil, 42% lebar | Disembunyikan |
| Dialog | Center, max 448px | Center, margin 16px |

Aturan lain:
- Semua `h-screen` diganti `min-h-dvh` supaya tidak melompat saat address bar Safari iOS sembunyi.
- Layout asimetris turun jadi satu kolom penuh di bawah `md`, tidak mengandalkan "Tailwind pasti beres".

---

## 6. Aksesibilitas

- Label form nyata dengan `htmlFor`. Placeholder tidak pernah jadi satu-satunya label.
- Modal: `role="dialog"`, `aria-modal`, ESC, fokus dipindah saat dibuka.
- Focus ring terlihat di semua elemen interaktif (tombol, link, input).
- Tombol ikon punya `aria-label` (`Revoke key <nama>`, `Buka menu`, `Tutup menu`).
- Ikon dekoratif diberi `aria-hidden="true"` supaya tidak dibacakan screen reader.
- `aria-current="page"` di link navigasi aktif.
- Pesan error memakai `role="alert"`, pesan sukses memakai `role="status"`.
- Kontras: teks utama `#e4e9f4` di atas `#0b0f17`, teks sekunder `#8a94a6` untuk teks kecil
  non-kritis. Tombol cyan memakai teks gelap `#0b0f17` supaya kontras aman.

---

## 7. State lengkap (loading, kosong, error)

| Halaman | Loading | Kosong | Error |
|---|---|---|---|
| Dashboard | Skeleton kartu | - | - |
| API Keys | Skeleton list | Ikon + ajakan buat key | - |
| Marketplace | Skeleton grid | Ikon + keterangan | Banner merah |
| Admin Users | Skeleton list | "Belum ada user." | Banner merah |
| Admin Providers | Skeleton list | "Belum ada provider terdaftar." | Banner merah |
| Shell (auth) | Skeleton shell | - | Redirect ke `/masuk` |

Tidak ada lagi teks "Memuat…" polos tanpa bentuk. Skeleton dibuat menyerupai
layout akhirnya supaya tidak ada pergeseran saat data masuk.

---

## 8. Yang sengaja TIDAK diubah

- Brand color navy + cyan. Sudah khas, tinggal dipakai konsisten.
- Struktur route dan slug URL (`/masuk`, `/dashboard`, `/api-keys`, `/market`). Aman untuk SEO
  dan kebiasaan user.
- Label navigasi (Dashboard, API Keys, Marketplace).
- Font Inter. Untuk dashboard, nilai tambah ganti font tidak sebanding risikonya.
- Semua nama field form (`email`, `password`) supaya autofill dan analytics tidak rusak.
- Logika backend, hanya konsumsi endpoint yang sudah ada.

---

## 9. Verifikasi

```
npm run build                -> Compiled successfully, 8 route, 0 error
bash web/check.sh            -> SEMUA LULUS
CSS build dicek              -> bg-bg, text-mute, .skel, .card, .label, min-h-dvh semua ada
curl /masuk                  -> HTTP 200, SSR memuat "Masuk ke akun" + form nyata
curl / /dashboard /market    -> HTTP 200
```

`check.sh` menjaga 3 regresi: kode korup `***`, em-dash di copy, dan pemakaian
template literal Bearer.

Sebelum perbaikan: SSR `/masuk` hanya berisi "Menghubungkan…" karena `AuthProvider`
memblokir render semua halaman. Sesudah: konten halaman benar-benar terkirim dari server.

---

## 10. Belum dikerjakan / rekomendasi lanjutan

**Perlu verifikasi orang lain**
- Build Go gateway belum dijalankan di sini (`go` tidak ada di PATH).
  Jalankan `go vet ./... && go build ./...` di `gateway/`.

**Perlu lingkungan hidup (PostgreSQL + Redis + gateway)**
- Uji end-to-end: daftar, login, buat API key, beli produk, toggle provider.

**Rekomendasi design berikutnya**
1. Token JWT dipindah dari `localStorage` ke HttpOnly cookie. Ini perbaikan backend,
   bukan design, tapi dampaknya ke keamanan sesi.
2. Marketplace belum bisa dilihat sebelum login. Kalau mau etalase publik
   (bagus untuk SEO dan akuisisi), perlu route `/market` publik dengan CTA daftar.
3. Belum ada favicon, og:image, dan `sitemap.xml`. Untuk dashboard internal tidak wajib,
   tapi kalau nanti ada halaman publik, ini harus ditambahkan.
4. Belum ada toggle tema terang/gelap. Saat ini dark-only, dan itu keputusan yang sah
   untuk produk developer. Tambahkan hanya kalau ada permintaan nyata.
