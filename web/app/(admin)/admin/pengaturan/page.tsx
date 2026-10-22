'use client';



import { useEffect, useState } from 'react';

import { api } from '@/lib/api';

import { patch, pesanError, post } from '@/lib/http';

import { useFetch } from '@/lib/useApi';

import { useToast } from '@/components/ui/toast';
import { useConfirm } from '@/components/ui/confirm';

import { useT } from '@/lib/i18n';

import { Button } from '@/components/ui/button';

import { ImageField } from '@/components/ui/ImageField';

import { Input } from '@/components/ui/input';

import { ArrowDown, ArrowUp, ImageIcon, Settings, Trash2 } from 'lucide-react';



type Nilai = string | number | boolean;

type TickerLogo = { id: string; label: string; image_url: string; sort: number; is_active: boolean };
type MarketBanner = { image_url: string; link_url: string; alt: string; active: boolean };

const GROUPS = [
  ['Umum', (k: string) => ['site_name', 'maintenance_mode', 'admin_store_user_email'].includes(k)],
  ['Pasar Digital & Penjual', (k: string) => k.startsWith('market_') || k.startsWith('boost_') || k.startsWith('verified_store_')],
  ['Token, Hadiah & Rujukan', (k: string) => k.includes('token') || k.startsWith('daily_reward_') || k.startsWith('referral_') || k.startsWith('usage_') || k.startsWith('cached_')],
  ['Isi Saldo & Penarikan', (k: string) => k.startsWith('topup_') || k.startsWith('withdrawal_')],
  ['Keamanan & Pencegahan Penyalahgunaan', (k: string) => k.startsWith('admin_ip_') || k.startsWith('v1_origin_') || k.startsWith('fraud_')],
  ['Komunitas', (k: string) => k.startsWith('community_')],
] as const;



