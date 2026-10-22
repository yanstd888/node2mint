'use client';
// Dompet: tab Isi Saldo & Mutasi / Tukar Kode / Hadiah Harian / Referral.
// Tab dibaca dari ?tab= saat pertama render (link lama /redeem dll diarahkan ke sini).
import { Suspense, useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { post, pesanError } from '@/lib/http';
import { useFetch } from '@/lib/useApi';
import ReferralSection from '@/components/sections/ReferralSection';
import { useAuth } from '@/lib/auth';
import { useToast } from '@/components/ui/toast';
import { useConfirm } from '@/components/ui/confirm';
import { Button, buttonVariants } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Pagination } from '@/components/ui/Pagination';
import { money, amountToIDR, rupiah, tanggal, angka, asList } from '@/lib/utils';
import { useT } from '@/lib/i18n';
import { LockedOverlay } from '@/components/LockedOverlay';
import { Wallet as WalletIcon, ArrowDownLeft, ArrowUpRight, Ticket, Gift, Check,
  Users, Copy } from 'lucide-react';

type Tab = 'utama' | 'redeem' | 'reward' | 'referral';
const TAB_KEY = { utama: 'wal.tabMain', redeem: 'wal.tabRedeem', reward: 'wal.tabReward', referral: 'wal.tabReferral' } as const;
type Trx = { id: string; amount: number; type: string; status: string; description?: string; environment?: 'sandbox' | 'production'; created_at?: string };
type PayoutAccount = { id: string; type: string; provider: string; account_identifier: string; account_name: string };
type ProfileData = { first_name?: string; last_name?: string; address?: string; payout_accounts?: PayoutAccount[] };
const TABS: Tab[] = ['utama', 'redeem', 'reward', 'referral'];

function WalletPageContent() {
  const { t } = useT();
  const params = useSearchParams();
  const [tab, setTab] = useState<Tab>('utama');
  useEffect(() => {
    const requested = params.get('tab');
    if (requested === 'redeem' || requested === 'reward' || requested === 'referral') setTab(requested);
    else if (!requested) setTab('utama');
  }, [params]);

  return (
    <div className="p-6 md:p-10 max-w-4xl mx-auto">
      <header className="mb-6">
        <p className="label mb-1">{t('wal.title')}</p>
        <h1 className="text-2xl font-semibold tracking-tight">{t('wal.h')}</h1>
      </header>

      <div className="tab-pills mb-8" role="tablist" aria-label={t('wal.tabsAria')}>
        {TABS.map((id) => (
          <button key={id} role="tab" aria-selected={tab === id} onClick={() => setTab(id)} className="tab-pill">
            {t(TAB_KEY[id])}
          </button>
        ))}
      </div>

      {tab === 'utama' && <TabUtama />}
      {tab === 'redeem' && <TabRedeem />}
      {tab === 'reward' && <TabReward />}
      {tab === 'referral' && <ReferralSection />}
    </div>
  );
}

