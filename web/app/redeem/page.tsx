// Rute lama dipindah ke tab /wallet; file ini hanya supaya link lama tidak 404.
import { redirect } from 'next/navigation';

export default function Halaman() {
  redirect('/wallet?tab=redeem');
}
