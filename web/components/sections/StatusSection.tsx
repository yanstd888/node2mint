'use client';
// Status: kesehatan platform (uptime monitor) + board per MODEL — availability
// 24 jam, latensi, dan spark bar 24 jam dari probe gateway ke provider.
// Uptime Kuma memantau endpoint HTTP; halaman ini yang tahu soal model.
import { useFetch } from '@/lib/useApi';
import { useAuth } from '@/lib/auth';
import { asList } from '@/lib/utils';
import { tanggalLang } from '@/lib/tanggal';
import { Activity, CheckCircle2, AlertTriangle, RotateCw } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useT, type Key } from '@/lib/i18n';

type Monitor = { name?: string; status?: string; uptime_pct?: number; uptimePct?: number; latency_ms?: number; checked_at?: string };
type ModelHealth = {
  name: string; type?: string; status?: string; availability?: number;
  avg_latency_ms?: number; p90_latency_ms?: number; checks?: number;
  last_checked?: string; spark?: string[];
};

export function StatusSection() {
  const { t, lang } = useT();
  const { user } = useAuth();
  const isAdmin = user?.role === 'ADMIN' || user?.role === 'SUPER_ADMIN';
  const status = useFetch<unknown>('/status', false, 600_000);
  const models = useFetch<unknown>('/models');
  const monitorData = useFetch<unknown>('/admin/monitors', !isAdmin);

  const monitors = asList<Monitor>(monitorData.data, 'monitors');
  const board = asList<ModelHealth>(status.data, 'models');
  const daftarModel = asList<{ name: string; type?: string; tier?: string; token_multiplier?: number }>(models.data, 'models');
  const ringkas = (status.data as { overall?: string; status?: string; gateway_live?: unknown } | null);
  const sehat = (ringkas?.overall ?? ringkas?.status ?? '').toUpperCase();
  const gatewayLive = ringkas?.gateway_live === true;
  const gatewayKnown = typeof ringkas?.gateway_live === 'boolean';
  const baik = sehat === 'OK' || sehat === 'UP' || sehat === 'OPERATIONAL';
  if (!status.loading && !status.err && gatewayKnown && !gatewayLive) return <><header className="mb-8"><p className="label mb-1">{t('st.kicker')}</p><h1 className="text-2xl font-semibold tracking-tight">Service Status</h1></header><section className="rounded-xl border border-gold/30 bg-gold/10 p-6 text-sm text-gold">API gateway sedang dalam tahap persiapan atau pemeliharaan. Katalog model akan tampil setelah layanan siap.</section></>;
  const diukur = board.filter((m) => (m.checks ?? 0) > 0);
  const rata = diukur.length
    ? diukur.reduce((t, m) => t + (m.availability ?? 0), 0) / diukur.length
    : null;

  return (
    <>
      <header className="mb-8 flex items-start justify-between gap-4">
        <div>
          <p className="label mb-1">{t('st.kicker')}</p>
          <h1 className="text-2xl font-semibold tracking-tight">{t('st.h')}</h1>
        </div>
        <Button variant="outline" size="sm" onClick={() => status.reload()} disabled={status.loading} className="shrink-0">
          <RotateCw className={`w-3.5 h-3.5 ${status.loading ? 'animate-spin' : ''}`} aria-hidden="true" />
          {t('st.reload')}
        </Button>
      </header>

      <section
        aria-label={t('st.summary')}
        className={`mb-6 rounded-xl border px-5 py-3 flex items-center gap-3 ${
          status.err ? 'border-danger/30 bg-danger/10'
            : baik ? 'border-emerald-400/30 bg-emerald-500/10'
            : 'border-gold/30 bg-gold/10'
        }`}
      >
        {baik ? <CheckCircle2 className="w-5 h-5 text-emerald-300 shrink-0" aria-hidden="true" />
          : sehat ? <AlertTriangle className="w-5 h-5 text-gold shrink-0" aria-hidden="true" />
          : <Activity className="w-5 h-5 text-mute shrink-0" aria-hidden="true" />}
        <p className="text-sm font-medium">
          {status.err ? t('err.load')
            : status.loading ? t('st.checking')
            : baik ? t('st.allOk')
            : labelStatus(sehat, t)}
        </p>
      </section>

      <section aria-label={t('st.numbers')} className="grid grid-cols-2 gap-3 mb-10">
        <div className="card lift p-4">
          <p className="text-xs text-mute">{t('st.avg')}</p>
          <p className="mt-1 text-2xl font-semibold tabular-nums">
            {rata === null ? <span className="text-mute">-</span> : `${rata.toFixed(2)}%`}
          </p>
        </div>
        <div className="card lift p-4">
          <p className="text-xs text-mute">{t('st.watched')}</p>
          <p className="mt-1 text-2xl font-semibold tabular-nums">
            {diukur.length} / {board.length}
          </p>
        </div>
      </section>

      <section aria-label={t('st.board')} className="mb-10">
        <h2 className="text-lg font-semibold tracking-tight mb-4">{t('st.board')}</h2>
        {status.err ? null : status.loading ? (
          <div className="space-y-3">{[0, 1, 2].map((i) => <div key={i} className="card p-4 h-24"><div className="skel h-4 w-40" /></div>)}</div>
        ) : board.length === 0 ? (
          <p className="card p-6 text-sm text-mute text-center">{t('st.noBoard')}</p>
        ) : (
          <ul className="space-y-3">
            {board.map((m) => (
              <li key={m.name} className="card lift p-4">
                <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
                  <div className="flex items-center gap-3 min-w-0">
                    <Titik status={m.status} />
                    <span className="text-sm font-medium font-mono truncate">{m.name}</span>
                    <Badge>{labelStatus(m.status, t)}</Badge>
                  </div>
                  {(m.availability !== undefined || m.p90_latency_ms !== undefined || m.avg_latency_ms !== undefined) && (
                    <div className="text-xs text-mute tabular-nums flex flex-wrap gap-x-4">
                      {m.availability !== undefined && <span title={t('tip.avail')}>{t('st.avail')}: {m.availability.toFixed(2)}%</span>}
                      {m.p90_latency_ms !== undefined && <span title={t('tip.p90')}>P90: {m.p90_latency_ms} ms</span>}
                      {m.avg_latency_ms !== undefined && <span title={t('tip.p90')}>{t('st.avgL')}: {m.avg_latency_ms} ms</span>}
                    </div>
                  )}
                </div>
                <Spark nilai={m.spark} />
                <p className="mt-2 text-xs text-mute">
                  {m.checks ? t('st.checks', { n: m.checks }) : t('st.noChecks')}
                  {m.last_checked && ` · ${t('st.last', { date: tanggalLang(m.last_checked, lang, true) })}`}
                </p>
              </li>
            ))}
          </ul>
        )}
      </section>

      {isAdmin && !monitorData.err && (
        <section aria-label={t('st.monitors')} className="mb-10">
          <h2 className="text-lg font-semibold tracking-tight mb-4">{t('st.monitors')}</h2>
          {monitorData.loading ? (
            <div className="space-y-3">{[0, 1].map((i) => <div key={i} className="card p-4 h-16"><div className="skel h-4 w-48" /></div>)}</div>
          ) : monitors.length === 0 ? (
            <p className="card p-6 text-sm text-mute text-center">{t('st.noMonitors')}</p>
          ) : (
            <ul className="card divide-y divide-border">
              {monitors.map((m, i) => (
                <li key={m.name ?? i} className="p-4 flex items-center justify-between gap-4">
                  <div className="min-w-0">
                    <p className="text-sm truncate">{m.name || t('st.service')}</p>
                    <p className="text-xs text-mute mt-1">
                      {labelStatus(m.status, t)}
                      {m.latency_ms !== undefined && ` · ${m.latency_ms} ms`}
                      {m.checked_at && ` · ${tanggalLang(m.checked_at, lang, true)}`}
                    </p>
                  </div>
                  <Uptime nilai={m.uptime_pct ?? m.uptimePct} />
                </li>
              ))}
            </ul>
          )}
        </section>
      )}

      <section aria-label={t('st.catalog')}>
        <h2 className="text-lg font-semibold tracking-tight mb-4">{t('st.catalog')}</h2>
        {models.err ? (
          <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{t('err.load')}</p>
        ) : models.loading ? (
          <div className="space-y-3">{[0, 1, 2].map((i) => <div key={i} className="card p-4 h-14"><div className="skel h-4 w-40" /></div>)}</div>
        ) : daftarModel.length === 0 ? (
          <p className="card p-6 text-sm text-mute text-center">{t('st.none')}</p>
        ) : (
          <ul className="card divide-y divide-border">
            {daftarModel.map((m) => (
              <li key={m.name} className="p-4 flex items-center justify-between gap-4">
                <div className="min-w-0">
                  <p className="text-sm font-mono truncate">{m.name}</p>
                  <p className="text-xs text-mute mt-1">{m.type || t('st.unk')}</p>
                </div>
                {m.tier && (
                  <span className="flex items-center gap-2 shrink-0">
                    <Badge tone={m.tier === 'PAID' ? 'pending' : 'netral'}>{m.tier === 'PAID' ? t('models.paidTag') : m.tier === 'FREE' ? t('models.freeTag') : m.tier}</Badge>
                    {typeof m.token_multiplier === 'number' && m.token_multiplier !== 1 && (
                      <span className="text-[11px] text-mute tabular-nums" title={t('tip.mult')}>×{m.token_multiplier}</span>
                    )}
                  </span>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>
    </>
  );
}

// Spark: 24 batang = 24 jam terakhir (paling kanan = jam ini). Jam tanpa probe
// dibiarkan kosong dan diberi keterangan — strip penuh warna gelap akan terbaca
// seperti "24 jam mati", padahal artinya belum diperiksa.
const warna: Record<string, string> = {
  ok: 'bg-emerald-400',
  slow: 'bg-gold',
  error: 'bg-orange-400',
  down: 'bg-danger',
};

function Spark({ nilai }: { nilai?: string[] }) {
  const jam = nilai && nilai.length ? nilai : Array(24).fill('');
  const adaData = jam.some((s) => s);
  return (
    <div className="mt-3 flex items-end gap-[2px]" aria-hidden="true">
      {jam.map((s, i) => (
        <span key={i} className={`h-5 flex-1 rounded-sm ${warna[s] ?? (adaData ? 'bg-border/70' : 'bg-border/30')}`} />
      ))}
    </div>
  );
}

function Titik({ status }: { status?: string }) {
  const c = warna[status ?? ''] ?? 'bg-mute';
  return <span className={`w-2 h-2 rounded-full shrink-0 ${c}`} aria-hidden="true" />;
}

function labelStatus(s: string | undefined, t: (k: Key) => string) {
  switch ((s ?? '').toLowerCase()) {
    case 'ok':
    case 'up':
    case 'healthy':
    case 'operational': return t('st.ok');
    case 'slow':
    case 'degraded':
    case 'limited': return t('st.slow');
    case 'error':
    case 'failed': return t('st.error');
    case 'down':
    case 'outage': return t('st.down');
    default: return t('st.unknown');
  }
}

// Uptime hanya punya arti kalau ada pemeriksaan; tanpa data tampil "-", bukan 0%.
function Uptime({ nilai }: { nilai: unknown }) {
  const n = Number(nilai);
  if (!Number.isFinite(n)) return <span className="text-sm text-mute shrink-0 tabular-nums">-</span>;
  const tone = n >= 99 ? 'text-cyan-300' : n >= 95 ? 'text-gold' : 'text-danger';
  return <span className={`text-sm shrink-0 tabular-nums ${tone}`}>{n.toFixed(2)}%</span>;
}