// ---------------------------------------------------------------- Tab 1
function TabUtama() {
  const { t, lang } = useT();
  const [page, setPage] = useState(1);
  const saldo = useFetch<{ balance?: number }>('/wallet');
  const trx = useFetch<{ has_more?: boolean }>(`/wallet/transactions?page=${page}`);
  const profile = useFetch<ProfileData>('/account/profile');

  const [topup, setTopup] = useState('');
  const [tarik, setTarik] = useState('');
  const [rekening, setRekening] = useState('');
  const [busy, setBusy] = useState<'tarik' | null>(null);
  const toast = useToast();
  const confirm = useConfirm();
  const [result, setResult] = useState<{ ok: boolean; title: string; message: string } | null>(null);

  const topupIDR = amountToIDR(topup, lang);
  const tarikIDR = amountToIDR(tarik, lang);
  const formatInput = (value: string) => lang === 'id' ? angka(value) : value;
  const setAmount = (set: (value: string) => void) => (value: string) => set(lang === 'id' ? value.replace(/\D/g, '') : value.replace(/[^\d.]/g, '').replace(/(\..*)\./g, '$1'));

  const daftar = asList<Trx>(trx.data, 'transactions');

  const kirimTarik = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!(await confirm({ title: t('wal.withdrawConfirmH'), body: t('wal.withdrawConfirmP', { amount: money(tarikIDR, lang) }), confirmLabel: t('wal.submitWithdraw'), variant: 'default' }))) return;
    setBusy('tarik');
    try {
      const { data } = await post<{ fee?: number; net?: number }>('/wallet/withdrawals', { amount: tarikIDR, payout_account_id: rekening });
      toast.ok(t('wal.withdrawOk', { fee: money(data?.fee, lang), net: money(data?.net, lang) }));
      setResult({ ok: true, title: t('wal.withdrawSuccessH'), message: t('wal.withdrawSuccessP') });
      setTarik('');
      setRekening('');
      saldo.reload();
      trx.reload();
    } catch (e) {
      toast.err(pesanError(e, t('wal.withdrawFail')));
    } finally {
      setBusy(null);
    }
  };

  return (
    <>
      {result && <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" role="presentation" onClick={() => setResult(null)}>
        <section role="dialog" aria-modal="true" className="card w-full max-w-md p-6 text-center" onClick={(e) => e.stopPropagation()}>
          <Check className={`mx-auto mb-3 h-10 w-10 ${result.ok ? 'text-emerald-300' : 'text-danger'}`} aria-hidden="true" />
          <h2 className="text-lg font-semibold">{result.title}</h2><p className="mt-2 text-sm text-mute">{result.message}</p>
          <Button className="mt-5" onClick={() => setResult(null)}>{t('wal.close')}</Button>
        </section>
      </div>}
      <section aria-label={t('wal.balanceAria')} className="card saldo p-5 mb-8">
        <span className="label">{t('wal.current')}</span>
        {saldo.err ? (
          <p role="alert" className="text-sm text-danger mt-2">{saldo.err}</p>
        ) : (
          <span className="block mt-2 text-3xl font-semibold tabular-nums tracking-tight text-cyan-300">
            {saldo.loading ? <span className="skel inline-block h-8 w-40" /> : money(saldo.data?.balance, lang)}
          </span>
        )}
      </section>


      <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-10">
        <section className="card p-5 space-y-4">
          <h2 className="font-semibold">{t('wal.topupH')}</h2>
          <div className="space-y-2"><label htmlFor="nominal" className="label">{lang === 'id' ? t('wal.amount') : t('wal.amountUsd')}</label><Input id="nominal" type="text" inputMode={lang === 'id' ? 'numeric' : 'decimal'} value={topup ? formatInput(topup) : ''} onChange={(e) => setAmount(setTopup)(e.target.value)} placeholder={lang === 'id' ? '50.000' : '10.00'} /></div>
          <Link href={topupIDR ? `/payment?kind=topup&amount=${topupIDR}&label=Isi%20Saldo` : '#'} aria-disabled={!topupIDR} className={buttonVariants({ className: `w-full ${topupIDR ? '' : 'pointer-events-none opacity-50'}` })}>{t('pay.pay')}</Link>
        </section>

        <form onSubmit={kirimTarik} className="card p-5 space-y-4">
          <h2 className="font-semibold">{t('wal.withdrawH')}</h2>
          <div className="space-y-2">
            <label htmlFor="tarik" className="label">{lang === 'id' ? t('wal.amount') : t('wal.amountUsd')}</label>
            <Input id="tarik" type="text" inputMode={lang === 'id' ? 'numeric' : 'decimal'} value={tarik ? formatInput(tarik) : ''}
              onChange={(e) => setAmount(setTarik)(e.target.value)} placeholder={lang === 'id' ? '1.000.000' : '100.00'} required />
          </div>
          <div className="space-y-2">
            <label htmlFor="rekening" className="label">{t('wal.payout')}</label>
            <Select id="rekening" value={rekening} onChange={(e) => setRekening(e.target.value)} required>
              <option value="">{t('wal.pickAccount')}</option>
              {(profile.data?.payout_accounts ?? []).map((a) => <option key={a.id} value={a.id}>{a.provider} · {a.account_identifier} · {a.account_name}</option>)}
            </Select>
            {(profile.data?.payout_accounts ?? []).length === 0 && <p className="text-xs text-amber-300">{t('wal.noAccounts')}</p>}
          </div>
          {tarikIDR > 0 && <div className="rounded-lg border border-border bg-surface-hi p-3 text-sm">
            <div className="flex justify-between"><span className="text-mute">{t('wal.fee')}</span><span>{money(tarikIDR * 0.05, lang)}</span></div>
            <div className="mt-2 flex justify-between font-medium"><span>{t('wal.received')}</span><span className="text-cyan-300">{money(tarikIDR * 0.95, lang)}</span></div>
          </div>}
          <Button type="submit" variant="outline" disabled={busy === 'tarik' || !tarikIDR || !rekening} className="w-full">
            {busy === 'tarik' ? t('wal.sending') : t('wal.submitWithdraw')}
          </Button>
        </form>
      </div>

      <section aria-label={t('wal.historyAria')}>
        <h2 className="text-lg font-semibold tracking-tight mb-4">{t('wal.history')}</h2>
        {trx.err ? (
          <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{trx.err}</p>
        ) : trx.loading ? (
          <div className="space-y-3">
            {[0, 1, 2].map((i) => <div key={i} className="card p-4 h-16"><div className="skel h-4 w-56" /></div>)}
          </div>
        ) : daftar.length === 0 ? (
          <div className="card p-8 text-center">
            <WalletIcon className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />
            <p className="font-medium">{t('wal.noTrx')}</p>
            <p className="text-sm text-mute mt-1">{t('wal.noTrxP')}</p>
          </div>
        ) : (
          <>
            <ul className="card zebra">
              {daftar.map((tr) => {
                const masuk = Number(tr.amount) >= 0;
                return (
                  <li key={tr.id} className="p-4 flex items-center gap-3">
                    <span className={`w-8 h-8 shrink-0 rounded-full flex items-center justify-center ${masuk ? 'bg-cyan-500/15 text-cyan-300' : 'bg-danger/10 text-danger'}`}>
                      {masuk ? <ArrowDownLeft className="w-4 h-4" aria-hidden="true" /> : <ArrowUpRight className="w-4 h-4" aria-hidden="true" />}
                    </span>
                    <div className="min-w-0 flex-1">
                      <p className="text-sm truncate">{tr.description || tr.type}</p>
                      <p className="text-xs text-mute mt-1">{tr.status} &middot; {tanggal(tr.created_at, true)} {tr.environment === 'sandbox' && <span className="ml-1 rounded border border-amber-300/30 px-1.5 py-0.5 text-[10px] text-amber-300">SANDBOX</span>}</p>
                    </div>
                    <span className={`text-sm tabular-nums shrink-0 ${masuk ? 'text-cyan-300' : 'text-danger'}`}>
                      {masuk ? '+' : ''}{money(tr.amount, lang)}
                    </span>
                  </li>
                );
              })}
            </ul>
            <Pagination page={page} hasMore={trx.data?.has_more} onPage={setPage} />
          </>
        )}
      </section>
    </>
  );
}

