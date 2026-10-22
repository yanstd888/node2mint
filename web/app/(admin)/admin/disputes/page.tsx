'use client';
// Admin · Sengketa: daftar dispute + resolve (buyer_wins / seller_wins / manual_review).
import { useState } from 'react';
import { post, pesanError } from '@/lib/http';
import { useFetch } from '@/lib/useApi';
import { useToast } from '@/components/ui/toast';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Dialog } from '@/components/ui/dialog';
import { rupiah, tanggal, asList } from '@/lib/utils';
import { Pagination, useClientPagination } from '@/components/ui/Pagination';
import { useT } from '@/lib/i18n';

type Dispute = {
  id: string;
  order_id: string;
  buyer_id: string;
  store_id?: string;
  reason: string;
  status: string;
  resolution?: string;
  seller_response?: string;
  seller_response_at?: string;
  created_at: string;
  subtotal?: number;
  buyer_email?: string;
  store_name?: string;
};

const RESOLVED = new Set(['BUYER_WINS', 'MANUAL_REVIEW', 'RESOLVED']);
const statusLabel = (status: string, t: ReturnType<typeof useT>['t']) => {
  switch (status) {
    case 'OPEN': return t('dispute.statusOpen');
    case 'RESOLVED': return t('dispute.statusResolved');
    case 'BUYER_WINS': return t('dispute.statusBuyerWins');
    case 'SELLER_WINS': return t('dispute.statusSellerWins');
    case 'MANUAL_REVIEW': return t('dispute.statusManual');
    default: return status;
  }
};

