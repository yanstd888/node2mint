'use client';

// Analytics: satu implementasi, dipakai di dalam /dashboard (#analytics).

// Range (24h/7d/30d) + scope (user / seller / platform-admin).

// Grafik bar batang CSS murni dari `series` - tanpa library chart.

import { useEffect, useState } from 'react';

import { useFetch } from '@/lib/useApi';

import { useAuth } from '@/lib/auth';

import { useT, type Key } from '@/lib/i18n';

import { rupiah, angka } from '@/lib/utils';

import { Store, User as UserIcon, Building2 } from 'lucide-react';



type Sel = { day: string; value: number };

type ModelRow = { model: string; requests: number; cost: number; tokens?: number };

type Overview = {

  series?: Sel[] | { signups?: Sel[]; requests?: Sel[]; tokens?: Sel[] };

  models?: ModelRow[];

  // seller

  products?: { id: string; title: string; views: number; price: number; units: number; revenue: number; cart_adds?: number }[];

  revenue?: number; views?: number; sales?: number; cart_adds?: number; conversion?: number;

  // platform

  totals?: { signups?: number; activeUsers?: number; requests?: number; inputTokens?: number;

    outputTokens?: number; aiCost?: number; revenue?: number; revenueCount?: number;

    orderRevenue?: number; orderCount?: number };

  prev?: { aiCost?: number; revenue?: number };

  funnel?: { step: string; count: number }[];

};



const RANGES = ['24h', '7d', '30d'] as const;

// Nama langkah funnel dikirim gateway dalam bahasa Indonesia -> dipetakan ke kunci kamus.

const FUNNEL_KEY: Record<string, Key> = { 'Daftar': 'an.fDaftar', 'Top-up': 'an.fTopup', 'Pakai API': 'an.fPakaiAPI' };

type Range = (typeof RANGES)[number];



export default function AnalyticsSection() {

  const { user } = useAuth();

  const { t } = useT();

  const isAdmin = user?.role === 'ADMIN' || user?.role === 'SUPER_ADMIN';

  // Scope tersedia sesuai peran: semua lihat Sendiri, penjual Toko, admin Platform.

  const scopes = [

    { id: 'user', label: t('an.sendiri'), icon: UserIcon },

    { id: 'seller', label: t('an.toko'), icon: Store },

    ...(isAdmin ? [{ id: 'platform', label: t('an.platform'), icon: Building2 }] : []),

  ];

  const [range, setRange] = useState<Range>('7d');

  const [scope, setScope] = useState('user');

  const { data, loading, err } = useFetch<{ overview?: Overview }>(

    `/analytics?range=${range}&scope=${scope}`);

  const o = data?.overview;



  // Scroll ke anchor #analytics. Next hanya menangani hash pada navigasi client-side,

  // dan di sini pun elemennya belum ada saat hash awal dibaca (halaman render setelah

  // auth), jadi tanpa effect ini /analytics mendarat di atas dashboard.

  useEffect(() => {

    const ke = () => { if (location.hash === '#analytics') document.getElementById('analytics')?.scrollIntoView(); };

    ke();

    window.addEventListener('hashchange', ke);

    return () => window.removeEventListener('hashchange', ke);

  }, []);



  return (

    // id + scroll-mt: /analytics redirect ke /dashboard#analytics dan mendarat di sini.

    // h2 (bukan h1): bagian dari halaman dashboard, di bawah h1 "Halo, ...".

    <section id="analytics" aria-labelledby="analytics-judul" className="scroll-mt-6">

      <header className="mb-6 flex flex-wrap items-start justify-between gap-4">

        <div>

          <p className="label mb-1">{t('an.data')}</p>

          <h2 id="analytics-judul" className="text-lg font-semibold tracking-tight">{t('an.title')}</h2>

        </div>

        {/* Pilih rentang */}

        <div className="tab-pills" role="tablist" aria-label={t('an.range')}>

          {RANGES.map((r) => (

            <button key={r} role="tab" aria-selected={range === r} onClick={() => setRange(r)} className="tab-pill">

              {r}

            </button>

          ))}

        </div>

      </header>



      <div className="tab-pills mb-8" role="tablist" aria-label={t('an.scope')}>

        {scopes.map((s) => (

          <button key={s.id} role="tab" aria-selected={scope === s.id} onClick={() => setScope(s.id)} className="tab-pill">

            <s.icon className="w-4 h-4 inline-block mr-2 -mt-1" aria-hidden="true" />{s.label}

          </button>

        ))}

      </div>



      {err ? (

        <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">

          {scope === 'seller' ? t('an.noStore') : err}

        </p>

      ) : loading || !o ? (

        <div className="card h-44 p-5" aria-busy="true"><div className="skel h-5 w-32" /></div>

      ) : scope === 'platform' ? (

        <PlatformView o={o} />

      ) : scope === 'seller' ? (

        <SellerView o={o} />

      ) : (

        <UserView o={o} />

      )}

    </section>

  );

}



