'use client';

import { useState } from 'react';
import { Gift, TicketPlus, Trash2 } from 'lucide-react';
import { post, pesanError } from '@/lib/http';
import { api } from '@/lib/api';
import { useFetch } from '@/lib/useApi';
import { useToast } from '@/components/ui/toast';
import { useConfirm } from '@/components/ui/confirm';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Pagination, useClientPagination } from '@/components/ui/Pagination';
import { useT } from '@/lib/i18n';

type Redeem = { id: string; code: string; amount: number; reward_type: string; used_count: number; max_uses?: number };
const kinds = ['WALLET', 'API_TOKEN', 'ACCESS_DAYS', 'BOOST', 'VERIFIED_30D'] as const;

export default function AdminRedeemPage() {
  const { t } = useT();
  const toast = useToast();
  const { data, loading, err, reload } = useFetch<{ redeems?: Redeem[] }>('/admin/redeem');
  const [amount, setAmount] = useState('');
  const [count, setCount] = useState('1');
  const [maxUses, setMaxUses] = useState('');
  const [kind, setKind] = useState<(typeof kinds)[number]>('WALLET');
  const [busy, setBusy] = useState(false);
  const confirm = useConfirm();
  const [created, setCreated] = useState<string[]>([]);
  const items = data?.redeems || [];
  const listPage = useClientPagination(items, 10);
  const label = (v: string) => v === 'WALLET' ? t('ad.rd.wallet') : v === 'API_TOKEN' ? t('ad.rd.token') : v === 'ACCESS_DAYS' ? t('ad.rd.days') : v === 'BOOST' ? t('ad.rd.boost') : t('ad.rd.vip');
  const fixedVIP = kind === 'VERIFIED_30D';

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      const res = await post<{ codes: string[] }>('/admin/redeem', {
        amount: fixedVIP ? 30 : Number(amount), count: Number(count), reward_type: kind,
        ...(maxUses ? { max_uses: Number(maxUses) } : {}),
      });
      setCreated(res.data.codes || []);
      toast.ok(t('ad.rd.created'));
      await reload();
    } catch (error) {
      toast.err(pesanError(error, t('ad.rd.createFail')));
    } finally { setBusy(false); }
  };

  const remove = async (item: Redeem) => {
    if (!(await confirm({ title: t('ad.common.deleteOf', { name: item.code }), confirmLabel: t('ad.common.delete') }))) return;
    try {
      await api.del(`/admin/redeem/${item.id}`);
      toast.ok(t('ad.rd.deleted'));
      await reload();
    } catch (error) {
      toast.err(pesanError(error, t('ad.rd.delFail')));
    }
  };

  return <main className="mx-auto max-w-5xl p-6 md:p-10">
    <header className="mb-8 flex flex-wrap items-end justify-between gap-4">
      <div>
        <p className="label mb-1">{t('ad.common.admin')}</p>
        <h1 className="text-2xl font-semibold tracking-tight">{t('ad.rd.h')}</h1>
        <p className="mt-1 text-sm text-mute">{t('ad.rd.sub')}</p>
      </div>
      <Badge tone="netral">{t('ad.rd.total', { n: items.length })}</Badge>
    </header>

    <section className="card mb-8 p-5 md:p-6" aria-labelledby="redeem-create-title">
      <div className="mb-5 flex items-start gap-3">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-cyan-500/15 text-cyan-300"><TicketPlus className="h-4 w-4" aria-hidden="true" /></span>
        <div>
          <h2 id="redeem-create-title" className="font-semibold">{t('ad.rd.new')}</h2>
          <p className="mt-1 text-sm text-mute">{t('ad.rd.createHint')}</p>
        </div>
      </div>
      <form onSubmit={create} className="space-y-5">
        <div className="grid gap-4 md:grid-cols-2">
          <label className="text-sm font-medium">{t('ad.rd.type')}<select value={kind} onChange={(e) => setKind(e.target.value as typeof kind)} className="input mt-2 w-full">{kinds.map((value) => <option key={value} value={value}>{label(value)}</option>)}</select></label>
          <label className="text-sm font-medium">{fixedVIP ? t('ad.rd.vipDays') : kind === 'BOOST' ? t('ad.rd.boostAmount') : t('ad.rd.amount')}<input required min="1" step={kind === 'WALLET' ? '0.01' : '1'} type="number" value={fixedVIP ? '30' : amount} disabled={fixedVIP} onChange={(e) => setAmount(e.target.value)} className="input mt-2 w-full disabled:opacity-60" /></label>
          <label className="text-sm font-medium">{t('ad.rd.count')}<span className="mt-1 block text-xs font-normal text-mute">{t('ad.rd.countHint')}</span><input required min="1" max="100" type="number" value={count} onChange={(e) => setCount(e.target.value)} className="input mt-2 w-full" /></label>
          <label className="text-sm font-medium">{t('ad.rd.uses')}<span className="mt-1 block text-xs font-normal text-mute">{t('ad.rd.usesHint')}</span><input min="1" type="number" value={maxUses} onChange={(e) => setMaxUses(e.target.value)} className="input mt-2 w-full" /></label>
        </div>
        <div className="flex justify-end border-t border-border pt-4"><Button disabled={busy}>{busy ? t('ad.common.processing') : t('ad.rd.new')}</Button></div>
      </form>
    </section>

    {created.length > 0 && <section className="mb-8 rounded-xl border border-cyan-500/25 bg-cyan-500/10 p-5" aria-live="polite">
      <div className="mb-3 flex items-center gap-2 text-sm font-medium text-cyan-200"><Gift className="h-4 w-4" aria-hidden="true" />{t('ad.rd.createdCodes')}</div>
      <pre className="overflow-x-auto whitespace-pre-wrap font-mono text-sm text-text">{created.join('\n')}</pre>
    </section>}

    {err && <p role="alert" className="mb-6 rounded-lg border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger">{err}</p>}
    {err ? null : loading ? <div className="space-y-3">{[0, 1, 2].map((value) => <div key={value} className="card h-20 p-4"><div className="skel mb-3 h-4 w-48" /><div className="skel h-3 w-32" /></div>)}</div> : items.length === 0 ? <div className="card p-10 text-center"><Gift className="mx-auto mb-4 h-10 w-10 text-mute" aria-hidden="true" /><p className="font-medium">{t('ad.rd.empty')}</p><p className="mt-1 text-sm text-mute">{t('ad.rd.emptyP')}</p></div> : <>
      <section aria-labelledby="redeem-list-title">
        <div className="mb-3 flex items-center justify-between gap-3"><h2 id="redeem-list-title" className="text-lg font-semibold">{t('ad.rd.list')}</h2><span className="text-sm text-mute">{t('ad.rd.showing', { n: listPage.items.length })}</span></div>
        <ul className="tbl-wrap zebra">
          {listPage.items.map((item) => {
            const limit = item.max_uses ?? 1;
            const exhausted = item.used_count >= limit;
            return <li key={item.id} className="flex flex-wrap items-center justify-between gap-4 p-4">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-3"><code className="rounded bg-surface-hi px-2 py-1 text-sm font-medium">{item.code}</code><Badge tone={exhausted ? 'danger' : 'ok'}>{exhausted ? t('ad.rd.exhausted') : t('ad.rd.active')}</Badge></div>
                <p className="mt-2 text-sm text-mute"><span className="text-text">{label(item.reward_type)}</span><span className="px-2">·</span><span className="tabular-nums">{item.amount.toLocaleString('id-ID')}</span><span className="px-2">·</span><span className="tabular-nums">{item.used_count}/{limit} {t('ad.rd.used')}</span></p>
              </div>
              <Button type="button" variant="ghost" size="sm" onClick={() => remove(item)} aria-label={t('ad.common.deleteOf', { name: item.code })}><Trash2 className="h-4 w-4" aria-hidden="true" />{t('ad.common.delete')}</Button>
            </li>;
          })}
        </ul>
      </section>
      {items.length > 10 && <Pagination page={listPage.page} hasMore={listPage.hasMore} onPage={listPage.setPage} />}
    </>}
  </main>;
}
