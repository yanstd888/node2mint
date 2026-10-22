'use client';
// Toko seller: daftar sengketa pada pesanan di toko sendiri + formulir respon.
import { useState } from 'react';
import Link from 'next/link';
import { post, pesanError } from '@/lib/http';
import { useFetch } from '@/lib/useApi';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Dialog } from '@/components/ui/dialog';
import { useToast } from '@/components/ui/toast';
import { Pagination, useClientPagination } from '@/components/ui/Pagination';
import { tanggal, asList } from '@/lib/utils';
import { useT } from '@/lib/i18n';
import { ArrowLeft, AlertCircle, Check } from 'lucide-react';

type Dispute = {
  id: string;
  order_id: string;
  buyer_email: string;
  product_title: string;
  reason: string;
  evidence_url?: string;
  status: string;
  resolution?: string;
  seller_response?: string;
  seller_response_at?: string;
  created_at?: string;
  subtotal?: number;
};

const statusLabel = (s: string, t: ReturnType<typeof useT>['t']) => {
  switch (s) {
    case 'OPEN': return t('dispute.statusOpen');
    case 'RESOLVED': return t('dispute.statusResolved');
    case 'BUYER_WINS': return t('dispute.statusBuyerWins');
    case 'SELLER_WINS': return t('dispute.statusSellerWins');
    case 'MANUAL_REVIEW': return t('dispute.statusManual');
    default: return s;
  }
};

const statusTone = (s: string): 'warn' | 'ok' | 'danger' | 'pending' => {
  switch (s) {
    case 'RESOLVED':
    case 'SELLER_WINS': return 'ok';
    case 'BUYER_WINS': return 'danger';
    case 'MANUAL_REVIEW': return 'pending';
    default: return 'warn';
  }
};

export default function TokoDisputesPage() {
  const { t } = useT();
  const toast = useToast();
  const [page, setPage] = useState(1);
  const fetch = useFetch<{ has_more?: boolean }>('/store/disputes?page=' + page);
  const disputes = asList<Dispute>(fetch.data, 'disputes');
  const halaman = useClientPagination(disputes);

  const [resDialog, setResDialog] = useState<{ open: boolean; id: string; orderId: string } | null>(null);
  const [respon, setRespon] = useState('');
  const [responBusy, setResponBusy] = useState(false);

  const kirimResponder = async () => {
    const txt = respon.trim();
    if (txt.length < 10) { toast.err(t('dispute.min')); return; }
    setResponBusy(true);
    try {
      await post(`/market/orders/${resDialog?.orderId}/dispute/respond`, { response: txt });
      toast.ok(t('dispute.responseOk'));
      setResDialog(null);
      setRespon('');
      fetch.reload();
    } catch (e) {
      toast.err(pesanError(e, t('dispute.responseFail')));
    } finally {
      setResponBusy(false);
    }
  };

  const badgeTone = (s: string) => statusTone(s) as 'warn' | 'ok' | 'danger' | 'pending';
  const resolved = (s: Dispute) => s.status !== 'OPEN';

  return (
    <div className="p-6 md:p-10 max-w-4xl mx-auto">
      <header className="mb-8">
        <Link href="/toko" className="inline-flex items-center gap-2 text-sm text-mute hover:text-text mb-2">
          <ArrowLeft className="w-4 h-4" aria-hidden="true" /> {t('sel.title')}
        </Link>
        <p className="label mb-1">{t('dispute.kicker')}</p>
        <h1 className="text-2xl font-semibold tracking-tight">{t('dispute.listH')}</h1>
      </header>

      {fetch.err ? (
        <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{fetch.err}</p>
      ) : fetch.loading ? (
        <div className="space-y-4" aria-busy="true">
          {[0, 1, 2].map((i) => <div key={i} className="card p-4"><div className="skel h-4 w-48 mb-3" /><div className="skel h-3 w-full mb-2" /><div className="skel h-3 w-2/3" /></div>)}
        </div>
      ) : disputes.length === 0 ? (
        <div className="card p-10 text-center">
          <AlertCircle className="w-12 h-12 mx-auto mb-4 text-mute" aria-hidden="true" />
          <p className="font-medium">{t('dispute.noDisputes')}</p>
          <p className="text-sm text-mute mt-1">{t('dispute.noDisputesP')}</p>
        </div>
      ) : (
        <ul className="space-y-4">
          {halaman.items.map((d) => (
            <li key={d.id} className="card p-4">
              <div className="flex flex-wrap items-center justify-between gap-3 mb-2">
                <p className="font-medium truncate">{d.product_title || '-'}</p>
                <Badge tone={badgeTone(d.status)}>{statusLabel(d.status, t)}</Badge>
              </div>
              <p className="text-xs text-mute mb-2">{t('dispute.orderId', { id: d.order_id.slice(0, 8) })} · {tanggal(d.created_at)}</p>
              <p className="text-sm text-mute mb-2 break-words">{d.reason}</p>
              {d.evidence_url && (
                <p className="text-xs text-mute mb-2 break-all">{d.evidence_url}</p>
              )}
              {d.seller_response && (
                <div className="mt-2 rounded-lg border border-border bg-surface-hi/50 px-3 py-2 text-sm">
                  <span className="text-xs font-medium text-mute">{t('dispute.sellerResponse')}</span>
                  <p className="text-sm text-mute mt-1 break-words">{d.seller_response}</p>
                </div>
              )}
              {!resolved(d) && !d.seller_response && (
                <Button size="sm" variant="outline" className="mt-3" onClick={() => { setResDialog({ open: true, id: d.id, orderId: d.order_id }); setRespon(''); }}>
                  {t('dispute.responseSubmit')}
                </Button>
              )}
              {resolved(d) && (
                <p className="text-xs text-mute mt-2">
                  {d.seller_response ? t('dispute.responseSent') : t('dispute.noResponse')}
                  {d.resolution && <span> · {t('dispute.outcome')}: {d.resolution}</span>}
                </p>
              )}
            </li>
          ))}
          {disputes.length > 10 && <Pagination page={page} hasMore={fetch.data?.has_more} onPage={setPage} />}
        </ul>
      )}

      <Dialog open={!!resDialog} title={t('dispute.sellerResponse')} onClose={() => setResDialog(null)}>
        <p className="text-sm text-mute mb-3">{t('dispute.sellerResponse')}</p>
        <textarea
          value={respon}
          onChange={(e) => setRespon(e.target.value)}
          placeholder={t('dispute.responsePh')}
          disabled={responBusy}
          rows={4}
          className="block w-full resize-none rounded-xl border border-border bg-surface-hi text-sm py-3 px-4 text-text outline-none placeholder:text-mute disabled:opacity-50 focus-within:border-cyan-400/50"
        />
        <div className="mt-4 flex gap-2 justify-end">
          <Button size="sm" variant="ghost" disabled={responBusy} onClick={() => setResDialog(null)}>{t('sel.cancel')}</Button>
          <Button size="sm" disabled={responBusy || respon.trim().length < 10} onClick={kirimResponder}>
            {responBusy ? t('dispute.responding') : <><Check className="w-4 h-4 mr-1" />{t('dispute.responseSubmit')}</>}
          </Button>
        </div>
      </Dialog>
    </div>
  );
}
