'use client';
// Detail produk: GET /api/market/products/{id} → {product:{...}}. 404 → pesan.
import { useParams } from 'next/navigation';
import Link from 'next/link';
import { useState, useEffect } from 'react';
import { api } from '@/lib/api';
import { useFetch } from '@/lib/useApi';
import { useAuth } from '@/lib/auth';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { rupiah, tanggal, asList, productUrl, paymentUrl } from '@/lib/utils';
import { Package as PackageIcon, ArrowLeft, ShoppingCart, Minus, Plus, Store, ShieldCheck } from 'lucide-react';
import PublicHeader from '@/components/PublicHeader';
import PublicFooter from '@/components/PublicFooter';
import { Bintang } from '@/components/ui/bintang';
import { useT, useUrlPrefix } from '@/lib/i18n';
import { useToast } from '@/components/ui/toast';
import { Dialog } from '@/components/ui/dialog';
import WishlistButton from '@/components/market/WishlistButton';

type Product = {
  id: string;
  title: string;
  description?: string;
  price: number;
  stock: number;
  is_demo?: boolean;
  image_url?: string;
  store_id?: string;
  store_name?: string;
  created_at?: string;
  is_instant?: boolean;
  sales_count?: number;
  review_count?: number;
  store?: { name?: string; is_verified?: boolean; is_admin?: boolean } | null;
};

type Review = {
  id: string;
  buyer_name?: string;
  rating: number;
  comment?: string;
  created_at?: string;
};

type SellerProfile = {
  id: string;
  name: string;
  description?: string;
  logo?: string;
  products?: number;
  orders?: number;
  rating?: number;
  review_count?: number;
  is_verified?: boolean;
  is_admin?: boolean;
  product_list?: Product[];
};

