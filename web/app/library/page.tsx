'use client';

import { useState } from 'react';
import Link from 'next/link';
import { api } from '@/lib/api';
import { pesanError } from '@/lib/http';
import { useFetch } from '@/lib/useApi';
import { useT } from '@/lib/i18n';
import { useToast } from '@/components/ui/toast';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Pagination } from '@/components/ui/Pagination';
import { asList, rupiah, tanggal } from '@/lib/utils';
import { BookOpen, Download, PackageOpen } from 'lucide-react';

type Purchase = {
  id: string;
  kind?: 'PACKAGE' | 'MARKET' | 'SERVICE';
  product_title?: string;
  quantity?: number;
  subtotal?: number;
  status?: string;
  created_at?: string;
  created_at_iso?: string;
  is_instant?: boolean;
  store_name?: string;
};

export default function LibraryPage() {
  const { t } = useT();
  const toast = useToast();
  const [page, setPage] = useState(1);
  const [items, setItems] = useState<Record<string, string[] | undefined>>({});
  const [busy, setBusy] = useState<string | null>(null);
  const { data, loading, err } = useFetch<{ has_more?: boolean }>(`/purchases?page=${page}`);
  const purchases = asList<Purchase>(data, 'orders').filter((item) =>
    item.kind === 'MARKET' && item.is_instant && !['CANCELLED', 'REFUNDED'].includes(item.status ?? '')
  );

  const toggleItems = async (id: string) => {
    if (items[id]) {
      setItems((current) => {
        const next = { ...current };
        delete next[id];
        return next;
      });
      return;
    }
    setBusy(id);
    try {
      const response = await api.get<{ items?: string[] }>(`/market/orders/${id}/items`);
      setItems((current) => ({ ...current, [id]: response.data?.items ?? [] }));
    } catch (error) {
      toast.err(pesanError(error, t('lib.itemsFail')));
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="p-6 md:p-10 max-w-4xl mx-auto">
      <header className="mb-8">
        <p className="label mb-1">{t('sb.library')}</p>
        <h1 className="text-2xl font-semibold tracking-tight">{t('lib.h')}</h1>
        <p className="text-sm text-mute mt-2">{t('lib.sub')}</p>
      </header>

      {err ? (
        <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{err}</p>
      ) : loading ? (
        <div className="space-y-3" aria-busy="true">
          {[0, 1, 2].map((i) => <div key={i} className="card p-4 h-24"><div className="skel h-4 w-48 mb-3" /><div className="skel h-3 w-72" /></div>)}
        </div>
      ) : purchases.length === 0 ? (
        <div className="card p-12 text-center">
          <BookOpen className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />
          <p className="font-medium">{t('lib.empty')}</p>
          <p className="text-sm text-mute mt-1 mb-6">{t('lib.emptyP')}</p>
          <Link href="/market"><Button>{t('cart.browse')}</Button></Link>
        </div>
      ) : (
        <ul className="space-y-3">
          {purchases.map((purchase) => {
            const revealed = items[purchase.id];
            return (
              <li key={purchase.id} className="card row-hover p-4">
                <div className="flex flex-wrap items-center gap-3 mb-3">
                  <Badge tone="ok">{t('lib.digital')}</Badge>
                  <span className="text-xs text-mute">{tanggal(purchase.created_at_iso ?? purchase.created_at, true)}</span>
                </div>
                <div className="flex flex-wrap items-center justify-between gap-4">
                  <div className="min-w-0">
                    <p className="font-medium truncate">{purchase.product_title || t('ord.product')}</p>
                    <p className="text-xs text-mute mt-1">
                      {purchase.store_name || t('mkt.h')} · {t('ord.unit', { n: purchase.quantity ?? 1 })} · {rupiah(purchase.subtotal)}
                    </p>
                  </div>
                  <Button size="sm" variant="outline" disabled={busy === purchase.id} onClick={() => toggleItems(purchase.id)}>
                    {busy === purchase.id ? t('lib.loading') : revealed ? t('lib.hide') : <><Download className="w-4 h-4" aria-hidden="true" /> {t('lib.open')}</>}
                  </Button>
                </div>
                {revealed && (
                  <div className="mt-4 border-t border-border pt-4">
                    {revealed.length === 0 ? (
                      <p className="text-sm text-mute">{t('lib.noItems')}</p>
                    ) : (
                      <ul className="space-y-2" aria-label={t('lib.itemsAria')}>
                        {revealed.map((value, index) => (
                          <li key={index} className="flex items-start gap-2 rounded-lg border border-border bg-surface-hi p-3 text-sm">
                            <PackageOpen className="w-4 h-4 mt-0.5 shrink-0 text-cyan-300" aria-hidden="true" />
                            <span className="font-mono break-all">{value}</span>
                          </li>
                        ))}
                      </ul>
                    )}
                  </div>
                )}
              </li>
            );
          })}
        </ul>
      )}

      {!err && !loading && purchases.length > 0 && <Pagination page={page} hasMore={data?.has_more} onPage={setPage} />}
    </div>
  );
}
