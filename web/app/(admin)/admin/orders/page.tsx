'use client';
// Admin · Pesanan: daftar semua pesanan + refund (konfirmasi bersama useConfirm).
import { useState } from 'react';
import { post, pesanError } from '@/lib/http';
import { useFetch } from '@/lib/useApi';
import { useToast } from '@/components/ui/toast';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { rupiah, tanggal, asList } from '@/lib/utils';
import { Pagination, useClientPagination } from '@/components/ui/Pagination';
import { useT } from '@/lib/i18n';
import { useConfirm } from '@/components/ui/confirm';

type Order = {
  id: string; status: string; total?: number; subtotal?: number; quantity?: number;
  created_at?: string; buyer_email?: string; user_email?: string; product?: { title?: string };
};

const statusLabel = (status: string, t: ReturnType<typeof useT>['t']) => ({
  PAID: t('ad.status.paid'), PROCESSING: t('ad.status.processing'), COMPLETED: t('ad.status.completed'),
  DISPUTED: t('ad.status.disputed'), REFUNDED: t('ad.status.refunded'), CANCELLED: t('ad.status.cancelled'),
}[status] ?? status);

export default function AdminOrdersPage() {
  const { data, loading, err, reload } = useFetch<unknown>('/admin/orders');
  const { t } = useT();
  const toast = useToast();
  const [busy, setBusy] = useState<string | null>(null);
  const confirm = useConfirm();

  const orders = asList<Order>(data, 'orders');
  const ordersPage = useClientPagination(orders, 10);

  const refund = async (o: Order) => {
    if (!(await confirm({
      title: t('ad.or.confirm'),
      body: t('ad.or.refundBody', { email: o.buyer_email || o.user_email || '-', amount: rupiah(o.total ?? o.subtotal) }),
      confirmLabel: t('ad.or.confirmYes'),
      variant: 'destructive',
    }))) return;
    setBusy(o.id);
    try {
      await post(`/admin/orders/${o.id}/refund`, {});
      toast.ok(t('ad.or.refunded'));
      await reload();
    } catch (e) {
      toast.err(pesanError(e, t('ad.or.refundFail')));
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="p-6 md:p-10 max-w-5xl mx-auto">
      <header className="mb-8">
        <p className="label mb-1">{t('ad.common.admin')}</p>
        <h1 className="text-2xl font-semibold tracking-tight">{t('adm.orders')}</h1>
      </header>

      {err && <p role="alert" className="mb-6 text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{err}</p>}

      {err ? null : loading ? (
        <div className="space-y-3">{[0, 1, 2].map((i) => <div key={i} className="card p-4 h-20"><div className="skel h-4 w-56 mb-2" /><div className="skel h-3 w-40" /></div>)}</div>
      ) : orders.length === 0 ? (
        <p className="card p-8 text-center text-sm text-mute">{t('ad.or.empty')}</p>
      ) : (
        <>
          <ul className="space-y-3">
            {ordersPage.items.map((o) => (
              <li key={o.id} className="card p-4 flex flex-wrap items-center justify-between gap-4">
                <div className="min-w-0">
                  <p className="text-sm font-medium truncate">{o.product?.title || t('ad.or.productFallback')}</p>
                  <p className="text-xs text-mute mt-1 flex items-center gap-2 flex-wrap">
                    {o.buyer_email || o.user_email || '-'} &middot; <Badge>{statusLabel(o.status, t)}</Badge> &middot; {tanggal(o.created_at, true)}
                  </p>
                  <p className="text-xs text-mute font-mono mt-1">#{o.id.slice(-8)}</p>
                </div>
                <div className="flex items-center gap-2 flex-wrap shrink-0">
                  <span className="text-sm tabular-nums text-cyan-300 mr-1">{rupiah(o.total ?? o.subtotal)}</span>
                  <Button size="sm" variant="outline" disabled={busy === o.id} onClick={() => refund(o)}>{t('ad.or.refund')}</Button>
                </div>
              </li>
            ))}
          </ul>
          {orders.length > 10 && (
            <Pagination page={ordersPage.page} hasMore={ordersPage.hasMore} onPage={ordersPage.setPage} />
          )}
        </>
      )}
    </div>
  );
}