export default function AdminDisputesPage() {
  const { t } = useT();
  const [q, setQ] = useState('OPEN');
  const { data, loading, err, reload } = useFetch<{ has_more?: boolean }>(`/admin/disputes?status=${q}`);
  const disputes = asList<Dispute>(data, 'disputes');
  const page = useClientPagination(disputes, 10);
  const toast = useToast();
  const [busy, setBusy] = useState<string | null>(null);
  const [resolveOpen, setResolveOpen] = useState<Dispute | null>(null);
  const [decision, setDecision] = useState<'buyer' | 'seller'>('buyer');
  const [note, setNote] = useState('');

  const resolve = async () => {
    if (!resolveOpen) return;
    setBusy(resolveOpen.id);
    try {
      await post(`/admin/disputes/${resolveOpen.id}/resolve`,
        { buyer_wins: decision === 'buyer', resolution: note });
      toast.ok(t('ad.dp.resolved'));
      setResolveOpen(null);
      setNote('');
      await reload();
    } catch (e) {
      toast.err(pesanError(e, t('ad.dp.resolveFail')));
    } finally {
      setBusy(null);
    }
  };

  const openResolve = (d: Dispute) => {
    setResolveOpen(d);
    setDecision('buyer');
    setNote('');
  };

  return (
    <div className="p-6 md:p-10 max-w-5xl mx-auto">
      <header className="mb-8">
        <p className="label mb-1">{t('ad.common.admin')}</p>
        <div className="flex items-center justify-between gap-4 flex-wrap">
          <h1 className="text-2xl font-semibold tracking-tight">{t('ad.dp.h')}</h1>
          <div role="group" aria-label={t('ad.dp.filter')} className="flex rounded-lg border border-border p-1">
            <button
              type="button"
              aria-pressed={q === 'OPEN'}
              onClick={() => setQ('OPEN')}
              className={`px-3 py-1 text-xs font-medium rounded ${q === 'OPEN' ? 'bg-cyan-500/15 text-cyan-300' : 'text-mute hover:text-text'}`}
            >
              {t('ad.dp.filterOpen')}
            </button>
            <button
              type="button"
              aria-pressed={q === ''}
              onClick={() => setQ('')}
              className={`px-3 py-1 text-xs font-medium rounded ${q === '' ? 'bg-cyan-500/15 text-cyan-300' : 'text-mute hover:text-text'}`}
            >
              {t('ad.dp.filterAll')}
            </button>
          </div>
        </div>
      </header>

      {err && (
        <p role="alert" className="mb-6 text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">
          {err}
        </p>
      )}

      {err ? null : loading ? (
        <div className="space-y-3">
          {[0, 1, 2].map((i) => (
            <div key={i} className="card p-4 h-20">
              <div className="skel h-4 w-56 mb-2" />
              <div className="skel h-3 w-40" />
            </div>
          ))}
        </div>
      ) : disputes.length === 0 ? (
        <p className="card p-8 text-center text-sm text-mute">{t('ad.dp.empty')}</p>
      ) : (
        <>
          <ul className="space-y-3">
            {page.items.map((d) => (
              <li key={d.id} className="card p-4 flex flex-wrap items-center justify-between gap-4">
                <div className="min-w-0">
                  <p className="text-sm font-medium truncate">{d.reason}</p>
                  <p className="text-xs text-mute mt-1 flex items-center gap-2 flex-wrap">
                    <span>{d.buyer_email || d.buyer_id?.slice(0, 8) || '-'}</span>
                    &middot; <Badge>{statusLabel(d.status, t)}</Badge>
                    &middot; <span>{rupiah(d.subtotal)}</span>
                    &middot; {tanggal(d.created_at, true)}
                  </p>
                  {d.store_name && <p className="text-xs text-mute mt-1">{t('ad.dp.store')}: {d.store_name}</p>}
                  {d.seller_response && <p className="text-xs text-mute mt-1"><span className="font-medium text-text">{t('ad.dp.sellerResponse')}:</span> {d.seller_response}</p>}
                  {d.seller_response_at && <p className="text-xs text-mute mt-1">{t('ad.dp.respondedAt', { date: tanggal(d.seller_response_at, true) })}</p>}
                  {d.resolution && <p className="text-xs text-mute mt-1">{d.resolution}</p>}
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  <Badge tone={RESOLVED.has(d.status) ? 'ok' : 'pending'}>
                    {RESOLVED.has(d.status) ? t('dispute.resolved') : t('ad.dp.status')}
                  </Badge>
                  {!RESOLVED.has(d.status) && (
                    <Button size="sm" variant="outline" onClick={() => openResolve(d)}>
                      {t('ad.dp.resolve')}
                    </Button>
                  )}
                </div>
              </li>
            ))}
          </ul>
          {disputes.length > 10 && (
            <Pagination page={page.page} hasMore={page.hasMore} onPage={page.setPage} />
          )}
        </>
      )}

      <Dialog open={!!resolveOpen} title={t('ad.dp.resolve')} onClose={() => setResolveOpen(null)}>
        {resolveOpen && (
          <>
            <p className="text-sm text-mute mb-3">{resolveOpen.reason}</p>
            <div className="space-y-2 mb-4">
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="radio"
                  name="decision"
                  value="buyer"
                  checked={decision === 'buyer'}
                  onChange={() => setDecision('buyer')}
                />
                <span>{t('ad.dp.buyerWins')}</span>
              </label>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="radio"
                  name="decision"
                  value="seller"
                  checked={decision === 'seller'}
                  onChange={() => setDecision('seller')}
                />
                <span>{t('ad.dp.sellerWins')}</span>
              </label>
            </div>
            <textarea
              value={note}
              onChange={(e) => setNote(e.target.value)}
              placeholder={t('ad.dp.resolutionPh')}
              disabled={busy === resolveOpen.id}
              rows={3}
              className="block w-full resize-none rounded-xl border border-border bg-surface-hi text-sm py-2 px-3 text-text outline-none placeholder:text-mute disabled:opacity-50 focus-within:border-cyan-400/50"
            />
            <div className="mt-4 flex gap-2 justify-end">
              <Button size="sm" variant="ghost" disabled={busy === resolveOpen?.id} onClick={() => setResolveOpen(null)}>
                {t('ad.common.cancel')}
              </Button>
              <Button size="sm" variant="destructive" disabled={busy === resolveOpen?.id} onClick={resolve}>
                {busy === resolveOpen?.id ? t('ad.dp.resolving') : t('ad.dp.resolve')}
              </Button>
              </div>
          </>
        )}
      </Dialog>
    </div>
  );
}