// ---------------------------------------------------------------- Tab 2
function TabRedeem() {
  const { t, lang } = useT();
  const { user, refresh } = useAuth();
  const [code, setCode] = useState('');
  const redeemKey = useRef('');
  const [busy, setBusy] = useState(false);
  const toast = useToast();
  const [saldo, setSaldo] = useState<number | null>(null);
  const [message, setMessage] = useState<{ ok: boolean; text: string } | null>(null);

  const tukar = async (e: React.FormEvent) => {
    e.preventDefault();
    const kode = code.trim();
    if (!kode) return;
    setBusy(true);
    try {
      redeemKey.current ||= crypto.randomUUID();
      const { data } = await post<{ amount: number; balance: number }>('/redeem', { code: kode }, { 'Idempotency-Key': redeemKey.current });
      // ponytail: saldo lokal dari respons redeem saja; refresh() cukup untuk sidebar.
      setSaldo(Number(data?.balance));
      toast.ok(t('wal.redeemOk', { amount: money(data?.amount, lang) }));
      setMessage({ ok: true, text: t('wal.voucherValid') });
      setCode('');
      refresh(); // saldo di sidebar ikut diperbarui
    } catch (e2) {
      setMessage({ ok: false, text: t('wal.voucherInvalid') });
      toast.err(pesanError(e2, t('wal.redeemFail')));
    } finally {
      setBusy(false);
    }
  };

  const tampil = saldo ?? user?.balance;

  return (
    <LockedOverlay feature="redeem"><div className="max-w-xl">
      {tampil !== undefined && (
        <div className="card saldo p-5 mb-6 flex items-center justify-between">
          <p className="label">{t('wal.current')}</p>
          <p className="text-xl font-semibold tabular-nums text-cyan-300">{money(tampil, lang)}</p>
        </div>
      )}

      <form onSubmit={tukar} className="card p-6 space-y-4">
        <div className="space-y-2">
          {message && <p role={message.ok ? 'status' : 'alert'} className={`rounded-lg border px-3 py-2 text-sm ${message.ok ? 'border-emerald-400/30 bg-emerald-500/10 text-emerald-300' : 'border-danger/30 bg-danger/10 text-danger'}`}>{message.text}</p>}
          <label htmlFor="kode" className="label">{t('wal.redeemCode')}</label>
          <Input
            id="kode"
            value={code}
            disabled={busy}
            onChange={(e) => { setCode(e.target.value); redeemKey.current = ''; }}
            placeholder={t('wal.redeemPh')}
            autoComplete="off"
            className="font-mono uppercase"
          />
          <p className="text-xs text-mute flex items-center gap-2">
            <Ticket className="w-3.5 h-3.5" aria-hidden="true" /> {t('wal.redeemOnce')}
          </p>
        </div>
        <Button type="submit" disabled={!code.trim() || busy} className="w-full">
          {busy ? t('wal.redeemBusy') : t('wal.redeemBtn')}
        </Button>
      </form>
    </div></LockedOverlay>
  );
}

