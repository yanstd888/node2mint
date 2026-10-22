/** @type {import('next').NextConfig} */
// Proxy /api/* ke gateway: <img src="/api/media/…"> dan fetch relatif tidak lagi
// 404 di port dev (gambar ticker/produk/artikel pecah karena src relatif).
const API = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
const dev = process.env.NODE_ENV !== 'production';

// CSP tanpa dependency dihapus di sini (lihat middleware.ts untuk nonce).
// img-src tetap longgar karena gambar produk/ticker datang dari URL yang diisi
// admin (host tidak bisa diprediksi di muka).
// CSP pindah ke middleware.ts (nonce per request); yang tersisa di berkas ini
// hanya empat header statis + rewrite.
const nextConfig = {
  // Jangan umumkan framework + versinya di setiap respons (mempermudah pemilihan CVE).
  poweredByHeader: false,
  // Next 16: URL yang tidak cocok TIDAK memakai app/not-found.tsx melainkan
  // halaman builtin yang polos. Flag ini mengaktifkan app/global-not-found.tsx.
  experimental: { globalNotFound: true },
  async headers() {
    return [
      {
        source: '/(.*)',
        headers: [
          { key: 'X-Frame-Options', value: 'DENY' },
          { key: 'X-Content-Type-Options', value: 'nosniff' },
          { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
          { key: 'Permissions-Policy', value: 'geolocation=(), microphone=(), camera=()' },
        ],
      },
    ];
  },
  async rewrites() {
    return [
      { source: '/api/:path*', destination: `${API}/api/:path*` },
      { source: '/v1/:path*', destination: `${API}/v1/:path*` },
      { source: '/models', destination: `${API}/models` },
    ];
  },
  // /analytics sudah menyatu ke dashboard: URL lama tetap jalan lewat redirect native
  // (bukan halaman duplikat). 307 karena targetnya halaman, bukan aset statis.
  async redirects() {
    return [{ source: '/analytics', destination: '/dashboard#analytics', permanent: false }];
  },
};
export default nextConfig;
