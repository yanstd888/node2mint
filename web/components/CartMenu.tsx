'use client';

import { useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { Minus, Plus, ShoppingCart, Trash2 } from 'lucide-react';
import { useAuth } from '@/lib/auth';
import { useT, useUrlPrefix } from '@/lib/i18n';
import { useFetch } from '@/lib/useApi';
import { api } from '@/lib/api';
import { patch, pesanError } from '@/lib/http';
import { asList, rupiah, paymentUrl } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { useToast } from '@/components/ui/toast';

type Item = {
  id: string;
  title?: string;
  price?: number;
  quantity: number;
  subtotal?: number;
  tersedia?: boolean;
  catatan?: string | null;
};

export default function CartMenu() {
  const { user } = useAuth();
  const { t } = useT();
  const prefix = useUrlPrefix();
  const toast = useToast();
  const ref = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);
  const cart = useFetch<unknown>('/cart', !user);
  const items = asList<Item>(cart.data, 'items');
  const count = items.reduce((total, item) => total + item.quantity, 0);
  const subtotal = Number((cart.data as { subtotal?: number } | null)?.subtotal)
    || items.reduce((total, item) => total + Number(item.subtotal ?? 0), 0);
  const bisaDibeli = items.filter((item) => item.tersedia !== false);

  useEffect(() => {
    if (!user) return;
    const reload = () => { void cart.reload(); };
    window.addEventListener('cart:changed', reload);
    return () => window.removeEventListener('cart:changed', reload);
  }, [user, cart.reload]);

  useEffect(() => {
    if (!open) return;
    const close = (event: PointerEvent) => {
      if (!ref.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener('pointerdown', close);
    return () => document.removeEventListener('pointerdown', close);
  }, [open]);

  const ubah = async (id: string, quantity: number) => {
    setBusy(id);
    try {
      if (quantity <= 0) await api.del(`/cart/${id}`);
      else await patch(`/cart/${id}`, { quantity });
      await cart.reload();
    } catch (error) {
      toast.err(pesanError(error, t('cart.failChange')));
    } finally {
      setBusy(null);
    }
  };

  const checkout = () => {
    setOpen(false);
    location.assign(paymentUrl('cart', subtotal, t('cart.label')));
  };

  if (!user) {
    return (
      <Link href="/masuk?next=%2Fmarket" aria-label={t('cart.aria')}
        className="grid h-11 w-11 place-items-center rounded-lg text-mute hover:bg-white/5 hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60">
        <ShoppingCart className="h-5 w-5" aria-hidden="true" />
      </Link>
    );
  }

  return (
    <div ref={ref} className="relative">
      <button type="button" onClick={() => setOpen((value) => !value)} aria-label={t('cart.label')}
        aria-expanded={open} aria-haspopup="dialog"
        className={`relative grid h-11 w-11 place-items-center rounded-lg transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60 ${open ? 'bg-white/10 text-text' : 'text-mute hover:bg-white/5 hover:text-text'}`}>
        <ShoppingCart className="h-5 w-5" aria-hidden="true" />
        {count > 0 && <span className="absolute right-0.5 top-0.5 min-w-4 rounded-full bg-cyan-300 px-1 text-center text-[10px] font-semibold leading-4 text-[#071015]">{count > 99 ? '99+' : count}</span>}
      </button>

      {open && (
        <section role="dialog" aria-label={t('cart.dialog')}
          className="fixed left-4 right-4 top-20 z-50 max-h-[calc(100dvh-6rem)] overflow-y-auto rounded-xl border border-border bg-bg p-4 shadow-2xl sm:absolute sm:left-auto sm:right-0 sm:top-12 sm:w-96">
          <div className="mb-4 flex items-center justify-between gap-3">
            <h2 className="font-semibold">{t('cart.label')}</h2>
            {items.length > 0 && <span className="text-xs text-mute tabular-nums">{t('cart.count', { n: count })}</span>}
          </div>

          {cart.err ? (
            <p role="alert" className="rounded-lg border border-danger/30 bg-danger/10 px-3 py-2 text-sm text-danger">{t('cart.fail')}</p>
          ) : cart.loading ? (
            <div className="space-y-3" aria-label={t('cart.loading')}>
              {[0, 1].map((i) => <div key={i} className="skel h-16" />)}
            </div>
          ) : items.length === 0 ? (
            <div className="py-8 text-center">
              <span className="mx-auto mb-4 grid h-16 w-16 place-items-center rounded-full bg-cyan-300/10 text-cyan-300">
                <ShoppingCart className="h-7 w-7" aria-hidden="true" />
              </span>
              <p className="font-semibold">{t('cart.emptyH')}</p>
              <p className="mt-1 text-sm text-mute">{t('cart.emptyP')}</p>
              <Link href={`${prefix}/market`} onClick={() => setOpen(false)} className="mt-5 inline-flex rounded-lg border border-cyan-300 px-4 py-2 text-sm font-semibold text-cyan-300 hover:bg-cyan-300/10">{t('cart.shop')}</Link>
            </div>
          ) : (
            <>
              <ul className="space-y-2">
                {items.map((item) => (
                  <li key={item.id} className="rounded-lg border border-border p-3">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className="truncate text-sm font-medium">{item.title || t('cart.product')}</p>
                        <p className="mt-1 text-xs tabular-nums text-mute">{t('cart.perUnit', { price: rupiah(item.price) })}</p>
                        {item.tersedia === false && <p className="mt-1 text-xs text-danger">{item.catatan || t('cart.unavailable')}</p>}
                      </div>
                      <button type="button" onClick={() => ubah(item.id, 0)} disabled={busy === item.id}
                        aria-label={t('cart.remove', { title: item.title || t('cart.product') })} className="p-1.5 text-mute hover:text-danger disabled:opacity-50">
                        <Trash2 className="h-4 w-4" aria-hidden="true" />
                      </button>
                    </div>
                    <div className="mt-3 flex items-center justify-between gap-3">
                      <div className="flex items-center rounded-lg border border-border" role="group" aria-label={t('cart.qty', { title: item.title || t('cart.product') })}>
                        <button type="button" onClick={() => ubah(item.id, item.quantity - 1)} disabled={busy === item.id}
                          aria-label={t('pd.dec')} className="p-1.5 text-mute hover:text-text disabled:opacity-50"><Minus className="h-3.5 w-3.5" /></button>
                        <span className="w-8 text-center text-xs tabular-nums">{item.quantity}</span>
                        <button type="button" onClick={() => ubah(item.id, item.quantity + 1)} disabled={busy === item.id || item.tersedia === false}
                          aria-label={t('pd.inc')} className="p-1.5 text-mute hover:text-text disabled:opacity-50"><Plus className="h-3.5 w-3.5" /></button>
                      </div>
                      <strong className="text-sm tabular-nums text-cyan-300">{rupiah(item.subtotal ?? Number(item.price ?? 0) * item.quantity)}</strong>
                    </div>
                  </li>
                ))}
              </ul>
              <div className="mt-4 border-t border-border pt-4">
                <div className="mb-4 flex items-center justify-between gap-3">
                  <span className="text-sm text-mute">{t('cart.subtotal')}</span>
                  <strong className="tabular-nums">{rupiah(subtotal)}</strong>
                </div>
                <Button className="w-full" disabled={busy === 'checkout' || bisaDibeli.length === 0} onClick={checkout}>
                  {busy === 'checkout' ? t('mkt.buying') : t('cart.checkout')}
                </Button>
              </div>
            </>
          )}
        </section>
      )}
    </div>
  );
}
