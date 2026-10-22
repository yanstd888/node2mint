'use client';

import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import { ChevronLeft, ChevronRight, ExternalLink } from 'lucide-react';
import { useT } from '@/lib/i18n';

type MarketBanner = { image_url: string; link_url?: string; alt?: string; active: boolean };

export function MarketplaceBanner() {
  const { t } = useT();
  const [items, setItems] = useState<MarketBanner[]>([]);
  const [active, setActive] = useState(0);
  const [paused, setPaused] = useState(false);

  useEffect(() => {
    let alive = true;
    api.get<{ banners: MarketBanner[] }>('/market/banners')
      .then(({ data }) => { if (alive) setItems(data.banners || []); })
      .catch(() => {});
    return () => { alive = false; };
  }, []);

  useEffect(() => {
    if (paused || items.length < 2) return;
    const timer = window.setInterval(() => setActive((n) => (n + 1) % items.length), 5000);
    return () => window.clearInterval(timer);
  }, [items.length, paused]);

  if (!items.length) return null;
  const go = (n: number) => setActive((n + items.length) % items.length);

  return (
    <section aria-label={t('market.banner')} className="mx-auto mb-6 max-w-5xl overflow-hidden" onMouseEnter={() => setPaused(true)} onMouseLeave={() => setPaused(false)} onFocus={() => setPaused(true)} onBlur={(e) => { if (!e.currentTarget.contains(e.relatedTarget)) setPaused(false); }}>
      <div className="relative aspect-[8/3] overflow-hidden rounded-xl border border-white/10 bg-surface-hi shadow-2xl shadow-black/30">
        {(() => {
          const item = items[active];
          const image = <img src={item.image_url} alt={item.alt || t('market.bannerNum', { n: active + 1 })} className="block h-full w-full object-contain" />;
          return item.link_url ? <a href={item.link_url} target={item.link_url.startsWith('http') ? '_blank' : undefined} rel={item.link_url.startsWith('http') ? 'noreferrer' : undefined} className="group block focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-cyan-300">{image}<span className="absolute bottom-3 right-3 rounded-full bg-black/60 p-2 text-white opacity-0 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100"><ExternalLink className="h-4 w-4" /></span></a> : image;
        })()}
        {items.length > 1 && <>
          <button type="button" aria-label={t('market.prev')} onClick={() => go(active - 1)} className="absolute left-2 top-1/2 z-10 -translate-y-1/2 rounded-full border border-white/15 bg-black/50 p-2 text-white backdrop-blur hover:bg-black/70 focus-visible:ring-2 focus-visible:ring-cyan-300"><ChevronLeft className="h-5 w-5" /></button>
          <button type="button" aria-label={t('market.next')} onClick={() => go(active + 1)} className="absolute right-2 top-1/2 z-10 -translate-y-1/2 rounded-full border border-white/15 bg-black/50 p-2 text-white backdrop-blur hover:bg-black/70 focus-visible:ring-2 focus-visible:ring-cyan-300"><ChevronRight className="h-5 w-5" /></button>
        </>}
      </div>
      {items.length > 1 && <div className="mt-3 flex justify-center gap-2">{items.map((_, index) => <button key={index} type="button" aria-label={t('market.open', { n: index + 1 })} aria-current={index === active} onClick={() => setActive(index)} className={`h-1.5 rounded-full transition-all ${index === active ? 'w-6 bg-cyan-300' : 'w-2 bg-white/30 hover:bg-white/50'}`} />)}</div>}
    </section>
  );
}
