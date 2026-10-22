'use client';

// Admin · Dompet: penarikan menunggu (setujui/tolak) + top-up manual. Satu halaman,

// dua tabel — keduanya datang dari satu panggilan GET.

import { useState } from 'react';

import { post, pesanError } from '@/lib/http';

import { useFetch } from '@/lib/useApi';

import { useToast } from '@/components/ui/toast';
import { useT } from '@/lib/i18n';
import { useConfirm } from '@/components/ui/confirm';

import { Button } from '@/components/ui/button';

import { Badge } from '@/components/ui/badge';

import { rupiah, tanggal, asList } from '@/lib/utils';

import { Pagination, useClientPagination } from '@/components/ui/Pagination';



type Tarik = { id: string; amount?: number; fee_amount?: number; net_amount?: number; status?: string; bank_account?: string; user_email?: string; created_at?: string };

type Topup = { id: string; amount?: number; status?: string; method?: string; payment_reference?: string; proof_url?: string; user_email?: string; created_at?: string };

const statusLabel = (status: string | undefined, t: ReturnType<typeof useT>['t']) => ({
  PENDING: t('ad.status.pending'), APPROVED: t('ad.status.approved'), REJECTED: t('ad.status.rejected'),
}[status ?? ''] ?? status ?? t('ad.status.pending'));



