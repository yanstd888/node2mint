// Cek mandiri logika biaya boost & sisa masa (jalan: node --experimental-strip-types web/lib/boost-cost.ts).
// Ini jalur uang: kalau biaya di popup beda dari yang dipotong server,
// user dirugikan. Rumus di sini harus sama dengan shop.go BoostProduct.
export const biayaBoost = (hari: number, perHari: number) => perHari * hari;

// Same as shop.go: one-off selalu 1 hari, harga flat.
export const biayaOneOff = (oneOffPrice: number) => oneOffPrice;

export const sisaHari = (iso?: string, now = Date.now()) => {
  if (!iso) return 0;
  const ms = new Date(iso).getTime() - now;
  return ms <= 0 ? 0 : Math.ceil(ms / 86_400_000);
};

export function demo() {
  // 7 hari × Rp5.000 = Rp35.000 (default backend)
  if (biayaBoost(7, 5000) !== 35000) throw new Error('biaya 7 hari salah');
  if (biayaBoost(1, 5000) !== 5000) throw new Error('biaya 1 hari salah');
  if (biayaBoost(90, 5000) !== 450000) throw new Error('biaya 90 hari salah');
  if (biayaOneOff(1000) !== 1000) throw new Error('one-off salah');
  // sisaHari: pembulatan ke atas supaya "<1 hari" tidak jadi 0.
  const now = Date.UTC(2026, 0, 1, 0, 0, 0);
  if (sisaHari(new Date(now + 36e5).toISOString(), now) !== 1) throw new Error('sisa 1 jam harus 1 hari');
  if (sisaHari(new Date(now + 7 * 864e5).toISOString(), now) !== 7) throw new Error('sisa 7 hari salah');
  if (sisaHari(new Date(now - 1000).toISOString(), now) !== 0) throw new Error('masa lalu harus 0');
  if (sisaHari(undefined, now) !== 0) throw new Error('undefined harus 0');
  console.log('boost-cost: OK');
}

if (typeof process !== 'undefined' && process.argv[1]?.includes('boost-cost')) demo();
