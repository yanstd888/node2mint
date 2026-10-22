'use client';
// Pesanan pembeli: konfirmasi terima, chat seller, lihat barang instan.
// Daftar milik sendiri (difilter server). Cancel sudah dihapus — order yang
// tidak dikerjakan seller dibatalkan otomatis oleh sistem setelah 24h.
import { useState } from 'react';
import Link from 'next/link';
import { api } from '@/lib/api';
import { post, pesanError } from '@/lib/http';
import { useFetch } from '@/lib/useApi';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useToast } from '@/components/ui/toast';
import { Pagination } from '@/components/ui/Pagination';
import { rupiah, tanggal, asList } from '@/lib/utils';
import { Package, MessageCircle } from 'lucide-react';
import { useT } from '@/lib/i18n';

type Order = {
  id: string;
  status: string;
  subtotal?: number;
  quantity?: number;
  created_at?: string;
  created_at_iso?: string;
  kind?: 'PACKAGE' | 'MARKET' | 'SERVICE';
  is_instant?: boolean;
  product_title?: string;
  store_name?: string;
};

const statusLabel = (status: string, t: ReturnType<typeof useT>['t']) => ({
  PAID: t('ord.statusPaid'), PROCESSING: t('ord.statusProcessing'), COMPLETED: t('ord.statusCompleted'),
  DISPUTED: t('ord.statusDisputed'), REFUNDED: t('ord.statusRefunded'), CANCELLED: t('ord.statusCancelled'),
}[status] ?? t('ord.statusUnknown'));

export default function PesananPage() {
  const { t } = useT();
  const [page, setPage] = useState(1);
  const { data, loading, err, reload } = useFetch<{ has_more?: boolean }>(`/purchases?page=${page}`);
  const [busy, setBusy] = useState<string | null>(null);
  // Barang instan diambil per pesanan dan HANYA saat diminta: isi pembelian
  // tidak ikut terunduh bersama daftar pesanan.
  const [barang, setBarang] = useState<Record<string, string[] | undefined>>({});
  const [busyBarang, setBusyBarang] = useState<string | null>(null);

  const orders = asList<Order>(data, 'orders');
  const toast = useToast();

  const aksi = async (id: string, apa: 'confirm') => {
    setBusy(`${id}:${apa}`);
    try {
      await post(`/market/orders/${id}/${apa}`, {});
      toast.ok(t('ord.confirmOk'));
      await reload();
    } catch (e) {
      toast.err(pesanError(e, t('ord.confirmFail')));
    } finally {
      setBusy(null);
    }
  };

  const toggleBarang = async (id: string) => {
    if (barang[id]) {
      setBarang((prev) => {
        const next = { ...prev };
        delete next[id];
        return next;
      });
      return;
    }
    setBusyBarang(id);
    try {
      const res = await api.get<{ items?: string[] }>(`/market/orders/${id}/items`);
      setBarang((prev) => ({ ...prev, [id]: res.data?.items ?? [] }));
    } catch (e) {
      toast.err(pesanError(e, t('ord.itemsFail')));
    } finally {
      setBusyBarang(null);
    }
  };

  return (
    <div className="p-6 md:p-10 max-w-4xl mx-auto">
      <header className="mb-8">
        <p className="label mb-1">{t('mkt.orders')}</p>
        <h1 className="text-2xl font-semibold tracking-tight">{t('ord.h')}</h1>
      </header>

      {err ? (
        <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{err}</p>
      ) : loading ? (
        <div className="space-y-3">
          {[0, 1, 2].map((i) => <div key={i} className="card p-4 h-24"><div className="skel h-4 w-40 mb-3" /><div className="skel h-3 w-64" /></div>)}
        </div>
      ) : orders.length === 0 ? (
        <div className="card p-12 text-center">
          <Package className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />
          <p className="font-medium">{t('ord.empty')}</p>
          <p className="text-sm text-mute mt-1 mb-6">{t('ord.emptyP')}</p>
          <Link href="/market"><Button>{t('cart.browse')}</Button></Link>
        </div>
      ) : (
        <ul className="space-y-3">
          {orders.map((o) => {
            const market = o.kind === 'MARKET';
            const service = o.kind === 'SERVICE';
            const terbuka = market && (o.status === 'PENDING' || o.status === 'PAID' || o.status === 'PROCESSING');
            const isi = barang[o.id];
            const title = o.product_title || t('ord.product');
            return (
              <li key={o.id} className="card row-hover p-4">
                <div className="flex flex-wrap items-center gap-3 mb-3">
                  <Badge>{statusLabel(o.status, t)}</Badge>
                  <span className="text-xs text-mute font-mono">#{o.id.slice(-6)}</span>
                  <span className="text-xs text-mute">{tanggal(o.created_at_iso ?? o.created_at, true)}</span>
                </div>
                <div className="flex flex-wrap items-center justify-between gap-4">
                  <div className="min-w-0">
                    <p className="font-medium truncate">{title}</p>
                    <p className="text-xs text-mute mt-1">
                      {o.store_name || (!market ? t('pay.mWallet') : '-')} &middot; {t('ord.unit', { n: o.quantity ?? 1 })}
                    </p>
                  </div>
                  <div className="act-group">
                    <span className="text-lg font-semibold tabular-nums text-cyan-300 mr-1">{rupiah(o.subtotal)}</span>
                    {market && !o.is_instant && o.status !== 'CANCELLED' && o.status !== 'REFUNDED' && (
                      <Link href={`/pesanan/${o.id}/chat`} passHref>
                        <Button size="sm" variant="outline" aria-label={t('ord.chatAria', { title })}>
                          <MessageCircle className="w-4 h-4" aria-hidden="true" /> {t('ord.chat')}
                        </Button>
                      </Link>
                    )}
                    {market && o.is_instant && o.status !== 'CANCELLED' && o.status !== 'REFUNDED' && (
                      <Button size="sm" variant="outline" disabled={busyBarang === o.id} onClick={() => toggleBarang(o.id)}>
                        {busyBarang === o.id ? t('ord.loading') : isi ? t('ord.hideItems') : t('ord.showItems')}
                      </Button>
                    )}
                    {service && <Link href="/toko?tab=produk"><Button size="sm" variant="outline">{t('ord.showItems')}</Button></Link>}
                    {terbuka && (
                      <Button size="sm" disabled={busy === `${o.id}:confirm`} onClick={() => aksi(o.id, 'confirm')}>
                        {busy === `${o.id}:confirm` ? t('pay.processing') : t('ord.confirm')}
                      </Button>
                    )}
                  </div>
                </div>
                {isi && (
                  <div className="mt-3 border-t border-border pt-3">
                    {isi.length === 0 ? (
                      <p className="text-xs text-mute">{t('ord.noInstant')}</p>
                    ) : (
                      <>
                        <p className="label mb-2">{t('ord.yourItems')}</p>
                        <ul className="space-y-1">
                          {isi.map((tt, i) => (
                            <li key={i} className="font-mono text-xs break-all bg-surface-hi border border-border rounded px-3 py-2">{tt}</li>
                          ))}
                        </ul>
                      </>
                    )}
                  </div>
                )}
              </li>
            );
          })}
        </ul>
      )}{/* end list */}

      {!err && !loading && orders.length > 0 && (
        <Pagination page={page} hasMore={data?.has_more} onPage={setPage} />
      )}
    </div>
  );
}
