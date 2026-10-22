// Bintang rating: satu bentuk untuk /toko dan /market. Setengah bintang dipakai
// (pola yang sama dengan versi Flutter): 4,5 ditampilkan 4 penuh + 1 separuh,
// supaya gambar bintang tidak bertentangan dengan angka di sebelahnya.
import { Star, StarHalf } from 'lucide-react';

export function Bintang({ n, className }: { n: number; className?: string }) {
  const nilai = Math.min(5, Math.max(0, Number(n) || 0));
  return (
    <span className={className} role="img" aria-label={`${nilai} dari 5 bintang`}>
      {[1, 2, 3, 4, 5].map((i) => {
        const kelas = `inline w-4 h-4 ${nilai >= i - 0.5 ? 'text-gold' : 'text-mute'}`;
        if (nilai >= i) return <Star key={i} aria-hidden="true" className={`${kelas} fill-gold`} />;
        if (nilai >= i - 0.5) return <StarHalf key={i} aria-hidden="true" className={`${kelas} fill-gold`} />;
        return <Star key={i} aria-hidden="true" className={kelas} />;
      })}
    </span>
  );
}
