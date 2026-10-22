'use client';

import Link from 'next/link';
import { useFetch } from '@/lib/useApi';
import { Button } from '@/components/ui/button';
import { tanggalLang } from '@/lib/tanggal';
import { FileQuestion, ArrowLeft } from 'lucide-react';
import PublicHeader from '@/components/PublicHeader';
import PublicFooter from '@/components/PublicFooter';
import { useT, useUrlPrefix } from '@/lib/i18n';

type Artikel = { slug?: string; title: string; content?: string; excerpt?: string; image?: string; cover_url?: string; published_at?: string; author?: string; is_admin_post?: boolean };

export default function ArticleDetailClient({ slug, initial }: { slug: string; initial: { article?: Artikel } }) {
  const { t, lang } = useT();
  const prefix = useUrlPrefix();
  const { data, loading, err } = useFetch<{ article?: Artikel }>(`/articles/${encodeURIComponent(slug)}`, !slug, 0, initial);
  const artikel = data?.article;

  return (
    <div className="min-h-dvh"><PublicHeader /><main className="p-6 pt-24 md:p-10 md:pt-28 max-w-3xl mx-auto">
      {err ? (
        <div className="card p-12 text-center">
          <FileQuestion className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />
          <p role="alert" className="font-medium">{t('art.fail')}</p>
          <p className="text-sm text-mute mt-1 mb-6">{t('err.load')}</p>
          <Link href={`${prefix}/artikel`}><Button variant="outline">{t('art.back')}</Button></Link>
        </div>
      ) : loading ? (
        <>
          <div className="skel h-4 w-32 mb-4" />
          <div className="skel h-9 w-3/4 mb-6" />
          <div className="skel h-4 w-full mb-2" />
          <div className="skel h-4 w-full mb-2" />
          <div className="skel h-4 w-2/3" />
        </>
      ) : !artikel ? null : (
        <article>
          <p className="label mb-3">
            {tanggalLang(artikel.published_at, lang)}{artikel.author ? ` · ${artikel.author}` : ''}
            {artikel.is_admin_post !== undefined && ` · ${artikel.is_admin_post ? t('art.team') : t('art.community')}`}
          </p>
          <h1 className="text-2xl md:text-3xl font-semibold tracking-tight leading-snug mb-6">{artikel.title}</h1>
          <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify({
            '@context': 'https://schema.org', '@type': 'Article', headline: artikel.title,
            description: artikel.excerpt || artikel.title, datePublished: artikel.published_at,
            author: { '@type': 'Person', name: artikel.author || 'Node2Mint' },
            image: artikel.image || artikel.cover_url || undefined,
          }) }} />
          {(artikel.image || artikel.cover_url) && (
            <img src={artikel.image || artikel.cover_url} alt={artikel.title} loading="lazy"
              className="w-full max-h-96 object-cover rounded-xl border border-border mb-8" />
          )}
          <div className="text-sm md:text-base text-text leading-relaxed whitespace-pre-wrap break-words [&_blockquote]:border-l-2 [&_blockquote]:border-cyan-400/60 [&_blockquote]:pl-4 [&_h2]:mb-3 [&_h2]:mt-7 [&_h2]:text-2xl [&_h2]:font-semibold [&_h3]:mb-2 [&_h3]:mt-5 [&_h3]:text-xl [&_h3]:font-semibold [&_ol]:my-4 [&_ol]:list-decimal [&_ol]:pl-6 [&_p]:my-3 [&_ul]:my-4 [&_ul]:list-disc [&_ul]:pl-6" dangerouslySetInnerHTML={{ __html: artikel.content || artikel.excerpt || t('art.emptyBody') }} />
          <Link href={`${prefix}/artikel`} className="inline-flex items-center gap-2 mt-10 text-sm text-cyan-300 hover:text-cyan-400 font-medium">
            <ArrowLeft className="w-4 h-4" aria-hidden="true" /> {t('art.all')}
          </Link>
        </article>
      )}
    </main><PublicFooter /></div>
  );
}
