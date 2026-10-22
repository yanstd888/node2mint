// Katalog homepage pada HTML pertama: crawler harus melihat nama model +
// context window tanpa menjalankan JS.
// Endpoint = sama dengan rewrite next.config.ts / proxy.ts (NEXT_PUBLIC_API_URL →
// gateway), bukan origin web sendiri: di produksi Caddy membelokkan /api/* terus
// ke gateway, jadi fetch ke origin sendiri hanya memutar balik lewat Caddy.
//
// ponytail: cache dalam proses 60 detik. Layout memakai dynamic='force-dynamic',
// jadi cache fetch Next tidak berlaku; satu variabel modul lebih pasti dan
// mencegah tiap render menghajar bucket rate-limit gateway yang sama.
// Naikkan ke revalidateTag saat simpan admin model perlu buka katalog segera.
const API = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
const TTL_MS = 60_000;
let jam = 0;
let isi: unknown | null = null;

export async function getHomeCatalog(): Promise<unknown | null> {
  if (Date.now() - jam < TTL_MS) return isi;
  try {
    const res = await fetch(`${API}/api/models`, { signal: AbortSignal.timeout(3000) });
    if (res.ok) isi = await res.json();
  } catch {
    // Gateway mati: halaman tetap dirender, fetch klien di HomeHero mengambil alih.
  }
  jam = Date.now();
  return isi;
}
