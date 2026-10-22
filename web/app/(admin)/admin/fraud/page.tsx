'use client';

// Admin · Fraud: event velocity/pattern. Read-only + tandai sudah ditinjau.
// Keputusan ban tetap lewat /admin/users — jangan gandakan aksi di sini.

import { useState } from 'react';
import { api } from '@/lib/api';
import { useFetch } from '@/lib/useApi';
import { tanggal, asList } from '@/lib/utils';
import { ShieldAlert } from 'lucide-react';
import { Pagination, useClientPagination } from '@/components/ui/Pagination';
import { useT } from '@/lib/i18n';

type Event = {
  id: string; user_id: string; email?: string; kind: string; detail: string;
  ip?: string; is_reviewed: boolean; is_actioned: boolean; created_at?: string;
};

export default function AdminFraudPage() {
  const { data, loading, err, reload } = useFetch<unknown>('/admin/fraud');
  const events = asList<Event>(data, 'events');
  const pager = useClientPagination(events, 10);
  const { t } = useT();
  const [busy, setBusy] = useState('');
  const [actionErr, setActionErr] = useState('');

  const markReviewed = async (id: string) => {
    setBusy(id); setActionErr('');
    try { await api.post(`/admin/fraud/${id}/review`, {}); await reload(); }
    catch { setActionErr(t('ad.fr.reviewFail')); } finally { setBusy(''); }
  };

  return (
    <div className="p-6 md:p-10 max-w-5xl mx-auto">
      <header className="mb-8">
        <p className="label mb-1">{t('ad.common.admin')}</p>
        <h1 className="text-2xl font-semibold tracking-tight">{t('ad.fr.h')}</h1>
        <p className="text-sm text-mute mt-1">{t('ad.fr.sub')}</p>
      </header>

      {(err || actionErr) && <p role="alert" className="mb-6 text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{err || actionErr}</p>}

      {err ? null : loading ? (
        <div className="space-y-3">{[0, 1, 2, 3].map((i) => <div key={i} className="card p-4 h-16"><div className="skel h-4 w-64" /></div>)}</div>
      ) : events.length === 0 ? (
        <div className="card p-10 text-center">
          <ShieldAlert className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />
          <p className="font-medium">{t('ad.fr.empty')}</p>
          <p className="text-sm text-mute mt-1">{t('ad.fr.emptyP')}</p>
        </div>
      ) : (
        <>
          <ul className="card divide-y divide-border">
            {pager.items.map((e) => (
              <li key={e.id} className="p-4 flex flex-wrap items-start gap-3">
                <div className="min-w-0 flex-1">
                  <p className="text-sm">
                    <span className="badge badge-off mr-2">{e.kind}</span>
                    <span className="font-medium break-all">{e.email || t('ad.fr.noEmail')}</span>
                  </p>
                  <p className="text-sm text-mute mt-1 break-words">{e.detail}</p>
                  <p className="text-xs text-mute mt-1">
                    {e.ip ? `${e.ip} · ` : ''}{tanggal(e.created_at, true)}
                    {e.is_reviewed ? ` · ${t('ad.fr.reviewed')}` : ''}
                  </p>
                </div>
                {e.is_reviewed ? <span className="text-mute text-sm">—</span> : (
                  <button disabled={busy === e.id} className="btn btn-sm" onClick={() => markReviewed(e.id)}>
                    {t('ad.fr.mark')}
                  </button>
                )}
              </li>
            ))}
          </ul>
          {events.length > 10 && <Pagination page={pager.page} hasMore={pager.hasMore} onPage={pager.setPage} />}
        </>
      )}
    </div>
  );
}
