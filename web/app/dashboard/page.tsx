'use client';
// Dashboard bento (brief §3 + Fanny #6): grid 1/2/4 kolom — saldo hero (2 kolom),
// pemakaian API tinggi (2x2), paket token (2), status toko (1), pesanan (4, full width).
// Analytics tetap menyatu di bawah (dulu halaman /analytics terpisah).
import { useEffect, useState } from 'react';
import Link from 'next/link';
import { api } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import { useT } from '@/lib/i18n';
import { money, tanggal, tanggalJam, asList } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { AlertOctagon } from 'lucide-react';
import AnalyticsSection from '@/components/AnalyticsSection';

type Summary = { balance: number; usageToday: number; tokenRemaining: number; tokenExpiresAt?: string | null; tokenActive: boolean };
type Toko = { id?: string; name?: string; products?: number; orders?: number };
type Pesanan = { id: string; kind?: 'PACKAGE' | 'MARKET'; quantity: number; subtotal: number; status: string; created_at?: string; product_title: string };
type RecentUsage = { model: string; input_tokens: number; cached_tokens: number; output_tokens: number; status: string; created_at: string };
type Usage = {
  totals: { requests: number; input_tokens: number; cached_tokens: number; output_tokens: number };
  recent?: RecentUsage[];
};
type AffSummary = { clicks?: number; conversions?: number; earnings?: number };

const localeFor = (lang: string) => ({ id: 'id-ID', cn: 'zh-CN', ru: 'ru-RU', jp: 'ja-JP' }[lang] || 'en-US');
const fmt = (n: number | null | undefined, lang = 'id') => {
  const value = Number(n);
  return Number.isFinite(value) ? Math.round(value).toLocaleString(localeFor(lang)) : '—';
};
const ago = (value: string, lang: string) => {
  const time = new Date(value).getTime();
  if (!Number.isFinite(time)) return '—';
  const seconds = Math.round((time - Date.now()) / 1000);
  const [amount, unit] = Math.abs(seconds) < 60
    ? [seconds, 'second']
    : Math.abs(seconds) < 3600
      ? [Math.round(seconds / 60), 'minute']
      : Math.abs(seconds) < 86400
        ? [Math.round(seconds / 3600), 'hour']
        : [Math.round(seconds / 86400), 'day'];
  return new Intl.RelativeTimeFormat(localeFor(lang), { numeric: 'auto', style: 'narrow' })
    .format(amount, unit as Intl.RelativeTimeFormatUnit);
};
const statusLabel = (status: string, lang: string) => ({
  PAID: { id: 'DIBAYAR', cn: '已支付', ru: 'ОПЛАЧЕНО', jp: '支払済み' },
  PROCESSING: { id: 'DIPROSES', cn: '处理中', ru: 'ОБРАБАТЫВАЕТСЯ', jp: '処理中' },
  COMPLETED: { id: 'SELESAI', cn: '已完成', ru: 'ЗАВЕРШЕНО', jp: '完了' },
  DISPUTED: { id: 'SENGKETA', cn: '有争议', ru: 'СПОР', jp: '紛争中' },
  REFUNDED: { id: 'DIKEMBALIKAN', cn: '已退款', ru: 'ВОЗВРАЩЕНО', jp: '返金済み' },
  CANCELLED: { id: 'DIBATALKAN', cn: '已取消', ru: 'ОТМЕНЕНО', jp: 'キャンセル' },
}[status]?.[lang as 'id' | 'cn' | 'ru' | 'jp'] || status);

