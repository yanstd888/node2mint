'use client';
// 3 artikel terbit terbaru dengan gambar (placeholder bergradien bila kosong),
// kategori resmi/komunitas + tanggal. API mati / kosong -> section hilang.
import Link from 'next/link';
import { useFetch } from '@/lib/useApi';
import { asList, tanggal } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Newspaper, ArrowUpRight, PenLine } from 'lucide-react';
import { useT, useUrlPrefix } from '@/lib/i18n';

type Artikel = {
  id?: string; slug: string; title: string; excerpt?: string; image?: string;
  published_at?: string; created_at?: string; is_admin_post?: boolean;
};

export default function LatestArticles() {
  const { t } = useT();
  const prefix = useUrlPrefix();
  const { data, loading, err } = useFetch<unknown>('/articles');
  const artikel = asList<Artikel>(data, 'articles').slice(0, 3);

  if (err || loading) return null;

  return (
    <section id="blog" aria-labelledby="artikel-judul" className="py-16 sm:py-20 scroll-mt-16">
      <div className="flex flex-wrap items-end justify-between gap-4 mb-8">
        <div>
          <p className="label mb-1">{t('articles.kicker')}</p>
          <h2 id="artikel-judul" className="text-2xl sm:text-3xl font-semibold tracking-tight">{t('articles.h')}</h2>
        </div>
        <Link
          href={`${prefix}/artikel`}
          className="inline-flex min-h-11 items-center text-sm font-medium text-cyan-300 transition-colors hover:text-cyan-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60"
        >
          {t('articles.all')}
        </Link>
      </div>
      {artikel.length === 0 ? (
        <p className="border-y border-white/10 py-7 text-sm text-mute" role="status">{t('articles.empty')}</p>
      ) : (
        <ul className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {artikel.map((a, i) => (
            <li key={a.id ?? a.slug} style={{ animationDelay: `${Math.min(0.08 * i, 0.2)}s` }} className="anim-fade-up">
              <Link
                href={`${prefix}/artikel/${a.slug}`}
                className="group item-card h-full overflow-hidden rounded-none border-white/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60"
              >
                {a.image ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img src={a.image} alt={a.title} loading="lazy" className="w-full h-36 object-cover" />
                ) : (
                  <div className="h-36 bg-gradient-to-br from-cyan-500/15 via-surface-hi to-surface flex items-center justify-center">
                    <PenLine className="w-8 h-8 text-cyan-300/70" aria-hidden="true" />
                  </div>
                )}
                <div className="p-6 flex flex-col flex-1">
                  <div className="flex items-center justify-between gap-3">
                    <Badge tone={a.is_admin_post ? 'ok' : 'netral'}>{a.is_admin_post ? t('articles.official') : t('articles.community')}</Badge>
                    <span className="inline-flex items-center gap-2 text-xs text-mute">
                      <Newspaper className="w-3.5 h-3.5 text-cyan-300" aria-hidden="true" />
                      {tanggal(a.published_at ?? a.created_at)}
                    </span>
                  </div>
                  <h3 className="mt-3 text-xl font-medium tracking-tight leading-snug">{a.title}</h3>
                  <ArrowUpRight className="mt-4 h-4 w-4 text-cyan-200 transition-transform duration-300 group-hover:translate-x-1 group-hover:-translate-y-1" aria-hidden="true" />
                  {a.excerpt && <p className="mt-2 text-sm text-mute leading-relaxed line-clamp-2">{a.excerpt}</p>}
                </div>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
