'use client';

import Link from 'next/link';
import { ArrowRight } from 'lucide-react';
import { useFetch } from '@/lib/useApi';
import { asList } from '@/lib/utils';
import { useT, useUrlPrefix } from '@/lib/i18n';
import MarketplaceShelf from '@/components/landing/MarketplaceShelf';

type Product = {
  id: string;
  title: string;
  price: number;
  stock: number;
  is_demo?: boolean;
  image_url?: string;
  store_name?: string;
  store_verified?: boolean;
  store_admin?: boolean;
  created_at?: string;
};

export default function Showcase({ initialBestSellers = null }: { initialBestSellers?: unknown }) {
  const { t } = useT();
  const prefix = useUrlPrefix();
  const best = useFetch<unknown>('/market/products?sort=best_selling&available=true&per_page=10', false, 60_000, initialBestSellers);

  return (
    <section id="marketplace" aria-labelledby="marketplace-title" className="scroll-mt-24 py-16 sm:py-24">
      <div className="max-w-3xl">
        <p className="label mb-2">{t('show.kicker')}</p>
        <h2 id="marketplace-title" className="max-w-3xl text-3xl font-semibold tracking-tight sm:text-4xl">{t('show.h')}</h2>
        <p className="mt-4 max-w-2xl text-sm leading-relaxed text-mute sm:text-base">{t('show.sub')}</p>
        <Link href={`${prefix}/market`} className="mt-7 inline-flex min-h-11 items-center gap-2 text-sm font-medium text-cyan-300 transition-colors hover:text-cyan-200">
          {t('show.all')} <ArrowRight className="h-4 w-4" aria-hidden="true" />
        </Link>
      </div>
      <div className="mt-6">
        <MarketplaceShelf title={t('show.best')} products={asList<Product>(best.data, 'products')} loading={best.loading} slider maxItems={10} />
      </div>
    </section>
  );
}
