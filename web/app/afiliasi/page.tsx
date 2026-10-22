'use client';
// Afiliasi: buat link untuk sebuah produk, lalu pantau klik & konversinya.
import { useState } from 'react';
import { post, pesanError } from '@/lib/http';
import { useFetch } from '@/lib/useApi';
import { useToast } from '@/components/ui/toast';
import { Button } from '@/components/ui/button';
import { Select } from '@/components/ui/select';
import { rupiah, angka, asList } from '@/lib/utils';
import { useT } from '@/lib/i18n';
import { Link2, Copy, Check } from 'lucide-react';
import { Pagination, useClientPagination } from '@/components/ui/Pagination';

type Link = { id?: string; code: string; url?: string; product_id?: string; title?: string; clicks?: number; conversions?: number; earnings?: number };
type Produk = { id: string; title: string; affiliate_percent?: number };

export default function AfiliasiPage() {
  const { t } = useT();
  const { data, loading, err, reload } = useFetch<unknown>('/affiliate/links');
  const produk = useFetch<unknown>('/market/products?affiliate=true&per_page=100');
  const [produkId, setProdukId] = useState('');
  const [busy, setBusy] = useState(false);
  const [copied, setCopied] = useState('');
  const toast = useToast();

  const links = asList<Link>(data, 'links');
  const daftarProduk = asList<Produk>(produk.data, 'products');
  const ringkas = (data as { summary?: { clicks?: number; conversions?: number; earnings?: number } } | null)?.summary;
  const linksPage = useClientPagination(links, 10);

  const buat = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      const { data: res } = await post<Link>('/affiliate/links', { product_id: produkId || daftarProduk[0]?.id });
      toast.ok(t('sel.linkCreated', { code: res?.code ?? 'OK' }));
      reload();
    } catch (e) {
      toast.err(pesanError(e, t('sel.errLink')));
    } finally {
      setBusy(false);
    }
  };

  const salin = async (url: string, code: string) => {
    const shareURL = new URL(url, location.origin).toString();
    try {
      await navigator.clipboard.writeText(shareURL);
      setCopied(code);
      toast.ok(t('sel.copied'));
      setTimeout(() => setCopied(''), 2000);
    } catch {
      toast.err(t('sel.errCopy'));
    }
  };

  return (
    <div className="p-6 md:p-10 max-w-3xl mx-auto">
      <header className="mb-8">
        <p className="label mb-1">{t('sel.affKicker')}</p>
        <h1 className="text-2xl font-semibold tracking-tight">{t('sel.affTitle')}</h1>
        <p className="text-sm text-mute mt-2">{t('sel.affIntro')}</p>
      </header>

      <section aria-label={t('sel.affHowH')} className="card p-5 mb-8 text-sm leading-6 text-mute">
        <h2 className="font-semibold text-text">{t('sel.affHowH')}</h2>
        <p className="mt-2">{t('sel.affHowP')}</p>
        <p className="mt-2">{t('sel.affHowP2')}</p>
      </section>

      {ringkas && (
        <section aria-label={t('sel.affSummaryAria')} className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
          {[[t('sel.clicks'), angka(ringkas.clicks)], [t('sel.conversions'), angka(ringkas.conversions)], [t('sel.earnings'), rupiah(ringkas.earnings)]].map(([l, v]) => (
            <div key={l} className="card lift p-4">
              <span className="label">{l}</span>
              <span className="block mt-2 font-semibold tabular-nums">{v}</span>
            </div>
          ))}
        </section>
      )}

      <form onSubmit={buat} className="card p-5 space-y-4 mb-10">
        <h2 className="font-semibold">{t('sel.newLink')}</h2>
        {produk.err && <p role="alert" className="text-sm text-danger">{produk.err}</p>}
        <div className="space-y-2">
          <label htmlFor="produk" className="label">{t('sel.product')}</label>
          <Select id="produk" value={produkId} onChange={(e) => setProdukId(e.target.value)} disabled={produk.loading || daftarProduk.length === 0}>
            {produk.loading && <option>{t('sel.loadingProducts')}</option>}
            {!produk.loading && daftarProduk.length === 0 && <option value="">{t('sel.noAffiliateProducts')}</option>}
            {daftarProduk.map((p) => <option key={p.id} value={p.id}>{t('sel.commission', { title: p.title, n: angka(p.affiliate_percent) })}</option>)}
          </Select>
        </div>
        <Button type="submit" disabled={busy || produk.loading || daftarProduk.length === 0}>
          {busy ? t('sel.creating') : t('sel.createAffiliateButton')}
        </Button>
        {!produk.loading && daftarProduk.length === 0 && <p className="text-xs text-mute">{t('sel.noFakeProducts')}</p>}
      </form>

      <section aria-label={t('sel.affLinksAria')}>
        <h2 className="text-lg font-semibold tracking-tight mb-4">{t('sel.myLinks')}</h2>
        {err ? (
          <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{err}</p>
        ) : loading ? (
          <div className="space-y-3">
            {[0, 1].map((i) => <div key={i} className="card p-4 h-20"><div className="skel h-4 w-56 mb-2" /><div className="skel h-3 w-40" /></div>)}
          </div>
        ) : links.length === 0 ? (
          <div className="card p-10 text-center">
            <Link2 className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />
            <p className="font-medium">{t('sel.noLinks')}</p>
            <p className="text-sm text-mute mt-1">{t('sel.noLinksP')}</p>
          </div>
        ) : (
          <ul className="space-y-3">
            {linksPage.items.map((l) => {
              const url = l.url || `/market?aff=${l.code}`;
              return (
                <li key={l.id ?? l.code} className="card p-4">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div className="min-w-0">
                      <p className="text-sm font-medium truncate">{l.title || l.product_id || t('sel.noneProducts')}</p>
                      <p className="text-xs text-mute font-mono mt-1 break-all">{url}</p>
                    </div>
                    <Button size="sm" variant="outline" onClick={() => salin(url, l.code)}>
                      {copied === l.code ? <Check className="w-4 h-4" aria-hidden="true" /> : <Copy className="w-4 h-4" aria-hidden="true" />}
                      {copied === l.code ? t('sel.copied') : t('sel.copy')}
                    </Button>
                  </div>
                  <p className="text-xs text-mute mt-3 tabular-nums">
                    {t('sel.linkStats', { clicks: angka(l.clicks), conversions: angka(l.conversions), earnings: rupiah(l.earnings) })}
                  </p>
                </li>
              );
            })}
          </ul>
        )}
        {links.length > 10 && (
          <Pagination page={linksPage.page} hasMore={linksPage.hasMore} onPage={linksPage.setPage} />
        )}
      </section>
    </div>
  );
}
