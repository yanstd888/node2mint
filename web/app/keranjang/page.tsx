'use client';
// Keranjang: ubah jumlah / hapus per baris, lalu checkout. Isi selalu dari server,
// bukan state lokal — keranjang bisa dibuka dari perangkat lain.
import { useState } from 'react';
import Link from 'next/link';
import { api } from '@/lib/api';
import { patch, pesanError } from '@/lib/http';
import { useFetch } from '@/lib/useApi';
import { Button } from '@/components/ui/button';
import { useToast } from '@/components/ui/toast';
import { rupiah, asList, paymentUrl } from '@/lib/utils';
import { useT } from '@/lib/i18n';
import { ShoppingCart, Trash2, Minus, Plus } from 'lucide-react';

type Item = {
  id: string;
  title?: string;
  product_id?: string;
  price?: number;
  quantity: number;
  subtotal?: number;
  tersedia?: boolean;
  catatan?: string | null;
};

export default function KeranjangPage() {
  const { t } = useT();
  const { data, loading, err, reload } = useFetch<unknown>('/cart');
  const [busy, setBusy] = useState<string | null>(null);
  const toast = useToast();

  const items = asList<Item>(data, 'items');
  const subtotal = Number((data as { subtotal?: number } | null)?.subtotal) || items.reduce((s, i) => s + Number(i.subtotal ?? 0), 0);
  const bisaDibeli = items.filter((i) => i.tersedia !== false);

  const ubah = async (id: string, quantity: number) => {
    setBusy(id);
    try {
      if (quantity <= 0) await api.del(`/cart/${id}`);
      else await patch(`/cart/${id}`, { quantity });
      await reload();
    } catch (e) {
      toast.err(pesanError(e, t('cart.failChange')));
    } finally {
      setBusy(null);
    }
  };

  const checkout = () => location.assign(paymentUrl('cart', subtotal, t('cart.label')));

  return (
    <div className="p-6 md:p-10 max-w-4xl mx-auto">
      <header className="mb-8">
        <p className="label mb-1">{t('cart.kicker')}</p>
        <h1 className="text-2xl font-semibold tracking-tight">{t('cart.h')}</h1>
      </header>

      {err ? (
        <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{err}</p>
      ) : loading ? (
        <div className="space-y-3">
          {[0, 1, 2].map((i) => <div key={i} className="card p-4 h-20"><div className="skel h-4 w-48 mb-2" /><div className="skel h-3 w-32" /></div>)}
        </div>
      ) : items.length === 0 ? (
        <div className="card p-12 text-center">
          <ShoppingCart className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />
          <p className="font-medium">{t('cart.emptyH')}</p>
          <p className="text-sm text-mute mt-1 mb-6">{t('cart.emptyP')}</p>
          <Link href="/market"><Button>{t('cart.browse')}</Button></Link>
        </div>
      ) : (
        <>
          <ul className="space-y-3 mb-6">
            {items.map((it) => (
              <li key={it.id} className="card p-4 flex flex-wrap items-center gap-4">
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium truncate">{it.title || t('cart.product')}</p>
                  <p className="text-xs text-mute mt-1 tabular-nums">
                    {rupiah(it.price)} / unit
                    {it.tersedia === false && <span className="text-danger"> &middot; {it.catatan || t('cart.unavailable')}</span>}
                  </p>
                </div>
                {/* Grup stepper: tidak ada kontrol bersarang di dalam tombol. */}
                <div className="flex items-center gap-1 shrink-0" role="group" aria-label={t('cart.qty', { title: it.title || t('cart.product') })}>
                  <Button variant="outline" size="sm" aria-label={t('pd.dec')}
                    disabled={busy === it.id} onClick={() => ubah(it.id, it.quantity - 1)}>
                    <Minus className="w-4 h-4" aria-hidden="true" />
                  </Button>
                  <span className="w-10 text-center text-sm tabular-nums">{it.quantity}</span>
                  <Button variant="outline" size="sm" aria-label={t('pd.inc')}
                    disabled={busy === it.id || it.tersedia === false} onClick={() => ubah(it.id, it.quantity + 1)}>
                    <Plus className="w-4 h-4" aria-hidden="true" />
                  </Button>
                </div>
                <span className="text-sm font-medium tabular-nums text-cyan-300 w-28 text-right shrink-0">
                  {rupiah(it.subtotal ?? Number(it.price ?? 0) * it.quantity)}
                </span>
                <button
                  onClick={() => ubah(it.id, 0)}
                  disabled={busy === it.id}
                  aria-label={t('cart.remove', { title: it.title || t('cart.product') })}
                  className="p-2 text-mute hover:text-danger hover:bg-danger/10 rounded-lg transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-danger/60 disabled:opacity-50"
                >
                  <Trash2 className="w-4 h-4" aria-hidden="true" />
                </button>
              </li>
            ))}
          </ul>

          <div className="card p-5 flex flex-wrap items-center justify-between gap-4">
            <div>
              <span className="label">{t('cart.subtotal')}</span>
              <p className="text-xl font-semibold tabular-nums text-cyan-300 mt-1">{rupiah(subtotal)}</p>
              {items.length !== bisaDibeli.length && (
                <p className="text-xs text-gold mt-1">{t('cart.partial')}</p>
              )}
            </div>
            <Button size="lg" disabled={busy === 'checkout' || bisaDibeli.length === 0} onClick={checkout}>
              {busy === 'checkout' ? t('pay.processing') : t('cart.checkout')}
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
