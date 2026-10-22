'use client';

import Link from 'next/link';
import { Badge } from '@/components/ui/badge';
import { useEffect, useRef, useState } from 'react';
import { ChevronLeft, ChevronRight, ArrowRight, Package, ShieldCheck, Zap } from 'lucide-react';
import { useT, useUrlPrefix } from '@/lib/i18n';
import { angka, money, productUrl } from '@/lib/utils';

type Product = {
  id: string;
  title: string;
  price: number;
  stock: number;
  is_demo?: boolean;
  is_instant?: boolean;
  image_url?: string;
  store_name?: string;
  store_verified?: boolean;
  store_admin?: boolean;
  sales_count?: number;
  review_count?: number;
  created_at?: string;
};

export default function MarketplaceShelf({
  title,
  products,
  loading = false,
  slider = false,
  maxItems = 8,
}: {
  title: string;
  products: Product[];
  loading?: boolean;
  slider?: boolean;
  maxItems?: number;
}) {
  const { t, lang } = useT();
  const prefix = useUrlPrefix();
  const listRef = useRef<HTMLUListElement>(null);
  const [hydrated, setHydrated] = useState(false);
  useEffect(() => setHydrated(true), []);
  const visibleProducts = products.slice(0, maxItems);
  const move = (direction: number) => listRef.current?.scrollBy({ left: direction * listRef.current.clientWidth * 0.9, behavior: 'smooth' });
  if (!loading && visibleProducts.length === 0) return null;

  return (
    <section aria-label={title} className="mt-10 border-t border-white/10 pt-10 sm:mt-14 sm:pt-14">
      <div className="mb-5 flex items-end justify-between gap-4">
        <h3 className="text-xl font-semibold tracking-tight sm:text-2xl">{title}</h3>
        <Link href={`${prefix}/market`} className="inline-flex min-h-11 shrink-0 items-center gap-1 text-sm font-medium text-cyan-300 hover:text-cyan-200">
          {t('show.all')} <ArrowRight className="h-4 w-4" aria-hidden="true" />
        </Link>
      </div>
      {loading ? (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 sm:gap-4" aria-busy="true">
          {[0, 1, 2, 3].map((item) => <div key={item} className="card p-3 sm:p-4"><div className="skel aspect-square w-full" /><div className="mt-3 skel h-4 w-2/3" /><div className="mt-2 skel h-3 w-1/2" /></div>)}
        </div>
      ) : (
        <div className="relative">
          {slider && visibleProducts.length > 4 && <>
            <button type="button" aria-label={t('pg.prev')} onClick={() => move(-1)} className="absolute left-2 top-1/2 z-10 grid h-11 w-11 -translate-y-1/2 place-items-center rounded-full border border-white/15 bg-black/60 text-white backdrop-blur transition-colors hover:bg-black/80 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-300">
              <ChevronLeft className="h-4 w-4" aria-hidden="true" />
            </button>
            <button type="button" aria-label={t('pg.next')} onClick={() => move(1)} className="absolute right-2 top-1/2 z-10 grid h-11 w-11 -translate-y-1/2 place-items-center rounded-full border border-white/15 bg-black/60 text-white backdrop-blur transition-colors hover:bg-black/80 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-300">
              <ChevronRight className="h-4 w-4" aria-hidden="true" />
            </button>
          </>}
          <ul ref={slider ? listRef : undefined} className={slider ? 'grid auto-cols-[82%] grid-flow-col gap-3 overflow-x-auto pb-2 snap-x snap-mandatory [scrollbar-width:none] sm:auto-cols-[48%] lg:auto-cols-[calc((100%_-_3rem)_/_4)] [&::-webkit-scrollbar]:hidden' : 'grid grid-cols-2 gap-3 sm:grid-cols-4 sm:gap-4'}>
          {visibleProducts.map((product) => {
            const recent = hydrated && Boolean(product.created_at && Date.now() - new Date(product.created_at).getTime() < 7 * 24 * 3600 * 1000);
            return (
              <li key={product.id} className={`item-card overflow-hidden ${slider ? 'snap-start' : ''}`}>
                <Link href={`${prefix}${productUrl(product.store_name, product.title, product.id)}`} className="group block">
                  <div className="relative aspect-square overflow-hidden border-b border-white/10">
                    <Badge className="absolute left-2 top-2 z-10 !text-[10px] !text-white" tone={product.is_demo ? 'warn' : product.stock > 0 ? 'ok' : 'danger'}>
                      {product.is_demo ? t('mkt.demo') : product.stock > 0 ? t('show.stock', { n: angka(product.stock) }) : t('show.out')}
                    </Badge>
                    {product.image_url ? (
                      // eslint-disable-next-line @next/next/no-img-element
                      <img src={product.image_url} alt={product.title} loading="lazy" className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105" />
                    ) : (
                      <div className="flex h-full w-full items-center justify-center bg-gradient-to-br from-cyan-500/10 to-surface-hi"><Package className="h-8 w-8 text-mute/70" aria-hidden="true" /></div>
                    )}
                  </div>
                  <div className="flex min-h-44 flex-col p-3 sm:p-4">
                    <div className="flex min-h-5 flex-wrap items-center gap-1.5">
                      {recent && <Badge tone="ok">{t('show.new')}</Badge>}
                      {product.is_instant && <span className="inline-flex items-center gap-1 text-[11px] text-violet-200"><Zap className="h-3 w-3" aria-hidden="true" />{t('show.instant')}</span>}
                    </div>
                    <h4 className="mt-2 line-clamp-2 text-sm font-medium leading-snug">{product.title}</h4>
                    {product.store_name && <p className="mt-1 flex items-center gap-1 truncate text-xs text-mute"><span className="truncate">{product.store_name}</span>{product.store_verified && <ShieldCheck className={`h-3.5 w-3.5 shrink-0 ${product.store_admin ? 'text-red-400' : 'text-yellow-300'}`} aria-label={product.store_admin ? t('mktp.adminVerified') : t('mktp.sellerVerified')} />}</p>}
                    <p className="mt-auto pt-3 font-mono text-sm font-semibold text-cyan-200">{money(product.price, lang)}</p>
                    {(product.sales_count || product.review_count) ? <p className="mt-1 text-[11px] text-mute">{product.sales_count ? t('show.sales', { n: angka(product.sales_count) }) : ''}{product.sales_count && product.review_count ? ' · ' : ''}{product.review_count ? t('show.reviews', { n: angka(product.review_count) }) : ''}</p> : null}
                  </div>
                </Link>
              </li>
            );
          })}
          </ul>
        </div>
      )}
    </section>
  );
}