function Kartu({ label, nilai }: { label: string; nilai: string }) {

  return (

    <div className="card lift p-5">

      <span className="label">{label}</span>

      <span className="block mt-2 text-lg sm:text-xl font-semibold tabular-nums">{nilai}</span>

    </div>

  );

}



// Grafik bar batang CSS murni: tinggi tiap batang = value / max * 100%.

function Bars({ series, warna = 'bg-cyan-400/70' }: { series?: Sel[]; warna?: string }) {

  const { t } = useT();

  const max = Math.max(1, ...(series ?? []).map((s) => Number(s.value) || 0));

  return (

    <div className="flex items-end gap-1 h-32" role="img" aria-label={t('an.chartDaily')}>

      {(series ?? []).map((s, i) => (

        <div key={s.day} title={`${s.day}: ${angka(s.value)}`}

          className="bar-col flex-1 h-full flex flex-col justify-end min-w-0" style={{ animationDelay: `${Math.min(i * 40, 200)}ms` }}>

          <div className={`rounded-t-sm ${warna}`} style={{ height: `${(Math.max(0, Number(s.value) || 0) / max) * 100}%`, minHeight: Number(s.value) > 0 ? 2 : 0, animationDelay: `${Math.min(i * 40, 200)}ms` }} />

        </div>

      ))}

      {(!series || series.length === 0) && <p className="text-sm text-mute">{t('an.noData')}</p>}

    </div>

  );

}



function UsageAreaChart({ series }: { series?: { requests?: Sel[]; tokens?: Sel[] } }) {

  const { t } = useT();

  const [metric, setMetric] = useState<'tokens' | 'requests'>('tokens');

  const data = series?.[metric] ?? [];

  const max = Math.max(1, ...data.map((point) => Number(point.value) || 0));

  const left = 64, right = 990, top = 12, bottom = 210;

  const points = data.map((point, index) => {

    const x = data.length === 1 ? left : left + (index / (data.length - 1)) * (right - left);

    const y = bottom - ((Number(point.value) || 0) / max) * (bottom - top);

    return { ...point, x, y };

  });

  const pointString = points.map((point) => `${point.x},${point.y}`).join(' ');

  const labelEvery = Math.max(1, Math.ceil(data.length / 7));



  return (

    <section className="card p-5 md:p-6" aria-label={t('an.reqsPerDay')}>

      <div className="tab-pills mb-5 w-fit" role="group" aria-label={t('an.reqsPerDay')}>

        {(['tokens', 'requests'] as const).map((value) => (

          <button key={value} type="button" aria-pressed={metric === value}

            onClick={() => setMetric(value)} className="tab-pill">

            {value === 'tokens' ? t('an.tokens') : t('an.reqs')}

          </button>

        ))}

      </div>



      {data.length === 0 ? (

        <p className="py-20 text-center text-sm text-mute">{t('an.noData')}</p>

      ) : (

        <svg viewBox="0 0 1000 250" className="h-56 w-full overflow-visible" role="img" aria-label={`${metric}: ${angka(data.reduce((sum, point) => sum + Number(point.value || 0), 0))}`}>

          <defs>

            <linearGradient id="usage-area" x1="0" y1="0" x2="0" y2="1">

              <stop offset="0%" stopColor="#6567f1" stopOpacity="0.28" />

              <stop offset="100%" stopColor="#6567f1" stopOpacity="0.02" />

            </linearGradient>

          </defs>

          {[0, 0.25, 0.5, 0.75, 1].map((part) => {

            const y = bottom - part * (bottom - top);

            return (

              <g key={part}>

                <line x1={left} y1={y} x2={right} y2={y} stroke="currentColor" className="text-border" strokeDasharray="3 4" />

                <text x={left - 10} y={y + 4} textAnchor="end" className="fill-current text-mute text-[11px]">{compact(max * part)}</text>

              </g>

            );

          })}

          {points.map((point, index) => index % labelEvery === 0 || index === points.length - 1 ? (

            <g key={point.day}>

              <line x1={point.x} y1={top} x2={point.x} y2={bottom} stroke="currentColor" className="text-border/60" strokeDasharray="3 4" />

              <text x={point.x} y={235} textAnchor={index === 0 ? 'start' : index === points.length - 1 ? 'end' : 'middle'} className="fill-current text-mute text-[11px]">

                {new Date(`${point.day}T00:00:00`).toLocaleDateString('id-ID', { day: 'numeric', month: 'short' })}

              </text>

            </g>

          ) : null)}

          <polygon points={`${left},${bottom} ${pointString} ${right},${bottom}`} fill="url(#usage-area)" />

          <polyline points={pointString} fill="none" stroke="#6567f1" strokeWidth="3" strokeLinejoin="round" strokeLinecap="round" />

          {points.map((point) => <circle key={point.day} cx={point.x} cy={point.y} r="3" fill="#6567f1"><title>{point.day}: {angka(point.value)}</title></circle>)}

        </svg>

      )}

    </section>

  );

}