export default function AdminPengaturanPage() {

  const settings = useFetch<unknown>('/admin/settings');

  const ticker = useFetch<{ logos?: TickerLogo[] }>('/admin/ticker');

  const [form, setForm] = useState<Record<string, Nilai>>({});
  const [banners, setBanners] = useState<MarketBanner[]>([]);
  const [newBanner, setNewBanner] = useState<MarketBanner>({ image_url: '', link_url: '', alt: '', active: true });

  const [logo, setLogo] = useState({ label: '', image_url: '', sort: '0' });

  const [busy, setBusy] = useState(false);

  const toast = useToast();
  const confirm = useConfirm();

  const { t } = useT();



  useEffect(() => {

    const sumber = ((settings.data as { settings?: Record<string, Nilai> } | null)?.settings ?? settings.data ?? {}) as Record<string, Nilai>;

    setForm(Object.fromEntries(Object.entries(sumber).filter(([k, v]) => k !== 'market_banners' && !k.startsWith('topup_ewallet_') && ['string', 'number', 'boolean'].includes(typeof v))));
    try { setBanners(JSON.parse(String(sumber.market_banners || '[]'))); } catch { setBanners([]); }

  }, [settings.data]);



  const simpan = async (e: React.FormEvent) => {

    e.preventDefault(); setBusy(true);

    try { await patch('/admin/settings', form); toast.ok(t('ad.st.saved')); settings.reload(); }

    catch (e) { toast.err(pesanError(e, t('ad.st.saveFail'))); }

    finally { setBusy(false); }

  };

  const tambahLogo = async (e: React.FormEvent) => {

    e.preventDefault();

    if (!logo.label.trim() || !logo.image_url) return toast.err(t('ad.st.logoRequired'));

    setBusy(true);

    try { await post('/admin/ticker', { label: logo.label.trim(), image_url: logo.image_url, sort: Number(logo.sort) || 0 }); setLogo({ label: '', image_url: '', sort: '0' }); toast.ok(t('ad.st.logoAdded')); ticker.reload(); }

    catch (e) { toast.err(pesanError(e, t('ad.st.logoAddFail'))); }

    finally { setBusy(false); }

  };

  const ubahLogo = async (id: string, body: Partial<TickerLogo>) => {

    try { await patch(`/admin/ticker`, { id, ...body }); ticker.reload(); }

    catch (e) { toast.err(pesanError(e, t('ad.st.logoUpdFail'))); }

  };

  const hapusLogo = async (id: string) => {
    const item = logos.find((x) => x.id === id);
    if (!(await confirm({ title: t('ad.common.deleteOf', { name: item?.label || t('ad.st.tickerH') }), body: `${item?.label || t('ad.st.tickerH')} will be permanently deleted.`, confirmLabel: t('ad.common.delete') }))) return;
    try { await api.del(`/admin/ticker/${id}`); ticker.reload(); }

    catch (e) { toast.err(pesanError(e, t('ad.st.logoDelFail'))); }

  };
  const simpanBanner = async (next: MarketBanner[]) => {
    setBusy(true);
    try { await patch('/admin/settings', { market_banners: next }); setBanners(next); toast.ok(t('ad.st.bannerUpdated')); }
    catch (e) { toast.err(pesanError(e, t('ad.st.bannerSaveFail'))); }
    finally { setBusy(false); }
  };
  const tambahBanner = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newBanner.image_url) return toast.err(t('ad.st.bannerImageRequired'));
    await simpanBanner([...banners, { ...newBanner, link_url: newBanner.link_url.trim(), alt: newBanner.alt.trim() }]);
    setNewBanner({ image_url: '', link_url: '', alt: '', active: true });
  };
  const pindahBanner = (index: number, delta: number) => {
    const target = index + delta;
    if (target < 0 || target >= banners.length) return;
    const next = [...banners];
    [next[index], next[target]] = [next[target], next[index]];
    simpanBanner(next);
  };
  const logos = ticker.data?.logos ?? [];
  const grouped: { title: string; keys: string[] }[] = GROUPS.map(([title, match]) => ({ title, keys: Object.keys(form).filter(match) }));
  const known = new Set(grouped.flatMap((g) => g.keys));
  grouped.push({ title: 'Lainnya', keys: Object.keys(form).filter((k) => !known.has(k)) });


  return <div className="mx-auto max-w-5xl p-6 md:p-10">
    <header className="mb-8"><p className="label mb-1">{t('ad.common.admin')}</p><h1 className="text-2xl font-semibold tracking-tight">{t('ad.st.h')}</h1><p className="mt-2 text-sm text-mute">{t('ad.st.sub')}</p></header>
    {settings.err ? <p role="alert" className="banner banner-bad">{settings.err}</p> : settings.loading ? <div className="card space-y-4 p-5">{[0, 1, 2].map((i) => <div key={i} className="skel h-10 w-full" />)}</div> : Object.keys(form).length > 0 ? <form onSubmit={simpan} className="space-y-6">
      {grouped.filter((g) => g.keys.length).map((g) => <section key={g.title} className="card p-5"><h2 className="mb-5 text-lg font-semibold">{g.title}</h2><div className="grid gap-5 md:grid-cols-2">{g.keys.map((k) => <div key={k} className="space-y-2">{k === 'topup_qris_image_url' ? <ImageField label={t('ad.st.qrisImage')} value={String(form[k] || '')} onChange={(url) => setForm({ ...form, [k]: url })} /> : <><label htmlFor={`set-${k}`} className="label">{k.replace(/[_.]/g, ' ')}</label>{typeof form[k] === 'boolean' ? <label className="flex items-center gap-3 text-sm"><input id={`set-${k}`} type="checkbox" checked={form[k] as boolean} onChange={(e) => setForm({ ...form, [k]: e.target.checked })} className="h-4 w-4 rounded accent-cyan-500" />{t('ad.st.active')}</label> : <Input id={`set-${k}`} type={typeof form[k] === 'number' ? 'number' : 'text'} value={String(form[k])} onChange={(e) => setForm({ ...form, [k]: typeof form[k] === 'number' ? Number(e.target.value) : e.target.value })} />}</>}</div>)}</div></section>)}
      <div className="sticky bottom-4 flex justify-end"><Button type="submit" disabled={busy}>{busy ? t('ad.common.saving') : t('ad.st.save')}</Button></div>
    </form> : <div className="card p-10 text-center"><Settings className="mx-auto mb-4 h-10 w-10 text-mute" /><p className="font-medium">{t('ad.st.empty')}</p></div>}

    <section className="mt-8 card p-5" aria-labelledby="market-banner-title">
      <div className="mb-5"><p className="label mb-1">{t('ad.st.market')}</p><h2 id="market-banner-title" className="text-lg font-semibold">{t('ad.st.bannerTitle')}</h2><p className="mt-1 text-sm text-mute">{t('ad.st.bannerDesc')}</p></div>
      <form onSubmit={tambahBanner} className="grid gap-4 border-b border-white/10 pb-5 md:grid-cols-2"><div className="md:col-span-2"><ImageField label={t('ad.st.bannerImage')} value={newBanner.image_url} onChange={(image_url) => setNewBanner({ ...newBanner, image_url })} /></div><div className="space-y-2"><label htmlFor="banner-alt" className="label">{t('ad.st.bannerAlt')}</label><Input id="banner-alt" value={newBanner.alt} onChange={(e) => setNewBanner({ ...newBanner, alt: e.target.value })} placeholder={t('ad.st.bannerPromoPh')} /></div><div className="space-y-2"><label htmlFor="banner-link" className="label">{t('ad.st.bannerLink')}</label><Input id="banner-link" value={newBanner.link_url} onChange={(e) => setNewBanner({ ...newBanner, link_url: e.target.value })} placeholder="https://... atau /market?..." /></div><div><Button type="submit" disabled={busy}><ImageIcon className="h-4 w-4" />{t('ad.st.bannerAdd')}</Button></div></form>
      {banners.length ? <ul className="mt-5 space-y-4">{banners.map((item, index) => <li key={`${item.image_url}-${index}`} className="grid gap-4 rounded-xl border border-white/10 p-4 md:grid-cols-[11rem_1fr_auto] md:items-center"><img src={item.image_url} alt="" className="h-24 w-full rounded-lg object-cover" /><div className="min-w-0"><p className="truncate text-sm font-medium">{item.alt || `${t('ad.st.bannerTitle')} ${index + 1}`}</p><p className="mt-1 truncate text-xs text-mute">{item.link_url || t('ad.st.bannerNoUrl')}</p><label className="mt-3 flex items-center gap-2 text-xs text-mute"><input type="checkbox" checked={item.active} onChange={(e) => simpanBanner(banners.map((b, i) => i === index ? { ...b, active: e.target.checked } : b))} className="h-4 w-4 accent-cyan-500" />{t('ad.st.bannerActive')}</label></div><div className="flex gap-2"><Button type="button" size="sm" variant="outline" aria-label={t('ad.st.bannerUp')} disabled={index === 0 || busy} onClick={() => pindahBanner(index, -1)}><ArrowUp className="h-4 w-4" /></Button><Button type="button" size="sm" variant="outline" aria-label={t('ad.st.bannerDown')} disabled={index === banners.length - 1 || busy} onClick={() => pindahBanner(index, 1)}><ArrowDown className="h-4 w-4" /></Button><Button type="button" size="sm" variant="outline" aria-label={t('ad.st.bannerDelete')} disabled={busy} onClick={async () => { if (await confirm({ title: t('ad.st.bannerDelete'), body: item.alt || `${t('ad.st.bannerTitle')} ${index + 1}`, confirmLabel: t('ad.common.delete') })) await simpanBanner(banners.filter((_, i) => i !== index)); }}><Trash2 className="h-4 w-4" /></Button></div></li>)}</ul> : <p className="mt-5 text-sm text-mute">{t('ad.st.empty')}</p>}
    </section>


    <section className="mt-8 card p-5" aria-labelledby="ticker-title">

      <div className="mb-5"><p className="label mb-1">{t('ad.st.landing')}</p><h2 id="ticker-title" className="text-lg font-semibold">{t('ad.st.tickerH')}</h2><p className="mt-1 text-sm text-mute">{t('ad.st.tickerP')}</p></div>

      <form onSubmit={tambahLogo} className="grid gap-4 border-b border-white/10 pb-5 sm:grid-cols-2">

        <div className="space-y-2"><label htmlFor="ticker-label" className="label">{t('ad.st.label')}</label><Input id="ticker-label" value={logo.label} onChange={(e) => setLogo({ ...logo, label: e.target.value })} required /></div>

        <div className="space-y-2"><label htmlFor="ticker-sort" className="label">{t('ad.st.sort')}</label><Input id="ticker-sort" type="number" value={logo.sort} onChange={(e) => setLogo({ ...logo, sort: e.target.value })} /></div>

        <div className="sm:col-span-2"><ImageField label={t('ad.st.logoImage')} value={logo.image_url} onChange={(image_url) => setLogo({ ...logo, image_url })} /></div>

        <div><Button type="submit" disabled={busy}>{busy ? t('ad.common.saving') : t('ad.st.logoAdd')}</Button></div>

      </form>

      {ticker.err ? <p role="alert" className="mt-5 text-sm text-danger">{ticker.err}</p> : <ul className="mt-5 divide-y divide-white/10">{logos.map((item) => <li key={item.id} className="flex flex-wrap items-center gap-3 py-3"><img src={item.image_url} alt="" className="h-10 w-16 rounded border border-white/10 object-contain p-1" /><span className="min-w-28 flex-1 text-sm font-medium">{item.label}</span><label className="flex items-center gap-2 font-mono text-xs text-mute">{t('ad.st.sort')} <input aria-label={t('ad.st.sortAria', { name: item.label })} type="number" defaultValue={item.sort} onBlur={(e) => { const sort = Number(e.target.value); if (sort !== item.sort) ubahLogo(item.id, { sort }); }} className="w-16 rounded-md border border-white/10 bg-white/[.035] px-2 py-1 text-text" /></label><label className="flex items-center gap-2 text-xs text-mute"><input aria-label={t('ad.st.activateAria', { name: item.label })} type="checkbox" checked={item.is_active} onChange={(e) => ubahLogo(item.id, { is_active: e.target.checked })} className="h-4 w-4 accent-cyan-500" />{t('ad.st.active')}</label><Button type="button" size="sm" variant="outline" aria-label={`${t('ad.common.delete')} ${item.label}`} onClick={() => hapusLogo(item.id)}><Trash2 className="h-4 w-4" /></Button></li>)}</ul>}

    </section>

  </div>;

}