export default function ProductDetail() {
  const { t } = useT();
  const prefix = useUrlPrefix();
  const toast = useToast();
  const params = useParams<{ store: string; slug: string }>();
  const slug = decodeURIComponent(String(params?.slug ?? ''));
  const fullId = slug.match(/[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i)?.[0] ?? '';
  const idPrefix = fullId ? '' : (slug.split('-').slice(-2)[0] ?? '');
  const { user } = useAuth();
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);
  const [adding, setAdding] = useState('');
  const [pesan, setPesan] = useState<{ ok: boolean; msg: string } | null>(null);
  const [qty, setQty] = useState(1);
  // Konfirmasi sebelum beli: user harus melihat jumlah & total yang akan
  // dibayar, bukan langsung dilempar ke halaman pembayaran.
  const [konfirmasi, setKonfirmasi] = useState(false);
  // Daftar tetap dipakai untuk produk terkait dan kompatibilitas URL lama ber-prefix.
  const allFetch = useFetch<{ products?: Product[] }>('/market/products');
  const allProducts = allFetch.data?.products ?? [];
  const matched = fullId ? undefined : allProducts.find((pp) => pp.id.slice(0, 6) === idPrefix);
  const productId = fullId || matched?.id || '';
  const { data, loading: dl, err } = useFetch<{ product?: Product }>(
    productId ? `/market/products/${encodeURIComponent(productId)}` : '', !productId
  );
  const loading = dl || allFetch.loading;
  const fetchErr = err || (allFetch.err && t('mkt.loadFail'));
  const p = data?.product ?? matched;
  const minK = 1;
  const maxK = p ? p.stock : 0;
  const dec = () => setQty((q) => Math.max(minK, q - 1));
  const inc = () => setQty((q) => Math.min(maxK, q + 1));
  const storeId = p?.store_id;
  const storeFetch = useFetch<{ store?: SellerProfile; products?: Product[] }>(storeId ? `/store/${storeId}` : '', !storeId);
  const seller = storeFetch.data?.store;
  const storeProducts = storeFetch.data?.products ?? [];
  const reviewsFetch = useFetch<{ reviews?: Review[]; count?: number; average?: number }>(
    productId ? `/market/reviews?product_id=${encodeURIComponent(productId)}` : '', !productId
  );
  const productReviews = reviewsFetch.data?.reviews ?? [];

  // Beli: buka konfirmasi dulu (jumlah + total), baru ke halaman pembayaran.
  // Tanpa langkah ini user terlempar ke pembayaran tanpa tahu apa yang dibeli.
  const buyNow = () => {
    if (!p || p.is_demo) return;
    if (!user) { location.assign(`/masuk?next=${encodeURIComponent(location.pathname)}`); return; }
    setKonfirmasi(true);
  };

  const lanjutBayar = () => {
    if (!p || p.is_demo) return;
    setKonfirmasi(false);
    location.assign(paymentUrl('product', p.price * qty, p.title, { id: p.id, qty }));
  };


  const addToCart = async () => {
    if (!p || p.is_demo) return;
    if (!user) { location.assign(`/masuk?next=${encodeURIComponent(location.pathname)}`); return; }
    setAdding(p.id);
    try {
      await api.post('/cart', { product_id: p.id, quantity: qty });
      window.dispatchEvent(new Event('cart:changed'));
      setPesan({ ok: true, msg: t('mkt.added') });
    } catch (e: any) {
      setPesan({ ok: false, msg: e?.response?.status === 400 ? t('mkt.noCart') : t('mkt.cartFail') });
    } finally {
      setAdding('');
    }
  };

  return (
    <div className="min-h-dvh">{mounted && !user && <PublicHeader />}<main className={`p-4 md:p-8 max-w-7xl mx-auto ${user ? '' : 'pt-24 md:pt-28'}`}>
      <Link href={`${prefix}/market`} className="inline-flex items-center gap-2 text-sm text-mute hover:text-text mb-6">
        <ArrowLeft className="w-4 h-4" aria-hidden="true" /> {t('pd.back')}
      </Link>

      {fetchErr ? (
        <div className="card p-12 text-center">
          <PackageIcon className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />
          <p role="alert" className="font-medium">{t('pd.missing')}</p>
          <p className="text-sm text-mute mt-1">{fetchErr}</p>
        </div>
      ) : loading ? (
        <div className="card p-5">
          <div className="skel w-full mb-4 rounded-xl" style={{ aspectRatio: '1/1' }} />
          <div className="skel h-6 w-2/3 mb-2" />
          <div className="skel h-4 w-full" />
        </div>
      ) : !p ? (
        <div className="card p-12 text-center">
          <PackageIcon className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />
          <p className="font-medium">{t('pd.missing')}</p>
          <p className="text-sm text-mute mt-1">{t('mkt.empty')}</p>
          <Link href={`${prefix}/market`} className="inline-block mt-4 text-sm text-cyan-300 hover:text-cyan-200">{t('pd.back')}</Link>
        </div>
      ) : (
        <article>
          <div className="grid gap-8 lg:grid-cols-12 lg:items-start">
            <div className="lg:col-span-4 lg:sticky lg:top-28">
          {p.image_url ? (
            <div className="w-full mb-6 rounded-xl border border-border overflow-hidden" style={{ aspectRatio: '1/1' }}>
              <img src={p.image_url} alt={p.title} className="w-full h-full object-cover" />
            </div>
          ) : (
            <div aria-hidden="true" className="w-full mb-6 rounded-xl border border-border overflow-hidden bg-gradient-to-br from-cyan-500/10 to-surface-hi flex items-center justify-center" style={{ aspectRatio: '1/1' }}>
              <PackageIcon className="w-12 h-12 text-mute/70" />
            </div>
          )}
            </div>

            <div className="lg:col-span-8 lg:grid lg:grid-cols-8 lg:gap-x-8">
              <div className="lg:col-span-5">
          <div className="flex flex-wrap items-center gap-3 mb-2">
            <h1 className="text-2xl font-semibold tracking-tight">{p.title}</h1>
            <Badge tone={p.is_demo ? 'warn' : p.stock > 0 ? 'ok' : 'danger'}>{p.is_demo ? t('mkt.demo') : p.stock > 0 ? t('pd.left', { n: p.stock }) : t('pd.out')}</Badge>
            <WishlistButton productId={p.id} className="ml-auto" />
          </div>
          <p className="text-xs text-mute mb-4">
            {p.store?.name ? `${p.store.name} · ` : ''}{t('pd.listed', { date: tanggal(p.created_at) })}
          </p>
          <p className="text-lg font-semibold tabular-nums text-cyan-300 mb-3">{rupiah(p.price)}</p>
          <div className="flex flex-wrap items-center gap-x-3 gap-y-2 mb-6 text-xs text-mute">
            {p.is_instant && <Badge tone="ok">{t('show.instant')}</Badge>}
            {Number(p.sales_count) > 0 && <span>{t('show.sales', { n: Number(p.sales_count) })}</span>}
            {Number(p.review_count) > 0 && <span>{t('show.reviews', { n: Number(p.review_count) })}</span>}
            {p.store?.is_verified && <span className="inline-flex items-center gap-1"><ShieldCheck className="h-3.5 w-3.5 text-cyan-400" aria-hidden="true" />{t('mkt.verified')}</span>}
          </div>

          {pesan && (
            <p role={pesan.ok ? 'status' : 'alert'}
              className={`mb-6 text-sm rounded-lg px-4 py-3 border ${pesan.ok ? 'bg-cyan-500/10 border-cyan-400/30 text-cyan-200' : 'bg-danger/10 border-danger/30 text-danger'}`}>
              {pesan.msg}
            </p>
          )}
              </div>

          {/* Qty selector + Beli Sekarang + Keranjang */}
          <aside className="mt-6 lg:mt-0 lg:col-span-3 lg:row-span-3 rounded-xl border border-border bg-surface p-4 lg:sticky lg:top-28">
            <h2 className="font-semibold mb-4">{t('pd.qty')}</h2>
          <div className="flex items-center justify-between gap-3 mb-4">
            <label htmlFor="pd-qty" className="text-xs text-mute">{t('pd.qty')}</label>
            <div className="inline-flex items-center rounded-lg border border-border bg-surface-hi text-sm">
              <button type="button" aria-label={t('pd.dec')} onClick={dec}
                disabled={qty <= minK || p.stock === 0 || p.is_demo}
                className="px-2 py-1.5 text-mute hover:text-text disabled:opacity-40 disabled:cursor-not-allowed">
                <Minus className="w-4 h-4" aria-hidden="true" />
              </button>
              <input id="pd-qty" type="number" min={minK} max={maxK} value={qty}
                onChange={(e) => setQty(Math.min(maxK, Math.max(minK, Number(e.target.value) || minK)))}
                disabled={p.stock === 0 || p.is_demo}
                className="w-12 border-0 bg-transparent text-center focus:outline-none focus:ring-0 [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none" />
              <button type="button" aria-label={t('pd.inc')} onClick={inc}
                disabled={qty >= maxK || p.stock === 0 || p.is_demo}
                className="px-2 py-1.5 text-mute hover:text-text disabled:opacity-40 disabled:cursor-not-allowed">
                <Plus className="w-4 h-4" aria-hidden="true" />
              </button>
            </div>
          </div>

          <div className="flex items-center justify-between border-t border-border pt-4 mb-4">
            <span className="text-sm text-mute">{t('cart.subtotal')}</span>
            <strong className="text-lg tabular-nums">{rupiah(p.price * qty)}</strong>
          </div>

          {p.is_demo && <p role="status" className="mb-4 text-sm text-amber-200">{t('mkt.demoNote')}</p>}

          <div className="grid gap-3">
            <Button variant="outline" disabled={adding === p.id || p.stock === 0 || p.is_demo} onClick={addToCart} className="w-full">
              <ShoppingCart className="w-4 h-4" aria-hidden="true" />
              {t('mkt.cart')}
            </Button>
            <Button disabled={p.stock === 0 || p.is_demo} onClick={buyNow} className="w-full">
              {p.is_demo ? t('mkt.demo') : p.stock === 0 ? t('mkt.out') : t('mkt.buy')}
            </Button>
          </div>
          </aside>

          <div className="mt-8 lg:col-span-5 lg:mt-6 border-t border-border pt-5">
            <h2 className="font-semibold mb-4">{t('pd.detail')}</h2>
            <div className="text-sm md:text-base text-text leading-relaxed whitespace-pre-wrap break-words">
              {p.description || t('mkt.noDesc')}
            </div>
          </div>

          {/* Seller profile section */}
          {seller && (
            <section className="mt-8 lg:col-span-5 border-t border-border pt-5">
              <div className="flex items-start gap-4">
                {seller.logo ? (
                  <img src={seller.logo} alt={seller.name} className="w-16 h-16 rounded-lg object-cover border border-border" />
                ) : (
                  <div aria-hidden="true" className="w-16 h-16 rounded-lg border border-border bg-gradient-to-br from-cyan-500/10 to-surface-hi flex items-center justify-center">
                    <Store className="w-8 h-8 text-mute/70" />
                  </div>
                )}
                <div className="flex-1">
                  <h2 className="font-semibold text-lg flex items-center gap-1.5">
                    {seller.name}
                    {seller.is_verified && <ShieldCheck aria-label={seller.is_admin ? t('mktp.adminVerified') : t('mktp.sellerVerified')} className={`h-5 w-5 ${seller.is_admin ? 'text-red-400' : 'text-yellow-300'}`} />}
                  </h2>
                  {seller.description && <p className="text-sm text-mute mt-1 line-clamp-2">{seller.description}</p>}
                  {seller.rating !== undefined && (
                    <div className="flex items-center gap-2 mt-2 text-xs">
                      <Bintang n={seller.rating} />
                      <span className="text-mute tabular-nums">{seller.rating.toFixed(1)} · {seller.review_count ?? 0} {t('mkt.reviews')}</span>
                    </div>
                  )}
                  <div className="flex gap-4 mt-2 text-xs text-mute">
                    <span>{seller.products ?? 0} {t('mkt.products')}</span>
                    <span>{seller.orders ?? 0} {t('mkt.orders')}</span>
                  </div>
                </div>
              </div>
            </section>
          )}

          <section className="mt-8 lg:col-span-5 border-t border-border pt-5" aria-labelledby="product-reviews-title">
            <div className="flex flex-wrap items-center justify-between gap-2 mb-4">
              <h2 id="product-reviews-title" className="font-semibold">{t('mkt.reviewsTitle')}</h2>
              {reviewsFetch.data?.average ? <span className="inline-flex items-center gap-2 text-sm text-mute"><Bintang n={reviewsFetch.data.average} />{reviewsFetch.data.average.toFixed(1)}</span> : null}
            </div>
            {reviewsFetch.loading ? (
              <div className="space-y-3" aria-busy="true">
                <div className="skel h-14 w-full" />
                <div className="skel h-14 w-full" />
              </div>
            ) : productReviews.length === 0 ? (
              <p className="text-sm text-mute">{t('mkt.noReviews')}</p>
            ) : (
              <ul className="space-y-3">
                {productReviews.slice(0, 5).map((review) => (
                  <li key={review.id} className="rounded-lg border border-border bg-surface-hi p-3">
                    <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
                      <span className="font-medium">{review.buyer_name || t('mkt.buyer')}</span>
                      <span className="inline-flex items-center gap-2 text-mute"><Bintang n={review.rating} />{tanggal(review.created_at)}</span>
                    </div>
                    {review.comment && <p className="mt-2 text-sm text-text whitespace-pre-wrap break-words">{review.comment}</p>}
                  </li>
                ))}
              </ul>
            )}
          </section>
            </div>
          </div>

          {/* Related products: same store, then all stores */}
          {storeProducts.length > 1 && (
            <section className="mt-10">
              <h2 className="text-lg font-semibold tracking-tight mb-4">{t('mkt.moreFromStore')}</h2>
              <ul className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-4">
                {storeProducts.filter((pp) => pp.id !== p.id).slice(0, 4).map((pp) => (
                  <li key={pp.id}>
                    <Link href={productUrl(pp.store_name, pp.title, pp.id)} className="block card p-3 hover:lift transition">
                      {pp.image_url ? (
                        <img src={pp.image_url} alt={pp.title} className="w-full aspect-square object-cover rounded-lg mb-2 border border-border" />
                      ) : (
                        <div className="w-full aspect-square rounded-lg mb-2 border border-border bg-gradient-to-br from-cyan-500/10 to-surface-hi flex items-center justify-center"><PackageIcon className="w-5 h-5 text-mute/70" /></div>
                      )}
                      <h3 className="font-medium text-sm text-text hover:text-cyan-300 line-clamp-1 mb-1">{pp.title}</h3>
                      <p className="text-cyan-300 font-semibold text-sm tabular-nums">{rupiah(pp.price)}</p>
                    </Link>
                  </li>
                ))}
              </ul>
            </section>
          )}{/* All-store related products (still inside article) */}
          <section className="mt-12">
          <h2 className="text-lg font-semibold tracking-tight mb-4">{t('mkt.moreFromAll')}</h2>
          <ul className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-4">
            {allProducts.filter((pp) => pp.store_id !== p?.store_id).slice(0, 4).map((pp) => (
              <li key={pp.id}>
                  <Link href={productUrl(pp.store_name, pp.title, pp.id)} className="block card p-3 hover:lift transition">
                  {pp.image_url ? (
                    <img src={pp.image_url} alt={pp.title} className="w-full aspect-square object-cover rounded-lg mb-2 border border-border" />
                  ) : (
                    <div className="w-full aspect-square rounded-lg mb-2 border border-border bg-gradient-to-br from-cyan-500/10 to-surface-hi flex items-center justify-center"><PackageIcon className="w-5 h-5 text-mute/70" /></div>
                  )}
                  <div className="flex justify-between items-center">
                    <h3 className="font-medium text-sm text-text hover:text-cyan-300 line-clamp-1">{pp.title}</h3>
                    <span className="text-cyan-300 font-semibold text-sm tabular-nums">{rupiah(pp.price)}</span>
                  </div>
                  {pp.store_name && <p className="text-xs text-mute mt-1 line-clamp-1">@{pp.store_name}</p>}
                </Link>
              </li>
            ))}
          </ul>
        </section>
        </article>
      )}

      {/* Konfirmasi beli: tampilkan jumlah, harga satuan, dan total sebelum
          user dibawa ke pembayaran. */}
      {p && (
        <Dialog open={konfirmasi} title={t('pd.confirmTitle')} onClose={() => setKonfirmasi(false)}>
          <div className="space-y-4">
            <p className="text-sm text-mute">{t('pd.confirmIntro')}</p>
            <dl className="rounded-lg border border-border bg-surface-hi p-4 space-y-2 text-sm">
              <div className="flex justify-between gap-4">
                <dt className="text-mute">{t('pd.confirmItem')}</dt>
                <dd className="font-medium text-right break-words">{p.title}</dd>
              </div>
              <div className="flex justify-between gap-4">
                <dt className="text-mute">{t('pd.confirmPrice')}</dt>
                <dd className="tabular-nums">{rupiah(p.price)}</dd>
              </div>
              <div className="flex justify-between gap-4">
                <dt className="text-mute">{t('pd.confirmQty')}</dt>
                <dd className="tabular-nums">{qty}</dd>
              </div>
              <div className="flex justify-between gap-4 border-t border-border pt-2">
                <dt className="font-medium">{t('pd.confirmTotal')}</dt>
                <dd className="font-semibold tabular-nums text-cyan-300">{rupiah(p.price * qty)}</dd>
              </div>
            </dl>
            {p.store?.is_verified && (
              <p className="flex items-center gap-2 text-xs text-mute">
                <ShieldCheck className="h-3.5 w-3.5 text-cyan-400" aria-hidden="true" />
                {t('pd.confirmVerified')}
              </p>
            )}
            <div className="flex justify-end gap-2">
              <Button variant="ghost" onClick={() => setKonfirmasi(false)}>{t('sel.cancel')}</Button>
              <Button variant="outline" disabled={adding === p.id} onClick={() => { setKonfirmasi(false); addToCart(); }}>
                {t('mkt.cart')}
              </Button>
              <Button onClick={lanjutBayar}>{t('pd.confirmPay')}</Button>
            </div>
          </div>
        </Dialog>
      )}
    </main>{mounted && !user && <PublicFooter />}</div>
  );
}
