import { notFound } from 'next/navigation';
import ProductDetailClient from './ProductDetailClient';
import { slug as slugify } from '@/lib/utils';

export default async function ProductDetailPage({ params }: { params: Promise<{ store: string; slug: string }> }) {
  const { store: rawStore, slug: rawSlug } = await params;
  const slug = decodeURIComponent(rawSlug);
  const id = slug.match(/[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i)?.[0];
  if (!id) notFound();
  const base = process.env.NEXT_PUBLIC_API_URL || 'http://gateway:8080';
  const response = await fetch(`${base}/api/market/products/${encodeURIComponent(id)}`, { cache: 'no-store' });
  if (!response.ok) notFound();
  // Slug toko di URL bersifat kosmetik, tapi harus cocok dengan toko asli produk
  // ini — kalau tidak, /market/<toko-apa-pun>/... mengembalikan 200 (duplicate
  // content + URL bisa mengklaim toko mana pun). Bandingkan slug, bukan nama.
  const body = await response.json().catch(() => null);
  const storeName: string | undefined = body?.product?.store?.name;
  if (slugify(decodeURIComponent(rawStore)) !== slugify(storeName)) notFound();
  return <ProductDetailClient />;
}
