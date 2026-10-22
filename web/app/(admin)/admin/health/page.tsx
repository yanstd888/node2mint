'use client';
// Admin · Health model: status tiap model/provider (sehat, waktu cek terakhir, error rate).
import { useFetch } from '@/lib/useApi';
import { tanggal, asList } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { HeartPulse } from 'lucide-react';
import { useT } from '@/lib/i18n';

type Baris = {
  id?: string; name?: string; model?: string; provider?: string;
  is_healthy?: boolean; healthy?: boolean; error_rate?: number; latency_ms?: number; latency?: number;
  last_checked?: string; last_check?: string; detail?: string; message?: string; status?: string;
};

const statusLabel = (status: string | undefined, t: ReturnType<typeof useT>['t']) => {
  switch ((status ?? '').toUpperCase()) {
    case 'OK':
    case 'UP':
    case 'HEALTHY':
    case 'OPERATIONAL': return t('st.ok');
    case 'SLOW':
    case 'DEGRADED':
    case 'LIMITED': return t('st.slow');
    case 'DOWN':
    case 'OUTAGE': return t('st.down');
    case 'ERROR': return t('st.error');
    default: return t('st.unknown');
  }
};

export default function AdminHealthPage() {
  const { data, loading, err } = useFetch<unknown>('/admin/health');
  // Gateway balas {checked, logs: [...]}, lama dibungkus bentuk lain — asList menoleran keduanya.
  const baris = asList<Baris>(data, 'logs', 'models', 'providers', 'health');
  const { t } = useT();

  return (
    <div className="p-6 md:p-10 max-w-4xl mx-auto">
      <header className="mb-8">
        <p className="label mb-1">{t('ad.common.admin')}</p>
        <h1 className="text-2xl font-semibold tracking-tight">{t('ad.hl.h')}</h1>
      </header>

      {err && <p role="alert" className="mb-6 text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{err}</p>}

      {err ? null : loading ? (
        <div className="space-y-3">{[0, 1, 2].map((i) => <div key={i} className="card p-4 h-16"><div className="skel h-4 w-48" /></div>)}</div>
      ) : baris.length === 0 ? (
        <div className="card p-10 text-center">
          <HeartPulse className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />
          <p className="font-medium">{t('ad.hl.empty')}</p>
          <p className="text-sm text-mute mt-1">{t('ad.hl.emptyP')}</p>
        </div>
      ) : (
        <ul className="card divide-y divide-border">
          {baris.map((b, i) => {
            const sehat = b.is_healthy ?? b.healthy;
            const badge = sehat ? t('ad.hl.healthy') : sehat === false ? t('ad.hl.bad') : statusLabel(b.status, t);
            return (
              <li key={b.id ?? `${b.name ?? b.model ?? i}`} className="p-4 flex items-center justify-between gap-4">
                <div className="min-w-0">
                  <p className="text-sm font-mono truncate">{b.name || b.model || t('ad.md.thModel')}</p>
                  <p className="text-xs text-mute mt-1 truncate">
                    {b.provider || t('ad.hl.noProvider')}
                    {(b.latency ?? b.latency_ms) !== undefined && ` · ${b.latency ?? b.latency_ms} ms`}
                    {(b.last_checked ?? b.last_check) && t('ad.hl.last', { date: tanggal(b.last_checked ?? b.last_check, true) })}
                  </p>
                  {(b.detail || b.message) && <p className="text-xs text-mute mt-1 truncate">{b.detail || b.message}</p>}
                </div>
                <div className="text-right shrink-0">
                  <Badge>{badge}</Badge>
                  {b.error_rate !== undefined && (
                    <p className="text-xs text-mute mt-1 tabular-nums">{t('ad.hl.errorRate', { n: Number(b.error_rate).toFixed(1) })}</p>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