// ---------------------------------------------------------------- Tab 3
type StatusReward = { claimed?: boolean; claimedToday?: boolean; claimed_today?: boolean; amount?: number; reward?: number; streak?: number; history?: unknown };

function TabReward() {
  const { t, lang } = useT();
  const { data, loading, err, reload } = useFetch<StatusReward>('/reward/daily');
  const { refresh } = useAuth();
  const [busy, setBusy] = useState(false);
  const toast = useToast();

  const sudah = data?.claimed ?? data?.claimedToday ?? data?.claimed_today ?? false;
  const riwayat = asList<{ id?: string; amount?: number; reward_date?: string }>(data?.history, 'history', 'claims');

  const klaim = async () => {
    setBusy(true);
    try {
      const { data: res } = await post<{ amount?: number }>('/reward/daily', {});
      toast.ok(t('wal.claimOk', { amount: money(res?.amount, lang) }));
      reload();
      refresh(); // saldo sidebar
    } catch (e) {
      toast.err(pesanError(e, t('wal.claimFail')));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="max-w-2xl">
      {err ? (
        <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{err}</p>
      ) : loading ? (
        <div className="card p-6"><div className="skel h-6 w-40 mb-4" /><div className="skel h-10 w-full" /></div>
      ) : (
        <>
          <LockedOverlay feature="daily"><section aria-label={t('wal.rewardAria')} className="card saldo p-6 text-center">
            <Gift className="w-10 h-10 mx-auto mb-4 text-cyan-300" aria-hidden="true" />
            <p className="font-medium">{sudah ? t('wal.rewardTaken') : t('wal.rewardOpen')}</p>
            {data?.amount !== undefined && data.amount > 0 && (
              <p className="text-2xl font-semibold tabular-nums text-cyan-300 mt-2">{money(data.amount, lang)}</p>
            )}
            {data?.reward !== undefined && (
              <p className="text-sm text-mute mt-1 tabular-nums">{t('wal.rewardToday', { amount: money(data.reward, lang) })}</p>
            )}
            {data?.streak !== undefined && (
              <p className="text-sm text-mute mt-1 tabular-nums">{t('wal.streak', { n: data.streak })}</p>
            )}
            <Button className="mt-6 w-full sm:w-auto" disabled={busy || sudah} onClick={klaim}>
              {busy ? t('wal.claiming') : sudah ? t('wal.claimed') : t('wal.claim')}
            </Button>
          </section></LockedOverlay>

          {riwayat.length > 0 && (
            <section aria-label={t('wal.claimHistoryAria')} className="mt-8">
              <h2 className="text-lg font-semibold tracking-tight mb-4">{t('wal.claimHistory')}</h2>
              <ul className="card zebra">
                {riwayat.map((h, i) => (
                  <li key={h.id ?? i} className="p-4 flex items-center justify-between gap-4">
                    <span className="text-sm text-mute flex items-center gap-2">
                      <Check className="w-4 h-4 text-cyan-300" aria-hidden="true" />
                      {tanggal(h.reward_date)}
                    </span>
                    <span className="text-sm tabular-nums text-cyan-300">{money(h.amount, lang)}</span>
                  </li>
                ))}
              </ul>
            </section>
          )}
        </>
      )}
    </div>
  );
}

// Tab 4: Referral — isi komponen dipakai bersama halaman /referral
// (satu implementasi, dua pintu masuk; lihat components/sections/ReferralSection.tsx).

export default function WalletPage() {
  return (
    <Suspense fallback={<div className="p-6 md:p-10 max-w-4xl mx-auto"><div className="skel h-8 w-52" /></div>}>
      <WalletPageContent />
    </Suspense>
  );
}
