'use client';
// Blog: daftar artikel. Kartu mengarah ke /artikel/[slug].
import Link from 'next/link';
import { useState } from 'react';
import { useFetch } from '@/lib/useApi';
import { Button } from '@/components/ui/button';
import { Pagination } from '@/components/ui/Pagination';
import { asList } from '@/lib/utils';
import { tanggalLang } from '@/lib/tanggal';
import { Newspaper, PenLine, Image as ImageIcon } from 'lucide-react';
import { useT, useUrlPrefix } from '@/lib/i18n';

type Artikel = { id?: string; slug: string; title: string; excerpt?: string; image?: string; cover_url?: string;
  published_at?: string; author?: string; is_admin_post?: boolean };

export function ArtikelSection() {
  const { t, lang } = useT();
  const prefix = useUrlPrefix();
  const [page, setPage] = useState(1);
  const { data, loading, err } = useFetch<{ has_more?: boolean }>(`/articles?page=${page}`);
  const daftar = asList<Artikel>(data, 'articles');

  return (
    <>
      <div className="flex items-start justify-between gap-4 mb-8">
        <div>
          <p className="label mb-1">{t('art.kicker')}</p>
          <h1 className="text-2xl font-semibold tracking-tight">{t('art.h')}</h1>
        </div>
        <Link href={`${prefix}/tulis-artikel`}><Button variant="outline"><PenLine className="w-4 h-4" aria-hidden="true" /> {t('art.write')}</Button></Link>
      </div>

      {err ? (
        <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{t('err.load')}</p>
      ) : loading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {[0, 1, 2, 3].map((i) => <div key={i} className="card p-5 h-48"><div className="skel h-4 w-24 mb-3" /><div className="skel h-5 w-3/4 mb-3" /><div className="skel h-3 w-full" /></div>)}
        </div>
      ) : daftar.length === 0 ? (
        <div className="card p-12 text-center">
          <Newspaper className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />
          <p className="font-medium">{t('art.empty')}</p>
          <p className="text-sm text-mute mt-1 mb-6">{t('art.emptyP')}</p>
          <Link href={`${prefix}/tulis-artikel`}><Button>{t('art.writeLong')}</Button></Link>
        </div>
      ) : (
        <ul className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {daftar.map((a) => {
            const gambar = a.image || a.cover_url;
            return (
            <li key={a.id ?? a.slug} className="card lift p-5 flex flex-col">
              {gambar ? (
                // ponytail: <img> biasa; ganti next/image saat domain cover tetap
                <img src={gambar} alt={a.title} loading="lazy"
                  className="w-full h-40 object-cover rounded-lg mb-4 border border-border" />
              ) : (
                <div aria-hidden="true" className="w-full h-40 rounded-lg mb-4 border border-border bg-gradient-to-br from-cyan-500/10 to-surface-hi flex items-center justify-center">
                  <ImageIcon className="w-8 h-8 text-mute/70" />
                </div>
              )}
              <div className="flex flex-wrap items-center gap-2 mb-2">
                <p className="label">{tanggalLang(a.published_at, lang)}{a.author ? ` · ${a.author}` : ''}</p>
                {/* Chip asal penulis: is_admin_post dari gateway (web lama: "Tim" / "Komunitas"). */}
                <span className="text-[11px] px-2 py-1 rounded-full border border-border text-mute">
                  {a.is_admin_post ? t('art.team') : t('art.community')}
                </span>
              </div>
              <h2 className="font-semibold leading-snug">{a.title}</h2>
              {a.excerpt && <p className="text-sm text-mute mt-2 mb-4 flex-1 line-clamp-3 leading-relaxed">{a.excerpt}</p>}
              <Link href={`${prefix}/artikel/${a.slug}`} className="text-sm text-cyan-300 hover:text-cyan-400 font-medium mt-auto">
                {t('art.more')}
              </Link>
            </li>
            );
          })}
        </ul>
      )}

      {!err && !loading && daftar.length > 0 && (
        <Pagination page={page} hasMore={data?.has_more} onPage={setPage} />
      )}
    </>
  );
}