export default function AdminWalletPage() {

  const { data, loading, err, reload } = useFetch<unknown>('/admin/wallet');

  const toast = useToast();
  const { t } = useT();

  const [busy, setBusy] = useState<string | null>(null);
  const confirm = useConfirm();



  const tarik = asList<Tarik>(data, 'withdrawals');

  const topup = asList<Topup>(data, 'topups');

  const tarikPage = useClientPagination(tarik, 10);

  const topupPage = useClientPagination(topup, 10);



  const putus = async (id: string, approve: boolean) => {

    const item = tarik.find((x) => x.id === id);
    if (!(await confirm({ title: approve ? t('ad.common.approve') : t('ad.common.reject'), body: `${item?.user_email || '-'} · ${rupiah(item?.amount)} — ${approve ? 'approve withdrawal' : 'reject withdrawal'}.` , confirmLabel: approve ? t('ad.common.approve') : t('ad.common.reject'), variant: approve ? 'default' : 'destructive' }))) return;
    setBusy(`${id}:${approve}`);

    try {

      await post(`/admin/withdrawals/${id}/${approve ? 'approve' : 'reject'}`, {});

      toast.ok(approve ? t('ad.wl.approved') : t('ad.wl.rejected'));

      await reload();

    } catch (e) {

      toast.err(pesanError(e, t('ad.wl.fail')));

    } finally {

      setBusy(null);

    }

  };



  const putusTopup = async (id: string, approve: boolean) => {

    const item = topup.find((x) => x.id === id);
    if (!(await confirm({ title: approve ? t('ad.common.approve') : t('ad.common.reject'), body: `${item?.user_email || '-'} · ${rupiah(item?.amount)} — ${approve ? 'complete top-up' : 'reject top-up'}.`, confirmLabel: approve ? t('ad.common.approve') : t('ad.common.reject'), variant: approve ? 'default' : 'destructive' }))) return;
    setBusy(`${id}:topup:${approve}`);

    try {

      await post(`/admin/topup/${id}/${approve ? 'complete' : 'reject'}`, {});

      toast.ok(approve ? t('ad.wl.topupOk') : t('ad.wl.topupRejected'));

      await reload();

    } catch (e) {

      toast.err(pesanError(e, t('ad.wl.topupFail')));

    } finally {

      setBusy(null);

    }

  };



  return (

    <div className="p-6 md:p-10 max-w-5xl mx-auto">

      <header className="mb-8">

        <p className="label mb-1">{t('ad.common.admin')}</p>

        <h1 className="text-2xl font-semibold tracking-tight">{t('ad.wl.h')}</h1>

      </header>



      {err && <p role="alert" className="mb-6 text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{err}</p>}



      {err ? null : loading ? (

        <div className="space-y-3">{[0, 1].map((i) => <div key={i} className="card p-4 h-20"><div className="skel h-4 w-56 mb-2" /><div className="skel h-3 w-40" /></div>)}</div>

      ) : (

        <>

          <section aria-label={t('ad.wl.withdrawals')} className="mb-10">

            <h2 className="text-lg font-semibold tracking-tight mb-4">

                  {t('ad.wl.withdrawals')} <span className="text-mute text-sm font-normal">({tarik.length})</span>

            </h2>

            {tarik.length === 0 ? (

              <p className="card p-6 text-sm text-mute text-center">{t('ad.wl.noWithdrawals')}</p>

            ) : (

              <ul className="space-y-3">

                {tarikPage.items.map((w) => {

                  const menunggu = (w.status || 'PENDING') === 'PENDING';

                  return (

                    <li key={w.id} className="card p-4 flex flex-wrap items-center justify-between gap-4">

                      <div className="min-w-0">

                        <p className="text-sm font-medium tabular-nums">{t('ad.wl.gross', { gross: rupiah(w.amount), fee: rupiah(w.fee_amount), net: rupiah(w.net_amount) })}</p>

                        <p className="text-xs text-mute mt-1 truncate">

                          {w.user_email || '-'} &middot; {w.bank_account || t('ad.wl.noBank')} &middot; {tanggal(w.created_at, true)}

                        </p>

                      </div>

                      <div className="flex items-center gap-2 shrink-0">

                        <Badge>{statusLabel(w.status, t)}</Badge>

                        {menunggu && (

                          <>

                            <Button size="sm" disabled={busy === `${w.id}:true`} onClick={() => putus(w.id, true)}>

                              {busy === `${w.id}:true` ? '…' : t('ad.common.approve')}

                            </Button>

                            <Button size="sm" variant="outline" disabled={busy === `${w.id}:false`} onClick={() => putus(w.id, false)}>

                              {busy === `${w.id}:false` ? '…' : t('ad.common.reject')}

                            </Button>

                          </>

                        )}

                      </div>

                    </li>

                  );

                })}

              </ul>

            )}

          </section>

          {tarik.length > 10 && (

            <Pagination page={tarikPage.page} hasMore={tarikPage.hasMore} onPage={tarikPage.setPage} />

          )}



          <section aria-label={t('ad.wl.topups')}>

            <h2 className="text-lg font-semibold tracking-tight mb-4">

              {t('ad.wl.topups')} <span className="text-mute text-sm font-normal">({topup.length})</span>

            </h2>

            {topup.length === 0 ? (

              <p className="card p-6 text-sm text-mute text-center">{t('ad.wl.noTopups')}</p>

            ) : (

              <ul className="card divide-y divide-border">

                {topupPage.items.map((item) => {

                  const menunggu = (item.status || 'PENDING') === 'PENDING';

                  return (

                  <li key={item.id} className="p-4 flex flex-wrap items-center justify-between gap-4">

                    <div className="min-w-0">

                      <p className="text-sm font-medium tabular-nums">{rupiah(item.amount)}</p>

                      <p className="text-xs text-mute mt-1 truncate">

                        {item.user_email || '-'} &middot; {item.method || '-'} &middot; {tanggal(item.created_at, true)}

                      </p>

                      <p className="mt-1 text-xs">{t('ad.wl.ref')} <span className="font-mono">{item.payment_reference || '-'}</span></p>
                      {item.proof_url && <a href={item.proof_url} target="_blank" rel="noreferrer" className="mt-2 inline-block text-xs font-medium text-cyan-300 hover:underline">{t('ad.wl.proof')}</a>}

                    </div>

                    <div className="flex items-center gap-2 shrink-0">

                      <Badge>{statusLabel(item.status, t)}</Badge>

                      {menunggu && <>

                        <Button size="sm" disabled={busy === `${item.id}:topup:true`} onClick={() => putusTopup(item.id, true)}>

                          {busy === `${item.id}:topup:true` ? '…' : t('ad.common.approve')}

                        </Button>

                        <Button size="sm" variant="outline" disabled={busy === `${item.id}:topup:false`} onClick={() => putusTopup(item.id, false)}>

                          {busy === `${item.id}:topup:false` ? '…' : t('ad.common.reject')}

                        </Button>

                      </>}

                    </div>

                  </li>

                  );

                })}

              </ul>

            )}

          </section>

          {topup.length > 10 && (

            <Pagination page={topupPage.page} hasMore={topupPage.hasMore} onPage={topupPage.setPage} />

          )}

        </>

      )}

    </div>

  );

}