const MODEL_COLORS = ['#5b5bd6', '#1f9d93', '#df9412', '#d95c59', '#8f6ad7'];

const compact = (n: number) => new Intl.NumberFormat('en', { notation: 'compact', maximumFractionDigits: 1 }).format(n);



function DaftarModel({ models }: { models?: ModelRow[] }) {

  const { t } = useT();

  const [metric, setMetric] = useState<'tokens' | 'requests'>('tokens');

  const rows = [...(models ?? [])]

    .sort((a, b) => Number(b[metric] ?? 0) - Number(a[metric] ?? 0))

    .slice(0, 5);

  const max = Math.max(1, ...rows.map((m) => Number(m[metric] ?? 0)));



  return (

    <section aria-label={t('an.modelsAria')} className="card p-5 md:p-6">

      <div className="mb-6 flex items-center justify-between gap-4">

        <h3 className="label text-sm text-cyan-200">{t('an.topModels')}</h3>

        <div className="tab-pills" role="group" aria-label={t('an.modelsAria')}>

          {(['tokens', 'requests'] as const).map((value) => (

            <button key={value} type="button" aria-pressed={metric === value}

              onClick={() => setMetric(value)} className="tab-pill">

              {value === 'tokens' ? t('an.tokens') : t('an.reqs')}

            </button>

          ))}

        </div>

      </div>



      {rows.length === 0 ? (

        <p className="py-16 text-center text-sm text-mute">{t('an.noModelUse')}</p>

      ) : (

        <div className="grid grid-cols-[minmax(6rem,10rem)_minmax(0,1fr)] gap-x-3 gap-y-3">

          {rows.map((m, index) => {

            const value = Number(m[metric] ?? 0);

            return (

              <div key={m.model} className="contents">

                <span className="self-center truncate text-right font-mono text-sm text-text" title={m.model}>{m.model}</span>

                <div className="relative h-7 overflow-hidden border-l border-border" title={`${m.model}: ${angka(value)} ${metric}`}>

                  {[25, 50, 75, 100].map((tick) => (

                    <span key={tick} aria-hidden="true" className="absolute inset-y-0 border-l border-dashed border-border/60" style={{ left: `${tick}%` }} />

                  ))}

                  <span className="relative block h-full rounded-r-sm transition-[width] duration-300"

                    style={{ width: `${Math.max(value > 0 ? 1 : 0, (value / max) * 100)}%`, backgroundColor: MODEL_COLORS[index % MODEL_COLORS.length] }} />

                </div>

              </div>

            );

          })}

          <span aria-hidden="true" />

          <div className="grid grid-cols-5 text-xs tabular-nums text-mute">

            {[0, 0.25, 0.5, 0.75, 1].map((part) => (

              <span key={part} className={part === 0 ? 'text-left' : part === 1 ? 'text-right' : 'text-center'}>{compact(max * part)}</span>

            ))}

          </div>

        </div>

      )}

    </section>

  );

}



