'use client';

// Admin · Artikel: moderasi (tayangkan / tarik) + sunting isi lengkap.

import { useState } from 'react';

import { patch, pesanError, post } from '@/lib/http';

import { useFetch } from '@/lib/useApi';

import { useT } from '@/lib/i18n';

import { Dialog } from '@/components/ui/dialog';

import { useToast } from '@/components/ui/toast';

import { Button } from '@/components/ui/button';

import { Input } from '@/components/ui/input';

import { Textarea } from '@/components/ui/textarea';

import { RichTextEditor } from '@/components/ui/RichTextEditor';

import { ImageField } from '@/components/ui/ImageField';

import { tanggal, asList } from '@/lib/utils';

import { Newspaper, Pencil } from 'lucide-react';

import { Pagination, useClientPagination } from '@/components/ui/Pagination';

type Artikel = { id: string; slug?: string; title: string; content?: string; excerpt?: string; image?: string; status?: string; published?: boolean; is_published?: boolean; author?: string; created_at?: string };

export default function AdminArtikelPage() {

  const { data, loading, err, reload } = useFetch<unknown>('/admin/articles');

  const { t } = useT();

  const toast = useToast();

  const [busy, setBusy] = useState<string | null>(null);

  const [sunting, setSunting] = useState<Artikel | null>(null);

  const [simpanBusy, setSimpanBusy] = useState(false);

  const daftar = asList<Artikel>(data, 'articles');

  const daftarPage = useClientPagination(daftar, 10);

  // Simpan suntingan lewat POST /admin/articles + id — satu-satunya jalur tulis
  // yang menerima judul, isi, ringkasan, dan sampul sekaligus (PATCH hanya status).
  const simpanSunting = async (e: React.FormEvent) => {

    e.preventDefault();

    if (!sunting) return;

    setSimpanBusy(true);

    try {

      await post('/admin/articles', {

        id: sunting.id, title: sunting.title, content: sunting.content || '',

        excerpt: sunting.excerpt || '', image: sunting.image || '',

        published: sunting.published ?? sunting.is_published ?? sunting.status === 'PUBLISHED',

      });

      toast.ok(t('ad.ar.saved'));

      setSunting(null);

      await reload();

    } catch (err) {

      toast.err(pesanError(err, t('ad.ar.fail')));

    } finally {

      setSimpanBusy(false);

    }

  };

  const setTerbit = async (id: string, nilai: boolean) => {

    setBusy(id);

    try {

      await patch(`/admin/articles/${id}`, { published: nilai });

      toast.ok(nilai ? t('ad.ar.published') : t('ad.ar.unpublished'));

      await reload();

    } catch (e) {

      toast.err(pesanError(e, t('ad.ar.fail')));

    } finally {

      setBusy(null);

    }

  };

  return (

    <div className="p-6 md:p-10 max-w-4xl mx-auto">

      <header className="mb-8">

        <p className="label mb-1">{t('ad.common.admin')}</p>

        <h1 className="text-2xl font-semibold tracking-tight">{t('ad.ar.h')}</h1>

      </header>

      {err && <p role="alert" className="mb-6 text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{err}</p>}

      {err ? null : loading ? (

        <div className="space-y-3">{[0, 1, 2].map((i) => <div key={i} className="card p-4 h-20"><div className="skel h-4 w-56 mb-2" /><div className="skel h-3 w-40" /></div>)}</div>

      ) : daftar.length === 0 ? (

        <div className="card p-10 text-center">

          <Newspaper className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />

          <p className="font-medium">{t('ad.ar.empty')}</p>

          <p className="text-sm text-mute mt-1">{t('ad.ar.emptyP')}</p>

        </div>

      ) : (

        <>

        <ul className="tbl-wrap zebra">

          {daftarPage.items.map((a) => {

            const terbit = a.published ?? a.is_published ?? a.status === 'APPROVED';

            return (

              <li key={a.id} className="p-4">

                <div className="min-w-0">

                  <div className="flex items-center gap-3 flex-wrap">

                    <span className="text-sm font-medium truncate">{a.title}</span>

                    <span className={`badge ${terbit ? 'badge-on' : 'badge-off'}`}>{terbit ? t('ad.ar.active') : t('ad.ar.inactive')}</span>

                  </div>

                  <p className="text-xs text-mute mt-1 truncate">

                    {a.author || '-'} &middot; {tanggal(a.created_at, true)} &middot; <span className="font-mono">{a.slug || '-'}</span>

                  </p>

                </div>

                {/* Basis penuh: baris tombol selalu turun ke bawah kartu, tidak
                    lagi menempel di kanan dan berdesakan dengan judul. */}
                <div className="act-group mt-3 basis-full justify-start border-t border-border pt-3">
                  <Button size="sm" variant="outline" onClick={() => setSunting(a)}>

                    <Pencil className="h-4 w-4" aria-hidden="true" /> {t('ad.ar.edit')}

                  </Button>
                  {a.status === 'PENDING' && (
                    <Button size="sm" variant="default" disabled={busy === a.id}
                      onClick={async () => {
                        setBusy(a.id);
                        try {
                          await patch(`/admin/articles/${a.id}`, { status: 'APPROVED' });
                          toast.ok(t('ad.ar.approved'));
                          await reload();
                        } catch (e) {
                          toast.err(pesanError(e, t('ad.ar.fail')));
                        } finally { setBusy(null); }
                      }}>
                      {busy === a.id ? t('ad.common.processing') : t('ad.common.approve')}
                    </Button>
                  )}
                  <Button size="sm" variant={terbit ? 'outline' : 'default'} disabled={busy === a.id}
                    onClick={() => setTerbit(a.id, !terbit)}>
                    {busy === a.id ? t('ad.common.processing') : terbit ? t('ad.ar.unpublish') : t('ad.ar.publish')}
                  </Button>
                </div>
              </li>

            );

          })}

        </ul>

        {daftar.length > 10 && (

          <Pagination page={daftarPage.page} hasMore={daftarPage.hasMore} onPage={daftarPage.setPage} />

        )}

        <Dialog open={!!sunting} title={t('ad.ar.dlgTitle')} onClose={() => setSunting(null)}>

          {sunting && <form onSubmit={simpanSunting} className="space-y-4">

            <div className="space-y-2"><label htmlFor="ar-judul" className="label">{t('ad.ar.fTitle')}</label><Input id="ar-judul" value={sunting.title} maxLength={200} onChange={(e) => setSunting({ ...sunting, title: e.target.value })} required /></div>

            <div className="space-y-2"><label htmlFor="ar-ringkas" className="label">{t('ad.ar.fExcerpt')}</label><Textarea id="ar-ringkas" value={sunting.excerpt || ''} maxLength={300} className="min-h-[60px]" onChange={(e) => setSunting({ ...sunting, excerpt: e.target.value })} /></div>

            <div className="space-y-2"><label htmlFor="ar-isi" className="label">{t('ad.ar.fContent')}</label><RichTextEditor id="ar-isi" value={sunting.content || ''} onChange={(content) => setSunting({ ...sunting, content })} required /></div>

            <ImageField label={t('ad.ar.fImage')} value={sunting.image || ''} onChange={(url) => setSunting({ ...sunting, image: url })} />

            <label className="flex items-center gap-3 text-sm"><input type="checkbox" className="h-4 w-4 accent-cyan-500" checked={sunting.published ?? sunting.is_published ?? sunting.status === 'PUBLISHED'} onChange={(e) => setSunting({ ...sunting, published: e.target.checked })} />{t('ad.ar.fPublished')}</label>

            <div className="flex justify-end gap-2"><Button type="button" variant="ghost" onClick={() => setSunting(null)} disabled={simpanBusy}>{t('sel.cancel')}</Button><Button type="submit" disabled={simpanBusy}>{simpanBusy ? t('sel.saving') : t('sel.save')}</Button></div>

          </form>}

        </Dialog>

        </>

      )}

    </div>

  );

}