export default function DashboardPage() {
  const { user } = useAuth();
  const { t, lang } = useT();
  const [data, setData] = useState<Summary | null>(null);
  const [err, setErr] = useState('');
  const [toko, setToko] = useState<Toko | null>(null);
  const [pesanan, setPesanan] = useState<Pesanan[]>([]);
  const [usage, setUsage] = useState<Usage | null>(null);
  const [aff, setAff] = useState<AffSummary | null>(null);

  useEffect(() => {
    if (!user) return;
    let alive = true;
    const load = async () => {
      // .catch per request: satu kegagalan tidak boleh menggantung seluruh kartu.
      const [me, st, ordr, usg, affl] = await Promise.all([
        api.get<Summary>('/dashboard').catch(() => null),
        api.get<{ store?: Toko }>('/store').catch(() => null),
        api.get<{ orders?: Pesanan[] }>('/purchases').catch(() => null),
        api.get<Usage>('/keys/usage').catch(() => null),
        api.get<{ summary?: AffSummary }>('/affiliate/links').catch(() => null),
      ]);
      if (!alive) return;
      if (me) setData(me.data);
      else setErr(t('dash.loadErr'));
      setToko(st?.data?.store ?? null);
      setPesanan(asList<Pesanan>(ordr?.data, 'orders').slice(0, 5));
      setUsage(usg?.data ?? null);
      setAff(affl?.data?.summary ?? null);
    };
    load();
    return () => { alive = false; };
  }, [user, t]);

  if (!user) return null;

  return (
    <div className="p-6 md:p-10 max-w-6xl mx-auto">
      <header className="mb-8">
        <p className="label mb-1">{t('dash.label')}</p>
        <h1 className="text-2xl md:text-3xl font-semibold tracking-tight truncate">
          {t('dash.hello', { name: user.name || user.email })}
        </h1>
      </header>

      {/* Bento user: saldo hero (2 kolom) + pemakaian API (2 kolom, 2 baris) */}
      <section aria-label={t('dash.acct')} className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4">
        {err ? (
          <div className="card p-6 md:col-span-2">
            <p role="alert" className="text-sm text-danger">{err}</p>
            <p className="text-sm text-mute mt-1">{t('dash.noWallet')}</p>
          </div>
        ) : !data ? (
          <div className="card p-6 md:col-span-2 h-36"><div className="skel h-4 w-20 mb-3" /><div className="skel h-9 w-40" /></div>
        ) : (
          <div className="card lift p-6 saldo md:col-span-2">
            {data.tokenRemaining <= 0 && (
              <p role="alert" className="mb-4 flex items-start gap-2 rounded-lg border border-danger/40 bg-danger/10 px-3 py-2 text-sm font-medium text-danger">
                <AlertOctagon className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
                {t('dash.tokenOutTitle')}
              </p>
            )}
            <span className="label">{t('dash.tokens')}</span>
            <span className="block mt-2 text-3xl md:text-4xl font-semibold tabular-nums tracking-tight text-cyan-300">
              {fmt(data.tokenRemaining, lang)} <span className="text-base font-normal text-mute">{t('dash.tokenUnit')}</span>
            </span>
            <p className={`text-sm mt-3 ${data.tokenActive ? 'text-mute' : 'text-amber-300'}`}>
              {!data.tokenActive && data.tokenRemaining > 0
                ? t('dash.stale')
                : data.tokenExpiresAt
                  ? t('dash.tokenExp', { date: tanggalJam(data.tokenExpiresAt, lang) })
                  : t('dash.tokenNone')}
            </p>
          </div>
        )}

        <div className="card lift p-6 md:col-span-2 xl:col-span-2 xl:row-span-2 flex flex-col">
          <span className="label text-xs">{t('dash.usage')}</span>
          <span className="block mt-2 text-3xl font-semibold tabular-nums tracking-tight text-text">
            {fmt(usage?.totals.requests, lang)} <span className="text-base font-normal text-mute">{t('dash.reqUnit')}</span>
          </span>
          <p className="text-sm text-mute mt-1">{t('dash.req24', { n: fmt(data?.usageToday ?? 0, lang) })}</p>
          <dl className="mt-5 grid grid-cols-3 gap-4" aria-label={t('dash.detail')}>
            <div><dt className="text-sm text-mute">{t('dash.in')}</dt><dd className="mt-1 text-lg font-semibold tabular-nums text-orange-300">{fmt(usage?.totals.input_tokens, lang)}</dd></div>
            <div><dt className="text-sm text-mute">{t('dash.cached')}</dt><dd className="mt-1 text-lg font-semibold tabular-nums text-cyan-300">{fmt(usage?.totals.cached_tokens, lang)}</dd></div>
            <div><dt className="text-sm text-mute">{t('dash.out')}</dt><dd className="mt-1 text-lg font-semibold tabular-nums text-emerald-300">{fmt(usage?.totals.output_tokens, lang)}</dd></div>
          </dl>

          <div className="mt-5 min-h-36 border-t border-border pt-4">
            <div className="mb-2 grid grid-cols-[minmax(0,1fr)_auto] gap-x-3 text-xs font-semibold uppercase tracking-wider text-mute sm:grid-cols-[minmax(0,1fr)_auto_auto] sm:gap-4">
              <span>{t('dash.recentModel')}</span>
              <span>{t('dash.recentTokens')}</span>
              <span className="hidden text-right sm:block">{t('dash.recentWhen')}</span>
            </div>
            {!usage?.recent?.length ? (
              <p className="py-8 text-center text-sm text-mute">{t('dash.noRecent')}</p>
            ) : (
              <ul className="max-h-48 overflow-y-auto divide-y divide-border/60 pr-1">
                {usage.recent.map((request, index) => (
                  <li key={`${request.created_at}-${index}`} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1 py-2.5 text-sm sm:grid-cols-[minmax(0,1fr)_auto_auto] sm:gap-4">
                    <span className="flex min-w-0 items-center gap-2 font-mono font-medium">
                      <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${request.status === 'OK' ? 'bg-emerald-400' : 'bg-danger'}`} aria-hidden="true" />
                      <span className="truncate" title={request.model}>{request.model}</span>
                    </span>
                    <span className="whitespace-nowrap tabular-nums">
                      <span className="text-orange-300">{fmt(request.input_tokens, lang)}↑</span>
                      <span className="mx-1 text-mute">/</span>
                      <span className="text-emerald-300">{fmt(request.output_tokens, lang)}↓</span>
                    </span>
                    <time dateTime={request.created_at} className="col-span-2 justify-self-end text-right text-xs text-mute tabular-nums sm:col-span-1 sm:whitespace-nowrap">{ago(request.created_at, lang)}</time>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <Link href="/api-keys" className="mt-auto pt-4 text-sm font-medium text-cyan-300 hover:text-cyan-200">{t('dash.manageKeys')}</Link>
        </div>

        <div className="card lift p-6">
          <span className="label">{t('dash.wallet')}</span>
          <span className="block mt-2 text-2xl font-semibold tabular-nums">{money(data?.balance ?? 0, lang)}</span>
          <p className="text-xs text-mute mt-1">{t('dash.walletP')}</p>
          <Link href="/wallet" className="text-sm text-cyan-300 hover:text-cyan-200">{t('dash.manageWallet')}</Link>
        </div>

        <div className="card lift p-6">
          <span className="label">{t('dash.store')}</span>
          {toko?.id ? (
            <>
              <span className="block mt-2 text-2xl font-semibold tabular-nums">{toko.products ?? 0}</span>
              <p className="text-xs text-mute mt-1">{t('dash.products', { n: toko.orders ?? 0 })}</p>
              <Link href="/toko" className="text-sm text-cyan-300 hover:text-cyan-200">{t('dash.manageStore')}</Link>
            </>
          ) : (
            <>
              <p className="text-sm text-mute mt-2">{t('dash.noStore')}</p>
              <Link href="/toko" className="text-sm text-cyan-300 hover:text-cyan-200">{t('dash.openStore')}</Link>
            </>
          )}
        </div>

        <div className="card lift p-6">
          <span className="label">{t('dash.aff')}</span>
          <span className="block mt-2 text-2xl font-semibold tabular-nums text-emerald-300">{money(aff?.earnings ?? 0, lang)}</span>
          <p className="text-xs text-mute mt-1 tabular-nums">{t('dash.affStat', { c: fmt(aff?.clicks, lang), v: fmt(aff?.conversions, lang) })}</p>
          <Link href="/afiliasi" className="text-sm text-cyan-300 hover:text-cyan-200">{t('dash.manageAff')}</Link>
        </div>

        {/* Pesanan terakhir: full width (Fanny #6) */}
        <div className="card lift p-6 md:col-span-2 xl:col-span-3">
          <div className="flex items-center justify-between gap-4 mb-4">
            <span className="label">{t('dash.orders')}</span>
            <Link href="/pesanan" className="text-sm text-cyan-300 hover:text-cyan-200">{t('dash.allOrders')}</Link>
          </div>
          {pesanan.length === 0 ? (
            <p className="text-sm text-mute">{t('dash.noOrders')}</p>
          ) : (
            <ul className="divide-y divide-border/60">
              {pesanan.map((o) => (
                <li key={o.id} className="py-3 flex items-center gap-4">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm truncate">{o.product_title}</p>
                    <p className="text-xs text-mute tabular-nums">
                      {o.quantity}× · {money(o.subtotal, lang)}{o.created_at && ` · ${tanggal(o.created_at, false, lang)}`}
                    </p>
                  </div>
                  <Badge>{statusLabel(o.status, lang)}</Badge>
                </li>
              ))}
            </ul>
          )}
        </div>
      </section>

      {/* Analytics menyatu di dashboard (dulu halaman /analytics terpisah).
          id="analytics" = target redirect /analytics -> /dashboard#analytics. */}
      <div className="mt-12 pt-10 border-t border-border">
        <AnalyticsSection />
      </div>
    </div>
  );
}
