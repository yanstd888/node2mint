'use client';
// Marketplace listing: grid 4 kolom, image square, klik seluruh card.
// Loading state renders skeleton grid; error and empty states are honest and actionable.
import { useEffect, useState, Suspense } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { api } from '@/lib/api';
import { useFetch } from '@/lib/useApi';
import { Badge } from '@/components/ui/badge';
import { Bintang } from '@/components/ui/bintang';
import { Button } from '@/components/ui/button';
import { Dialog } from '@/components/ui/dialog';
import { Select } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { Pagination } from '@/components/ui/Pagination';
import Link from 'next/link';
import { useAuth } from '@/lib/auth';
import { Store, Package as PackageIcon, Search, Gem, ShoppingCart, Laptop, BookOpen, Folder, ShieldCheck, SlidersHorizontal, ArrowDownUp, BadgeCheck, Zap, ChevronDown } from 'lucide-react';
import PublicHeader from '@/components/PublicHeader';
import PublicFooter from '@/components/PublicFooter';
import { MarketplaceBanner } from '@/components/market/MarketplaceBanner';
import { useT, type Key } from '@/lib/i18n';
import { rupiah, angka, asList, productUrl } from '@/lib/utils';
import { rataRating } from '@/lib/rating';

type Product = {
  id: string;
  title: string;
  description: string;
  price: number;
  stock: number;
  is_demo?: boolean;
  is_instant?: boolean;
  image_url?: string;
  store_id?: string;
  store_name?: string;
  store_verified?: boolean;
  store_admin?: boolean;
  sales_count?: number;
  review_count?: number;
};

type Ulasan = {
  id: string;
  product_id?: string;
  store_id?: string;
  buyer_name?: string;
  rating?: number;
  comment?: string;
  created_at?: string;
};

type Banner = { type: 'ok' | 'err'; msg: string };

function KatIkon({ slug }: { slug: string }) {
  const cls = 'h-3.5 w-3.5 shrink-0';
  if (slug === 'soul-skill') return <Gem className={cls} aria-hidden="true" />;
  if (slug === 'premiums-app') return <ShoppingCart className={cls} aria-hidden="true" />;
  if (slug === 'akun') return <Store className={cls} aria-hidden="true" />;
  if (slug === 'tools-software') return <Laptop className={cls} aria-hidden="true" />;
  if (slug === 'ebook') return <BookOpen className={cls} aria-hidden="true" />;
  return <Folder className={cls} aria-hidden="true" />;
}

type Kategori = { id: string; name: string; slug: string; icon: string; products: number };

const MARKET_INFO: [Key, Key][] = [
  ['mkt.infoVerifiedH', 'mkt.infoVerifiedP'],
  ['mkt.infoFeeH', 'mkt.infoFeeP'],
  ['mkt.infoPayoutH', 'mkt.infoPayoutP'],
];

const CATEGORY_LABEL: Record<string, Key> = {
  'premiums-app': 'mkt.catPremiumsApp',
  'soul-skill': 'mkt.catSoulSkill',
  'tools-software': 'mkt.catToolsSoftware',
  ebook: 'mkt.catEbook',
  akun: 'mkt.catAccounts',
  others: 'mkt.catOthers',
};

export default function MarketPage() {
  const { user } = useAuth();
  return (
    <Suspense fallback={<div className="flex min-h-dvh flex-col">{!user && <PublicHeader />}<main className={`mx-auto w-full max-w-6xl flex-1 p-6 ${user ? 'md:p-10' : 'pt-24'}`}><div className="skel h-10 w-full" /></main></div>}>
      <MarketBody />
    </Suspense>
  );
}

