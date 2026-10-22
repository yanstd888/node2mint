# 🚀 LOCALHOST UI/UX BLUEPRINT

> Spec dari user (2026-09-22). Nama brand di produk kita **Node2Mint**, bukan
> "Localhost" — semua label menyesuaikan produk. Nama model upstream (9Router)
> tetap tidak boleh terekspos.

**Visi Desain:** Premium, *Developer-Focused*, Modern, dan Aman (*High-Trust*). Perpaduan antara infrastruktur API berkinerja tinggi dan etalase produk digital kelas atas.

## 1. THE DESIGN SYSTEM (Aturan Dasar Visual)
*   **Background Utama**: *Obsidian Dark* (`#08090E`) dengan tambahan tekstur *noise* transparan yang sangat tipis agar tidak terlihat flat.
*   **Warna Aksen (Pendaran/Glow)**:
    *   🟢 **Neon Cyan / Emerald** → Untuk elemen *API, Speed, & System Status*.
    *   🟣 **Electric Violet** → Untuk elemen *Marketplace, Vault, & Premium Assets*.
*   **Tipografi**:
    *   *Heading & Body Text*: `Geist Sans` atau `Inter` (bersih dan keterbacaan tinggi).
    *   *Angka Harga, Saldo, & Label Sistem*: **WAJIB** `Geist Mono` atau `JetBrains Mono`. Memberikan kesan mahal bergaya *developer-tools*.
*   **Gaya Komponen (Glassmorphism)**: Kaca buram (`backdrop-blur`) dengan garis tepi luar biasa tipis (`border-white/10`). Hindari penggunaan bayangan pekat (*drop-shadow*), ganti dengan *inner glow* ringan.

---

## 2. ANATOMI HALAMAN (Dari Atas ke Bawah)

### A. Navigation & Header
*   **Tata Letak**: Melayang (*floating*) dan lengket (*sticky*) di bagian paling atas.
*   **Efek Visual**: Transparan murni di ujung atas, bertransisi halus menjadi *frosted glass* saat layar di-scroll ke bawah.
*   **Animasi**:
    *   Menu navigasi memiliki garis bawah tipis yang merayap (*slide-in*) saat di-*hover*.
    *   Tombol utama "Dashboard" di kanan atas memiliki efek kilau cahaya (*shimmer*) melintas setiap 5 detik.

### B. Hero Section ("The Unified Display")
*   **Kiri (Copywriting)**: Judul utama menggunakan *Gradient Text* (putih terang ke abu-abu metalik). Teks muncul dengan animasi meluncur naik bergiliran (*staggered fade-up*) berikan jeda 0.1 detik antar baris teks.
*   **Kanan (Visual Isometrik 3D)**: Dua kartu kaca bertumpuk silang.
    *   *Kartu Depan*: UI Terminal Koding dengan animasi teks mengetik otomatis (*live typing*) simulasi pemanggilan API.
    *   *Kartu Belakang (Sedikit blur)*: UI etalase produk digital.
*   **Interaksi**: Efek *Parallax Tilt*. Saat kursor bergerak di area Hero, susunan kartu akan bergoyang halus mengikuti arah kursor.

### C. Logo Ticker (Client / Supported Models)
*   **Desain**: Pita horizontal transparan memuat logo ekosistem (OpenAI, Claude, Llama, E-Wallet, QRIS).
*   **Animasi**: *Infinite Marquee* (berjalan ke kiri tanpa henti). Warna *default* abu-abu redup, menyala putih terang saat dilewati kursor.

### D. Bento Grid (The 60/40 Split Layout)
Grid asimetris untuk fitur utama, dibekali efek *Mouse-Tracking Spotlight* (cahaya pendar di *background* kotak mengikuti posisi kursor).
*   **Kotak 1 (Besar - API Speed)**: Menampilkan *sparkline* (grafik garis) Cyan bersinar yang berombak pelan (simulasi *low-latency*).
*   **Kotak 2 (Sedang - Unified Wallet)**: Menampilkan ikon koin 3D dengan angka saldo font *monospace* yang menghitung naik (*number counter animation*).
*   **Kotak 3 (Sedang - Secured Escrow)**: Ikon perisai hologram melayang dengan teks "100% Secured Escrow".

### E. Community Vault (Marketplace Highlight)
*   **Tata Letak**: *Horizontal Carousel* (geser kiri/kanan). Sisi kiri-kanan layar ditutup efek *gradient-to-transparent* agar kartu seolah muncul/hilang dari kegelapan.
*   **Desain Kartu**: Gambar sampul, judul rapi, harga font *monospace*, dan *badge* menyala `⚡ Instant` di pojok atas.
*   **Animasi Hover**: Kartu terangkat ringan (`translate-y-2`), gambar *zoom-in* 5%, dan tombol "Quick Buy" muncul dari bawah.

### F. Edukasi & Updates (Blog / Info Terbaru)
*   **Tata Letak**: 3 Kartu artikel tegak sejajar dengan garis batas super tipis dan rapi.
*   **Desain**: Thumbnail memanjang, label kategori menyala terang, judul besar, dan tanggal.
*   **Animasi Hover**: Muncul ikon tanda panah `→` yang bergerak maju-mundur secara halus.

### G. Straight Answers (FAQ Section)
*   **Tata Letak**: Terpusat di tengah (*centered-max-width*) agar fokus dibaca.
*   **Desain**: Model *Accordion* tanpa *background* kotak (hanya dipisah garis horizontal tipis). Pertanyaan di-*bold*.
*   **Animasi**: Saat diklik, ikon `+` berputar menjadi `-`, lalu panel jawaban terbuka dengan animasi *spring/bouncy reveal* (luwes dan tidak kaku).

### H. The Command Center (Footer)
*   **Background**: Lebih gelap dari seluruh halaman (`#05060A`) dibatasi garis atas tipis (`border-white/10`) sebagai tanda batas akhir *website*.
*   **4 Kolom Fungsional**:
    *   **Kolom 1 (Identitas)**: Logo, *tagline*, dan tombol *Live Status* `🟢 All Systems Operational` (bisa di-klik menuju *status page*).
    *   **Kolom 2 (API/Devs)**: Documentation, API Reference, Pricing, Supported Models.
    *   **Kolom 3 (Marketplace)**: Browse Vault, Seller Dashboard, Escrow Policy.
    *   **Kolom 4 (Legal/Support)**: Contact, ToS, Privacy, Refund Policy.
*   **Terminal-Style Bar**: Baris tipis di atas *copyright* dengan font *monospace* hijau redup bergaya log server:
    `~/ai-sell/api-relay v1.0.4 — [Latency: 12ms] — [Escrow Volume: Active]`
*   **Copyright Bar**: Sangat minimalis. `© 2026 Node2Mint` di kiri, deretan ikon *Social Media* di kanan.
