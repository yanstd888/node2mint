# Panduan Restrukturisasi UI/UX & Sistem Desain Platform

Dokumen ini berisi panduan komprehensif untuk merombak tata letak dan antarmuka (UI/UX) platform agar setara dengan standar proyek SaaS kelas *enterprise* (*funded project*). Fokus utama adalah efisiensi navigasi, responsivitas *multi-device*, serta penerapan elemen visual modern.

---

## 1. Arsitektur Navigasi & Pemisahan Lingkungan (RBAC)

Sistem menggunakan *Role-Based Access Control* (RBAC). Menu Admin disembunyikan sepenuhnya dari pandangan *User*. Navigasi dipadatkan menggunakan sistem *Accordion* (menu lipat) untuk meminimalisasi beban kognitif pada *sidebar*.

### A. Struktur Navigasi Pengguna (User / Seller)
Fokus pada akses cepat ke transaksi harian tanpa opsi teknis yang membingungkan.

*   **Akses Utama (Statis):**
    *   Dashboard
    *   Marketplace
    *   Chat
*   **Transaksi & Keuangan (Grup Accordion):**
    *   Keranjang
    *   Pesanan
    *   Dompet
    *   Paket & Harga
*   **Kemitraan & Integrasi (Grup Accordion):**
    *   Toko Saya
    *   Afiliasi
    *   API Keys
*   **Sistem & Dukungan (Statis - Bawah Sidebar):**
    *   Pusat Bantuan (Gabungan Docs, Artikel, Status Layanan)
    *   Pengaturan Akun

### B. Struktur Navigasi Admin (Workspace)
Diakses melalui lingkungan tersendiri (Mode Admin) khusus untuk visibilitas staf dan pengelolaan operasional.

*   **Overview (Statis):**
    *   Admin Dashboard (Ringkasan Metrik Utama)
*   **Operasional & Keuangan (Grup Accordion):**
    *   Pesanan (Pemantauan seluruh transaksi)
    *   Dompet (Persetujuan Top-up & Penarikan)
    *   Produk (Manajemen layanan)
*   **Sistem AI & Pengguna (Grup Accordion):**
    *   Users (Manajemen peran & modifikasi saldo)
    *   Providers (Manajemen koneksi API eksternal)
    *   Models (Pengaturan model dan *tiering*)
*   **Infrastruktur & Log (Grup Accordion):**
    *   Uptime (Status server, gateway, database)
    *   Health (Konektivitas provider)
    *   Audit Log (Perekaman aktivitas sistem)
*   **Sistem (Statis - Bawah Sidebar):**
    *   Manajemen Artikel
    *   Pengaturan Platform

---

## 2. Standar Responsivitas & Perilaku Multi-Device

Antarmuka harus beradaptasi secara mulus di berbagai ukuran layar tanpa memotong data kritis.

| Perangkat | Perilaku Sidebar | Penyesuaian Area Konten |
| :--- | :--- | :--- |
| **Desktop (> 1024px)** | Terbuka (*Expanded*) secara default. Terdapat *toggle* untuk menciutkan menjadi deretan ikon. | Menampilkan grid *Bento-box* secara penuh. Tabel data dirender melebar dengan opsi *pagination*. |
| **Tablet (768px - 1024px)**| Otomatis *Collapsed* (berupa ikon statis). Label teks muncul sebagai *tooltip* melayang saat di-*hover*. | Grid *Bento-box* menyusut menjadi 2 kolom. Ruang baca diutamakan. |
| **Mobile (< 768px)** | Disembunyikan sepenuhnya. Digantikan oleh menu *Hamburger* (*Overlay / Drawer*). | Tabel data lebar ditransformasi menjadi struktur *Card-based List* vertikal agar mudah digulir (*scroll*). |

---

## 3. Panduan Estetika Visual & Komponen (The "Funded" Look)

Penerapan detail desain mikro untuk mencapai estetika premium yang berfokus pada kebersihan ruang (*white-space*) dan fungsionalitas.

### Arsitektur Visual Inti
*   **Tata Letak Bento-Box:** Area dasbor, formulir pengaturan, dan ringkasan metrik harus dikelompokkan ke dalam struktur grid kotak-kotak (*Bento-box*) bersudut membulat (*rounded corners*) agar elemen tidak tersebar tanpa batas.
*   **Gaya Framer-Style:** Desain antarmuka dibuat minimalis dengan *line-icons* tipis, menghindari garis batas (*border*) yang tebal, dan menggunakan bayangan difusi halus (*soft drop-shadows*).
*   **Glassmorphism:** Terapkan efek kaca buram tembus pandang (*background-blur*) pada komponen yang melayang (seperti *header* navigasi atas, *dropdown*, menu laci *mobile*, atau *modal popup*) untuk menambah kedalaman ruang.
*   **Penempatan Logo:** Logo platform secara statis wajib diletakkan rata kiri (*left-aligned*) pada sudut atas antarmuka. Area *header* tengah dan kanan didedikasikan murni untuk kontrol dinamis (pencarian, bahasa, profil).

### Standarisasi UX & Komponen
*   **Sistem i18n (Multi-Bahasa):** Implementasikan arsitektur lokalisasi yang seragam. Sediakan *toggle* pemilih bahasa (EN/ID) di bagian *header*. Tidak boleh ada label teks *hardcoded* yang bercampur.
*   **Skeleton Loaders:** Gantikan animasi lingkaran berputar (*spinner*) dengan *skeleton loader* (blok kerangka dengan efek kilau bergerak) saat mengambil data dari *database* atau penyedia pihak ketiga.
*   **State Kosong (Empty States):** Pada halaman tanpa data, tampilkan ilustrasi garis (*line art*) di tengah halaman yang diikuti dengan teks instruksional yang bersahabat dan tombol *Call-to-Action* primer.
*   **Indikator Status Soft-Tone:** Pada tema gelap (*dark mode*), *badge* status (seperti "PAID", "UP", "DOWN") harus menggunakan latar belakang warna beropasitas rendah (15%) dengan teks warna solid yang serasi.
*   **Sistem Spasi Skala 8pt:** Seluruh penempatan jarak (*margin*, *padding*, dan jarak antar elemen grid) wajib mematuhi kelipatan angka 8 (8px, 16px, 24px, 32px) untuk presisi visual yang konsisten.
*   **Micro-Interactions (Umpan Balik):** Seluruh pesan konfirmasi sistem (sukses simpan, error) disampaikan melalui *Toast Notifications* yang melayang di sudut layar dan menghilang otomatis, bukan lewat kotak *alert* yang merusak alur pandangan.