function MarketBody() {
  const { user } = useAuth();
  const { t } = useT();
  const labelKategori = (category: Kategori) => {
    const key = CATEGORY_LABEL[category.slug];
    return key ? t(key) : category.name;
  };
  const router = useRouter();
  const sp = useSearchParams();
  const q = sp.get('q') ?? '';
  const category = sp.get('category') ?? '';
  const verified = sp.get('verified') === 'true';
  const minPrice = sp.get('min_price') ?? '';
  const maxPrice = sp.get('max_price') ?? '';
  const instant = sp.get('instant') === 'true';
  const available = sp.get('available') === 'true';
  const sortParam = sp.get('sort') === 'best_selling' || sp.get('sort') === 'price_low' || sp.get('sort') === 'price_high' ? sp.get('sort') as 'best_selling' | 'price_low' | 'price_high' : 'newest';
  const hasFilters = Boolean(q || category || verified || minPrice || maxPrice || instant || available || sortParam !== 'newest');
  const [draft, setDraft] = useState(q);
  const [minDraft, setMinDraft] = useState(minPrice);
  const [maxDraft, setMaxDraft] = useState(maxPrice);
  const [instantDraft, setInstantDraft] = useState(instant);
  const [availableDraft, setAvailableDraft] = useState(available);
  const [categories, setCategories] = useState<Kategori[]>([]);
  const [products, setProducts] = useState<Product[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState('');
  const [banner, setBanner] = useState<Banner | null>(null);
  const [dialog, setDialog] = useState<{ id: string; title: string; storeId: string } | null>(null);
  const [rating, setRating] = useState(5);
  const [komentar, setKomentar] = useState('');
  const [kirimBusy, setKirimBusy] = useState(false);
  const [sheet, setSheet] = useState(false);
  const [sort, setSort] = useState<'newest' | 'best_selling' | 'price_low' | 'price_high'>(sortParam);

  const ulasan = useFetch<unknown>('/market/reviews');
  const semuaUlasan = asList<Ulasan>(ulasan.data, 'reviews');
  const ulasanProduk = (pid: string) => semuaUlasan.filter((r) => r.product_id === pid);
  const rataProduk = (pid: string) => rataRating(ulasanProduk(pid));
  const tampil = [...products].sort((a, b) => sort === 'price_low' ? a.price - b.price : sort === 'price_high' ? b.price - a.price : 0);

  useEffect(() => {
    setDraft(q);
    setMinDraft(minPrice);
    setMaxDraft(maxPrice);
    setInstantDraft(instant);
    setAvailableDraft(available);
    setSort(sortParam);
  }, [q, minPrice, maxPrice, instant, available, sortParam]);

  useEffect(() => {
    let alive = true;
    api.get<{ categories: Kategori[] }>('/market/categories')
      .then(({ data }) => { if (alive) setCategories(data.categories || []); })
      .catch(() => {});
    return () => { alive = false; };
  }, []);

  const setFilter = (next: { q?: string; category?: string; verified?: boolean; min_price?: string; max_price?: string; instant?: boolean; available?: boolean; sort?: typeof sort }) => {
    const p = new URLSearchParams();
    const nq = next.q ?? q;
    const nc = next.category ?? category;
    const nv = next.verified ?? verified;
    const nmin = next.min_price ?? minPrice;
    const nmax = next.max_price ?? maxPrice;
    const ninstant = next.instant ?? instant;
    const navailable = next.available ?? available;
    const nsort = next.sort ?? sortParam;
    if (nq) p.set('q', nq);
    if (nc) p.set('category', nc);
    if (nv) p.set('verified', 'true');
    if (nmin) p.set('min_price', nmin);
    if (nmax) p.set('max_price', nmax);
    if (ninstant) p.set('instant', 'true');
    if (navailable) p.set('available', 'true');
    if (nsort !== 'newest') p.set('sort', nsort);
    const s = p.toString();
    router.replace(s ? `/market?${s}` : '/market', { scroll: false });
    setPage(1);
  };
  const resetFilters = () => setFilter({ q: '', category: '', verified: false, min_price: '', max_price: '', instant: false, available: false, sort: 'newest' });

  useEffect(() => {
    let alive = true;
    setLoading(true);
    const p = new URLSearchParams({ page: String(page) });
    if (q) p.set('q', q);
    if (category) p.set('category', category);
    if (verified) p.set('verified', 'true');
    if (minPrice) p.set('min_price', minPrice);
    if (maxPrice) p.set('max_price', maxPrice);
    if (instant) p.set('instant', 'true');
    if (available) p.set('available', 'true');
    if (sortParam !== 'newest') p.set('sort', sortParam);
    (async () => {
      try {
        const { data } = await api.get<{ products: Product[]; has_more?: boolean }>(`/market/products?${p}`);
        if (alive) {
          setProducts(data.products || []);
          setHasMore(!!data.has_more);
          setErr('');
        }
      } catch {
        if (alive) setErr(t('mkt.loadFail'));
      } finally {
        if (alive) setLoading(false);
      }
    })();
    return () => { alive = false; };
  }, [page, q, category, verified, t]);

  useEffect(() => {
    if (!banner) return;
    const timer = setTimeout(() => setBanner(null), 4000);
    return () => clearTimeout(timer);
  }, [banner]);

  const butuhLogin = () => {
    if (user) return false;
    const back = window.location.pathname + window.location.search;
    router.push(`/masuk?next=${encodeURIComponent(back)}`);
    return true;
  };

  const kirimUlasan = async () => {
    if (!dialog) return;
    setKirimBusy(true);
    try {
      await api.post('/market/reviews', {
        store_id: dialog.storeId,
        product_id: dialog.id,
        rating,
        comment: komentar.trim(),
      });
      setBanner({ type: 'ok', msg: t('mkt.reviewed') });
      setDialog(null);
      await ulasan.reload();
    } catch {
      setBanner({ type: 'err', msg: t('mkt.reviewFail') });
    } finally {
      setKirimBusy(false);
    }
  };

  return (
    <div className="flex min-h-dvh flex-col">{!user && <PublicHeader />}<main className={`mx-auto w-full max-w-6xl flex-1 p-6 md:p-10 ${user ? '' : 'pt-24 md:pt-28'}`}>
      <header className="mb-8 grid gap-4 lg:grid-cols-[auto_minmax(18rem,1fr)_auto] lg:items-end">
        <div>
          <p className="label mb-1">{t('mkt.kicker')}</p>
          <h1 className="text-2xl font-semibold tracking-tight">{t('mkt.h')}</h1>
        </div>
        <form role="search" onSubmit={(e) => { e.preventDefault(); setFilter({ q: draft.trim() }); }}>
          <label htmlFor="mkt-q" className="sr-only">{t('mkt.search')}</label>
          <div className="relative">
          <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-mute" aria-hidden="true" />
          <input id="mkt-q" value={draft} onChange={(e) => setDraft(e.target.value)} placeholder={t('mkt.searchPh')}
            className="flex h-10 w-full rounded-xl border border-white/15 bg-surface-hi pl-9 pr-3 text-sm text-white placeholder-slate-400 transition-colors focus:border-cyan-300/70 focus:outline-none focus-visible:ring-2 focus-visible:ring-cyan-300/50" />
          </div>
        </form>
        <Link href="/toko?tab=produk" className="inline-flex h-10 items-center justify-center rounded-xl bg-cyan-300 px-4 text-sm font-semibold text-slate-950 transition-colors hover:bg-cyan-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-300 focus-visible:ring-offset-2 focus-visible:ring-offset-surface">{t('mkt.sellCta')}</Link>
      </header>

      <MarketplaceBanner />

      <div className="mb-8" role="group" aria-label={t('mkt.cats')}>
        <div className="mb-3 flex justify-start">
          <Button variant="outline" size="sm" onClick={() => setSheet(true)}><SlidersHorizontal className="h-4 w-4" />{t('mkt.filter')}</Button>
        </div>
        <div className="-mx-6 flex gap-2 overflow-x-auto px-6 pb-2 [scrollbar-width:none] md:mx-0 md:px-0" style={{ WebkitOverflowScrolling: 'touch' }}>
        <button type="button" aria-pressed={!category && !verified} onClick={() => setFilter({ category: '', verified: false })}
          className={`inline-flex shrink-0 items-center rounded-full border px-3 py-1 text-sm transition-colors ${!category && !verified ? 'border-cyan-300/50 bg-cyan-300/10 text-cyan-100' : 'border-border text-mute hover:border-white/20 hover:text-text'}`}>
          {t('mkt.forYou')}
        </button>
        <button type="button" aria-pressed={verified} onClick={() => setFilter({ category: '', verified: true })}
          className={`inline-flex shrink-0 items-center gap-1.5 rounded-full border px-3 py-1 text-sm font-semibold transition-colors ${verified ? 'border-violet-300/70 bg-violet-500/20 text-violet-100 shadow-[0_0_18px_rgba(139,92,246,.22)]' : 'border-violet-400/45 bg-violet-500/10 text-violet-200 hover:bg-violet-500/20'}`}>
          <BadgeCheck className="h-4 w-4 fill-violet-500 text-white" aria-hidden="true" /> {t('mkt.verified')}
        </button>
        {categories.map((c) => (
          <button key={c.id} type="button" aria-pressed={category === c.slug || category === c.id}
            onClick={() => setFilter({ category: c.slug, verified: false })}
            className={`inline-flex shrink-0 items-center gap-1.5 rounded-full border px-3 py-1 text-sm transition-colors ${(category === c.slug || category === c.id) ? 'border-cyan-300/50 bg-cyan-300/10 text-cyan-100' : 'border-border text-mute hover:border-white/20 hover:text-text'}`}>
            <KatIkon slug={c.slug} />
            <span>{labelKategori(c)}</span>
            <span className="rounded-full bg-white/[.06] px-1.5 font-mono text-[11px] tabular-nums text-mute">{c.products}</span>
          </button>
        ))}
        {hasFilters && (
          <button type="button" onClick={resetFilters} className="shrink-0 text-sm text-cyan-300 underline-offset-2 hover:text-cyan-200 hover:underline">
            {t('mkt.reset')}
          </button>
        )}
        </div>
      </div>

      {banner && (
        <div role={banner.type === 'err' ? 'alert' : 'status'}
          className={`mb-6 rounded-lg border px-4 py-3 text-sm ${banner.type === 'ok' ? 'bg-cyan-500/10 border-cyan-400/30 text-cyan-200' : 'bg-danger/10 border-danger/30 text-danger'}`}>
          {banner.msg}
        </div>
      )}

      {err && (
        <p role="alert" className="mb-6 text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">
          {t('err.load')}
        </p>
      )}

      {err ? null : loading ? (
        <div className="grid grid-cols-2 gap-3 md:grid-cols-2 md:gap-6 lg:grid-cols-3 xl:grid-cols-4">
          {[0, 1, 2, 3].map((i) => (
            <div key={i} className="card p-3 md:p-5">
              <div className="skel w-full mb-4 rounded-lg" style={{ aspectRatio: '1/1' }} />
              <div className="skel h-4 w-2/3 mb-2" />
              <div className="skel h-3 w-full mb-4" />
              <div className="skel h-4 w-1/3 mb-2" />
              <div className="skel h-9 w-full" />
            </div>
          ))}
        </div>
      ) : products.length === 0 ? (
        <div className="card p-12 text-center">
          <Store className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />
          <p className="font-medium">{hasFilters ? t('mkt.noMatch') : t('mkt.empty')}</p>
          <p className="text-sm text-mute mt-1">{hasFilters ? t('mkt.noMatchP') : t('mkt.emptyNoDemoP')}</p>
          {hasFilters && (
            <button type="button" onClick={resetFilters} className="mt-5 text-sm text-cyan-300 hover:text-cyan-200">
              {t('mkt.reset')}
            </button>
          )}
        </div>
      ) : (
        <ul className="grid grid-cols-2 gap-3 md:grid-cols-2 md:gap-6 lg:grid-cols-3 xl:grid-cols-4">
          {tampil.map((p) => (
            <li key={p.id} className="card lift flex flex-col p-3 md:p-5">
              <Link href={productUrl(p.store_name, p.title, p.id)} className="block">
                <div className="relative mb-3 w-full overflow-hidden rounded-lg border border-border md:mb-4" style={{ aspectRatio: '1/1' }}>
                  <Badge className={`absolute left-2 top-2 z-10 shrink-0 whitespace-nowrap !text-[10px] !text-white shadow-lg shadow-black/50 md:left-3 md:top-3 md:!text-xs ${p.is_demo ? '!bg-amber-700 !ring-amber-300/70' : p.stock > 0 ? '!bg-emerald-700 !ring-emerald-300/70' : '!bg-red-700 !ring-red-300/70'}`} tone={p.is_demo ? 'warn' : p.stock > 0 ? 'ok' : 'danger'}>
                    {p.is_demo ? t('mkt.demo') : p.stock > 0 ? t('mkt.inStock', { n: p.stock }) : t('mkt.soldOut')}
                  </Badge>
                  {p.image_url ? (
                    <img src={p.image_url} alt={p.title} loading="lazy" className="w-full h-full object-cover" />
                  ) : (
                    <div aria-hidden="true" className="w-full h-full bg-gradient-to-br from-cyan-500/10 to-surface-hi flex items-center justify-center">
                      <PackageIcon className="w-8 h-8 text-mute/70" />
                    </div>
                  )}
                </div>
                <h2 className="mb-1 line-clamp-1 text-sm font-semibold text-text transition-colors hover:text-cyan-300 md:text-base">
                  {p.title}
                </h2>
                {p.store_name && <p className="mb-2 flex truncate text-xs text-mute">
                  <span className="truncate">{p.store_name}</span>
                  {p.store_verified && <ShieldCheck aria-label={p.store_admin ? t('mktp.adminVerified') : t('mktp.sellerVerified')} className={`h-3.5 w-3.5 shrink-0 ${p.store_admin ? 'text-red-400' : 'text-yellow-300'}`} />}
                </p>}
                <p className="mb-3 hidden flex-1 text-sm leading-relaxed text-mute sm:mb-4 sm:line-clamp-2 sm:block">
                  {p.description || t('mkt.noDesc')}
                </p>
                <div className="mb-3 flex min-h-5 flex-wrap items-center gap-2">
                  {p.is_instant && <span className="inline-flex items-center gap-1 text-[11px] text-violet-200"><Zap className="h-3 w-3" aria-hidden="true" />{t('show.instant')}</span>}
                  {p.sales_count ? <span className="text-[11px] text-mute">{t('show.sales', { n: angka(p.sales_count) })}</span> : null}
                  {p.review_count ? <span className="text-[11px] text-mute">{t('show.reviews', { n: angka(p.review_count) })}</span> : null}
                </div>
              </Link>
              <div className="mt-auto flex items-center justify-between gap-2">
                <span className="text-base font-semibold tabular-nums text-cyan-300 md:text-lg">{rupiah(p.price)}</span>
                {user && p.store_id && (
                  <Button size="sm" variant="outline" onClick={() => setDialog({ id: p.id, title: p.title, storeId: p.store_id || '' })}>
                    {t('mkt.review')}
                  </Button>
                )}
              </div>
              {ulasanProduk(p.id).length > 0 && (
                <p className="mb-4 hidden text-xs tabular-nums text-mute sm:block">
                  <Bintang n={rataProduk(p.id)} /> {angka(rataProduk(p.id))} ({t('mkt.reviews', { n: angka(ulasanProduk(p.id).length) })})
                </p>
              )}
            </li>
          ))}
        </ul>
      )}

      {!err && !loading && products.length > 0 && (
        <Pagination page={page} hasMore={hasMore} onPage={setPage} />
      )}

      <section aria-labelledby="market-info-title" className="mt-8 mb-8">
        <h2 id="market-info-title" className="sr-only">{t('mkt.infoTitle')}</h2>
        <div className="space-y-3">
          {MARKET_INFO.map(([heading, body]) => (
            <details key={heading} className="group rounded-xl border border-border bg-surface text-sm leading-6 text-mute">
              <summary className="flex cursor-pointer list-none items-center justify-between gap-4 px-5 py-4 font-semibold text-text marker:hidden focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-cyan-300 [&::-webkit-details-marker]:hidden">
                <span>{t(heading)}</span>
                <ChevronDown className="h-4 w-4 shrink-0 text-mute transition-transform duration-200 group-open:rotate-180" aria-hidden="true" />
              </summary>
              <p className="border-t border-border px-5 pb-5 pt-4">{t(body)}</p>
            </details>
          ))}
        </div>
      </section>

      {sheet && <div className="fixed inset-0 z-50 bg-black/60" role="presentation" onClick={() => setSheet(false)}>
        <section role="dialog" aria-modal="true" aria-labelledby="mkt-filter-title" className="absolute inset-x-0 bottom-0 rounded-t-2xl border border-border bg-surface p-6" onClick={(e) => e.stopPropagation()}>
          <div className="mx-auto max-w-2xl"><div className="mb-5 flex items-center justify-between"><h2 id="mkt-filter-title" className="font-semibold">{t('mkt.filter')}</h2><button type="button" onClick={() => setSheet(false)} aria-label={t('mkt.close')}>×</button></div>
            <div className="grid grid-cols-2 gap-3">
              <label className="space-y-2"><span className="label">{t('mkt.priceMin')}</span><input value={minDraft} onChange={(e) => setMinDraft(e.target.value.replace(/[^0-9]/g, ''))} inputMode="numeric" className="input" placeholder="0" /></label>
              <label className="space-y-2"><span className="label">{t('mkt.priceMax')}</span><input value={maxDraft} onChange={(e) => setMaxDraft(e.target.value.replace(/[^0-9]/g, ''))} inputMode="numeric" className="input" placeholder="∞" /></label>
            </div>
            <div className="mt-4 flex flex-wrap gap-4 text-sm">
              <label className="inline-flex items-center gap-2"><input type="checkbox" checked={instantDraft} onChange={(e) => setInstantDraft(e.target.checked)} className="accent-cyan-400" />{t('mkt.instantOnly')}</label>
              <label className="inline-flex items-center gap-2"><input type="checkbox" checked={availableDraft} onChange={(e) => setAvailableDraft(e.target.checked)} className="accent-cyan-400" />{t('mkt.availableOnly')}</label>
            </div>
            <label className="label mb-2 mt-5 block" htmlFor="mkt-sort"><ArrowDownUp className="mr-1 inline h-4 w-4" />{t('mkt.sort')}</label>
            <select id="mkt-sort" value={sort} onChange={(e) => setSort(e.target.value as typeof sort)} className="h-10 w-full rounded-xl border border-border bg-surface-hi px-3 text-sm">
              <option value="newest">{t('mkt.sortNewest')}</option><option value="best_selling">{t('mkt.sortBest')}</option><option value="price_low">{t('mkt.sortLow')}</option><option value="price_high">{t('mkt.sortHigh')}</option>
            </select><Button className="mt-5 w-full" onClick={() => { setFilter({ min_price: minDraft, max_price: maxDraft, instant: instantDraft, available: availableDraft, sort }); setSheet(false); }}>{t('mkt.apply')}</Button>
          </div>
        </section>
      </div>}

      <Dialog open={!!dialog} title={t('mkt.reviewOf', { name: dialog?.title ?? '' })} onClose={() => setDialog(null)}>
        <div className="space-y-5">
          <p className="text-xs text-mute">{t('mkt.reviewNote')}</p>
          <div className="space-y-2">
            <label htmlFor="r-bintang" className="label">{t('mkt.rating')}</label>
            <Select id="r-bintang" value={String(rating)} onChange={(e) => setRating(Number(e.target.value))}>
              {[5, 4, 3, 2, 1].map((n) => (
                <option key={n} value={n}>{t('mkt.stars', { n })}</option>
              ))}
            </Select>
            <Bintang n={rating} className="block mt-2" />
          </div>
          <div className="space-y-2">
            <label htmlFor="r-komentar" className="label">{t('mkt.comment')}</label>
            <Textarea id="r-komentar" value={komentar} maxLength={500}
              onChange={(e) => setKomentar(e.target.value)} placeholder={t('mkt.commentPh')} />
          </div>
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setDialog(null)}>{t('mkt.cancel')}</Button>
            <Button onClick={kirimUlasan} disabled={kirimBusy}>{kirimBusy ? t('mkt.sending') : t('mkt.send')}</Button>
          </div>
        </div>
      </Dialog>
    </main>{!user && <PublicFooter />}</div>
  );
}