function UserView({ o }: { o: Overview }) {

  return (

    <>

      <UsageAreaChart series={o.series as { requests?: Sel[]; tokens?: Sel[] }} />

      <div className="mt-8"><DaftarModel models={o.models} /></div>

    </>

  );

}



function SellerView({ o }: { o: Overview }) {

  const { t } = useT();

  return (

    <>

      <section className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-4 mb-8">

        <Kartu label={t('an.revenue')} nilai={rupiah(o.revenue)} />

        <Kartu label={t('an.views')} nilai={angka(o.views)} />

        <Kartu label={t('an.inCarts')} nilai={angka(o.cart_adds)} />

        <Kartu label={t('an.sold')} nilai={t('an.units', { n: angka(o.sales) })} />

        <Kartu label={t('an.conv')} nilai={`${Number(o.conversion ?? 0).toFixed(1)}%`} />

      </section>

      <section aria-label={t('an.topProducts')} className="card p-5">

        <h3 className="font-semibold mb-4">{t('an.topProducts')}</h3>

        {(o.products ?? []).length === 0 ? (

          <p className="text-sm text-mute">{t('an.noProducts')}</p>

        ) : (

          <ul className="divide-y divide-border">

            {o.products!.map((p) => (

              <li key={p.id} className="py-3 flex flex-wrap items-center justify-between gap-2 text-sm">

                <span className="font-medium truncate">{p.title}</span>

                <span className="text-mute tabular-nums shrink-0">

                  {t('an.viewedSold', { v: angka(p.views), s: angka(p.units) })} &middot; {t('an.cartCount', { n: angka(p.cart_adds) })} &middot; {rupiah(p.revenue)}

                </span>

              </li>

            ))}

          </ul>

        )}

      </section>

    </>

  );

}



function PlatformView({ o }: { o: Overview }) {

  const { t: tr } = useT(); // `t` sudah terpakai untuk totals

  const t = o.totals ?? {};

  const series = o.series as { signups?: Sel[]; requests?: Sel[] } | undefined;

  return (

    <>

      <section className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">

        <Kartu label={tr('an.signups')} nilai={angka(t.signups)} />

        <Kartu label={tr('an.activeUsers')} nilai={angka(t.activeUsers)} />

        <Kartu label={tr('an.reqs')} nilai={angka(t.requests)} />

        <Kartu label={tr('an.aiCost')} nilai={rupiah(t.aiCost)} />

        <Kartu label={tr('an.topupRevenue')} nilai={rupiah(t.revenue)} />

        <Kartu label={tr('an.topupTx')} nilai={angka(t.revenueCount)} />

        <Kartu label={tr('an.orderGmv')} nilai={rupiah(t.orderRevenue)} />

        <Kartu label={tr('an.ordersCard')} nilai={angka(t.orderCount)} />

      </section>



      <section className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">

        <div className="card p-5" aria-label={tr('an.reqsPerDay')}>

          <h3 className="font-semibold mb-4">{tr('an.reqsPerDay')}</h3>

          <Bars series={series?.requests} />

        </div>

        <div className="card p-5" aria-label={tr('an.signupsPerDay')}>

          <h3 className="font-semibold mb-4">{tr('an.signupsPerDay')}</h3>

          <Bars series={series?.signups} warna="bg-gold/70" />

        </div>

      </section>



      <section aria-label={tr('an.funnel')} className="card p-5 mb-8">

        <h3 className="font-semibold mb-4">{tr('an.funnel')}</h3>

        <ul className="space-y-3">

          {(o.funnel ?? []).map((f, i, arr) => {

            const maks = Math.max(1, ...arr.map((x) => Number(x.count) || 0));

            const lebar = 100 - i * (arr.length > 1 ? 42 / (arr.length - 1) : 0);

            return (

              <li key={f.step} className="funnel-step" style={{ width: `${lebar}%` }}>

                <div className="flex items-center justify-between text-sm mb-1">

                  <span>{tr(FUNNEL_KEY[f.step] ?? f.step)}</span>

                  <span className="tabular-nums text-mute">{angka(f.count)}</span>

                </div>

                <div className="h-2 rounded-full bg-surface-hi overflow-hidden">

                  <div className="credit-fill" style={{ width: `${(Math.max(0, Number(f.count) || 0) / maks) * 100}%` }} />

                </div>

              </li>

            );

          })}

        </ul>

      </section>



      <DaftarModel models={o.models} />

    </>

  );

}

