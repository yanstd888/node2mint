'use client';

import { useFetch } from '@/lib/useApi';
import { asList, tanggal } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { useT } from '@/lib/i18n';

type Log = { id: string; model?: string; provider?: string; input?: number; output?: number; cached?: number; cost?: number; latency?: number; status?: string; error?: string; created?: string };

export default function AdminRequestsPage() {
  const { t } = useT();
  const { data, loading, err } = useFetch<unknown>('/admin/requests?limit=100');
  const logs = asList<Log>(data, 'logs');
  return <div className="mx-auto max-w-6xl p-6 md:p-10">
    <header className="mb-8"><p className="label mb-1">{t('ad.common.admin')}</p><h1 className="text-2xl font-semibold tracking-tight">API requests</h1><p className="mt-2 text-sm text-mute">Metadata only; prompt and response content are never stored.</p></header>
    {err && <p role="alert" className="banner banner-bad">{err}</p>}
    {loading ? <div className="card p-6">Loading…</div> : logs.length === 0 ? <div className="card p-8 text-center text-mute">No request records yet.</div> : <div className="tbl-wrap"><table className="w-full text-sm"><thead><tr className="tbl-head"><th>Time</th><th>Model</th><th>Provider</th><th>Tokens</th><th>Cost</th><th>Latency</th><th>Status</th><th>Error</th></tr></thead><tbody>{logs.map((x) => <tr key={x.id} className="border-t border-border"><td className="px-4 py-3 whitespace-nowrap">{tanggal(x.created, true)}</td><td className="px-4 py-3">{x.model || '-'}</td><td className="px-4 py-3">{x.provider || '-'}</td><td className="px-4 py-3 tabular-nums">{(x.input ?? 0) + (x.output ?? 0)}</td><td className="px-4 py-3 tabular-nums">{x.cost ?? 0}</td><td className="px-4 py-3 tabular-nums">{x.latency ?? 0} ms</td><td className="px-4 py-3"><Badge>{x.status || '-'}</Badge></td><td className="px-4 py-3 text-danger">{x.error || ''}</td></tr>)}</tbody></table></div>}
  </div>;
}
