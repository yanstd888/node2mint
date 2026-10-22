# Design System — Node2Mint

Sumber keputusan: `Restrukturisasi UIUX & Sistem Desain Platform.md` + brainstorm Fanny (sesi 20260923). Wajib dibaca sebelum menambah halaman/komponen baru.

## Tema & warna
- Dark-first, glassmorphism: panel `bg-bg/85 backdrop-blur-md`, kartu `.card` + `.lift` (hover).
- Aksen tunggal per konteks: **cyan** = produk/user, **amber** = admin (`/admin/*`). Jangan campur.
- Teks: `text-text` (utama), `text-mute` (sekunder). Angka pakai `tabular-nums`.

## Spasi — grid 4px (Fanny #7)
- Hanya kelipatan 4px: `p-1 p-2 p-3 p-4 p-6 p-8` dst.
- **Fraksi dilarang**: `p-1.5 / py-2.5 / gap-3.5 / m-0.5` — jangan dipakai di kode baru. (108 kelas lama sudah dinormalkan, 37 file.)
- `w-0.5 / h-0.5` dekoratif (bar indikator) dikecualikan.

## Badge status (Fanny #8)
Satu komponen: `components/ui/badge.tsx` — `<Badge tone="...">{status}</Badge>`.
Soft-tone: latar `-500/15` + teks `-300` + `ring-1 ring-inset ring-{c}-500/25`, uppercase.

| tone | warna | contoh status |
|------|-------|----------------|
| `ok` | emerald | PAID, UP, COMPLETED, AKTIF |
| `pending` | amber | PENDING, PROCESSING |
| `danger` | red | CANCELLED, DOWN, FAILED |
| `warn` | orange | DISPUTED, FLAGGED |
| `netral` | surface | sisanya |

Larangan: emoji sebagai ikon (pakai `lucide-react`), warna ad-hoc `text-yellow-400` dll.

## Toast (Fanny #5)
`components/ui/toast.tsx`, provider global di `app/layout.tsx`.
- `const toast = useToast(); toast.ok('Tersimpan.') / toast.err('Gagal.')`
- Posisi fixed bottom-4 right-4, lebar w-80. Sukses 3 dtk auto, error 7 dtk + tombol close.
- Stack maks 3 (FIFO), animasi 150ms, `role=alert` untuk error / `role=status` untuk sukses, hormat `prefers-reduced-motion`.
- Aturan pemakaian: feedback **aksi sesaat** → toast. Error validasi dekat field, empty state, banner permanen → tetap inline.

## Shell & navigasi (Fanny #1–#4)
- Sidebar accordion **multi-open**, chevron `rotate-90` + `aria-expanded`, animasi `grid-template-rows 0fr↔1fr` (tanpa library). Default terbuka: grup pertama (user: "Transaksi & Keuangan", admin: "Operasional & Keuangan").
- Mode admin = route `/admin/*` → shell otomatis ganti sidebar amber + guard role. Tanpa state mode global.
- Collapse: state `ui.sidebar.collapsed` (localStorage), toggle di header kiri (ikon PanelLeft, target sentuh 44px). Tablet (768–1024px) auto-collapse sekali di mount, **tidak** ditulis ke storage. Lebar `w-64 ↔ w-16`, ikon + tooltip `title` saat collapsed.
- Header atas permanen: kiri = collapse + logo (+ badge ADMIN), kanan = segmented `[EN|ID]` + avatar dropdown (Dompet, Paket, Pengaturan, Keluar). Global search sengaja tidak ada sampai endpoint-nya ada — jangan tampilkan dummy disabled.

## Dashboard bento (Fanny #6)
`grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4`; Sisa Token API hero `md:col-span-2`,
Pemakaian API `xl:col-span-2 xl:row-span-2`, Dompet 1 kolom, Toko 1 kolom,
Afiliasi 1 kolom, Pesanan Terakhir `xl:col-span-3` (menyusul kartu Afiliasi).
Prinsip: kartu bento = ringkasan + link "Kelola →" ke halaman asal; jangan menduplikasi tabel penuh.

## I18n
Semua label user-facing lewat `useT()` (`lib/i18n.tsx`), dict EN + ID. Belum semua halaman migrasi — label baru wajib masuk dict, jangan hardcode.

## Aturan verifikasi
- Dilarang `npm run build` (merusak `.next` milik dev server). Gunakan:
  `node node_modules/typescript/bin/tsc --noEmit` dari `web/` + `npm run lint`.
- Ikon hanya `lucide-react`. Tidak menambah dependency tanpa persetujuan.
