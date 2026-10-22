// Rata-rata rating. Ditaruh di lib (bukan di dalam halaman) supaya ada satu
// tempat yang bisa dijalankan sebagai pemeriksaan: `node --experimental-strip-types lib/rating.ts`
// Nilai kosong/aneh tidak boleh menjadi NaN — itu yang dulu muncul sebagai "NaN".
export function rataRating(daftar: readonly { rating?: number }[]): number {
  const nilai = daftar.map((r) => Number(r?.rating)).filter((n) => Number.isFinite(n) && n > 0);
  if (nilai.length === 0) return 0;
  const rata = nilai.reduce((s, n) => s + n, 0) / nilai.length;
  return Math.round(rata * 10) / 10;
}

// Pemeriksaan mandiri: jalankan langsung, bukan bagian dari bundel halaman.
if (typeof process !== 'undefined' && process.argv?.[1]?.endsWith('rating.ts')) {
  const eq = (lab: string, a: unknown, b: unknown) => {
    if (a !== b) throw new Error(`${lab}: ${a} !== ${b}`);
    console.log('ok  ', lab, '=', a);
  };
  eq('daftar kosong -> 0 (bukan NaN)', rataRating([]), 0);
  eq('rating hilang diabaikan', rataRating([{}, { rating: 4 }]), 4);
  eq('rata dibulatkan 1 desimal', rataRating([{ rating: 5 }, { rating: 3 }, { rating: 4 }, { rating: 5 }]), 4.3);
  eq('nilai rusak tidak jadi NaN', rataRating([{ rating: Number.NaN }, { rating: 5 }]), 5);
  console.log('SEMUA LULUS');
}
