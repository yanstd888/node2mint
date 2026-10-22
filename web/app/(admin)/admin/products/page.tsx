'use client';

// Admin · Produk: moderasi listing (tayangkan / sembunyikan).

import { useState } from 'react';

import { patch, pesanError } from '@/lib/http';

import { useFetch } from '@/lib/useApi';

import { useToast } from '@/components/ui/toast';

import { Button } from '@/components/ui/button';

import { rupiah, asList } from '@/lib/utils';

import { Store } from 'lucide-react';

import { Pagination, useClientPagination } from '@/components/ui/Pagination';

import { useT } from '@/lib/i18n';
import { useConfirm } from '@/components/ui/confirm';



type Produk = { id: string; title: string; price: number; stock?: number; is_active?: boolean; pin_order?: number | null; store?: { name?: string } | string };



export default function AdminProdukPage() {

  const { data, loading, err, reload } = useFetch<unknown>('/admin/products');

  const { t } = useT();

  const toast = useToast();
  const [busy, setBusy] = useState<string | null>(null);
  const [pinDraft, setPinDraft] = useState<Record<string, string>>({});
  const confirm = useConfirm();


  const daftar = asList<Produk>(data, 'products');

  const daftarPage = useClientPagination(daftar, 10);



  const setTayang = async (id: string, nilai: boolean, title?: string) => {
    if (!nilai && !(await confirm({ title: t('ad.pr.hide'), body: `${title || 'Product'} — it will be hidden from the marketplace.`, confirmLabel: t('ad.pr.hide') }))) return;
    setBusy(id);

    try {

      await patch(`/admin/products/${id}`, { is_active: nilai });

      toast.ok(nilai ? t('ad.pr.shown') : t('ad.pr.hidden'));

      await reload();

    } catch (e) {

      toast.err(pesanError(e, t('ad.pr.fail')));

    } finally {

      setBusy(null);

    }

  };

  const setPinOrder = async (id: string, raw: string) => {
    const value = raw.trim();
    const n = value ? Number(value) : 0;
    if (value && (!Number.isInteger(n) || n < 1)) {
      toast.err(t('ad.pr.pinInvalid'));
      return;
    }
    setBusy(`${id}:pin`);
    try {
      await patch(`/admin/products/${id}`, value ? { pin_order: n } : { clear_pin_order: true });
      toast.ok(t('ad.pr.pinSaved'));
      await reload();
    } catch (e) {
      toast.err(pesanError(e, t('ad.pr.pinFail')));
    } finally {
      setBusy(null);
    }
  };



  return (

    <div className="p-6 md:p-10 max-w-4xl mx-auto">

      <header className="mb-8">

        <p className="label mb-1">{t('ad.common.admin')}</p>

        <h1 className="text-2xl font-semibold tracking-tight">{t('adm.products')}</h1>

      </header>



      {err && <p role="alert" className="mb-6 text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{err}</p>}



      {err ? null : loading ? (

        <div className="space-y-3">{[0, 1, 2].map((i) => <div key={i} className="card p-4 h-16"><div className="skel h-4 w-56" /></div>)}</div>

      ) : daftar.length === 0 ? (

        <div className="card p-10 text-center">

          <Store className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />

          <p className="font-medium">{t('ad.pr.empty')}</p>

          <p className="text-sm text-mute mt-1">{t('ad.pr.emptyP')}</p>

        </div>

      ) : (

        <>

        <ul className="tbl-wrap zebra">

          {daftarPage.items.map((p) => {

            const tayang = p.is_active !== false;

            const toko = typeof p.store === 'string' ? p.store : p.store?.name;

            return (

              <li key={p.id} className="p-4 flex flex-wrap items-center justify-between gap-4">

                <div className="min-w-0">

                  <div className="flex items-center gap-3 flex-wrap">

                    <p className="text-sm font-medium truncate">{p.title}</p>

                    <span className={`badge ${tayang ? 'badge-on' : 'badge-off'}`}>{tayang ? t('ad.common.active') : t('ad.common.inactive')}</span>

                  </div>

                  <p className="text-xs text-mute mt-1 tabular-nums">

                    {rupiah(p.price)} &middot; {t('ad.pr.stock', { n: p.stock ?? 0 })}{toko ? ` · ${toko}` : ''}

                  </p>

                </div>
                <div className="act-group">
                  <div className="flex items-center gap-2">
                    <label htmlFor={`pin-${p.id}`} className="text-xs text-mute">{t('ad.pr.featured')}</label>
                    <input id={`pin-${p.id}`} type="number" min="1" step="1" inputMode="numeric"
                      value={pinDraft[p.id] ?? (p.pin_order == null ? '' : String(p.pin_order))}
                      onChange={(e) => setPinDraft((prev) => ({ ...prev, [p.id]: e.target.value }))}
                      placeholder="—" className="input h-9 w-16 px-2 text-center" />
                    <Button size="sm" variant="outline" disabled={busy === `${p.id}:pin`} onClick={() => setPinOrder(p.id, pinDraft[p.id] ?? (p.pin_order == null ? '' : String(p.pin_order)))}>
                      {busy === `${p.id}:pin` ? t('ad.common.processing') : t('ad.pr.pinSave')}
                    </Button>
                  </div>
                  <Button size="sm" variant={tayang ? 'ghost' : 'default'} disabled={busy === p.id}
                    onClick={() => setTayang(p.id, !tayang, p.title)}>
                    {busy === p.id ? t('ad.common.processing') : tayang ? t('ad.pr.hide') : t('ad.common.publish')}

                  </Button>

                </div>

              </li>

            );

          })}

        </ul>

        {daftar.length > 10 && (

          <Pagination page={daftarPage.page} hasMore={daftarPage.hasMore} onPage={daftarPage.setPage} />

        )}

        </>

      )}

    </div>

  );

}

