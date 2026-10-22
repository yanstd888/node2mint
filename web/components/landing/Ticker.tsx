'use client';

import { useFetch } from '@/lib/useApi';

import { useT } from '@/lib/i18n';

type TickerLogo = { id: string; label: string; image_url: string; sort: number; is_active: boolean };

export default function Ticker() {
  const { t } = useT();
  const { data, loading, err } = useFetch<{ logos?: TickerLogo[] }>('/ticker');
  const logos = (data?.logos ?? []).filter((logo) => logo.is_active && logo.image_url);

  if (loading || err || logos.length === 0) return null;

  return (
    <section aria-label={t('ticker')} className="relative -mx-4 overflow-hidden border-y border-white/10 py-4 sm:-mx-6">
      <div className="marquee-track">
        {[0, 1].map((copy) => (
          <div className="marquee-group" aria-hidden={copy === 1} key={copy}>
            {Array.from({ length: 3 }, (_, repeat) => logos.map((logo) => (
              <div className="ticker-item" aria-hidden={copy === 1 || repeat > 0} data-provider={logo.label.toLowerCase().replace(/[^a-z0-9]+/g, '-')} key={`${logo.id}-${copy}-${repeat}`}>
                <img
                  src={logo.image_url}
                  alt=""
                  aria-hidden="true"
                  className="h-6 w-6 shrink-0 object-contain"
                />
                <span>{logo.label}</span>
              </div>
            )))}
          </div>
        ))}
      </div>
    </section>
  );
}

