'use client';

import { Suspense, useCallback, useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { CheckCircle2, ChevronDown, CreditCard, LoaderCircle, ShieldCheck, Wallet, XCircle } from 'lucide-react';
import { useConfirm } from '@/components/ui/confirm';
import { Button } from '@/components/ui/button';
import { useAuth } from '@/lib/auth';
import { post, pesanError } from '@/lib/http';
import { rupiah } from '@/lib/utils';
import { useT } from '@/lib/i18n';
import { UsdtInvoiceDialog } from '@/components/UsdtInvoiceDialog';
import { ImageField } from '@/components/ui/ImageField';
import { useFetch } from '@/lib/useApi';

type Kind = 'product' | 'package' | 'cart' | 'store_verify' | 'boost_pack' | 'topup';
type PurchaseResult = { id?: string; order_id?: string; orders?: { id?: string }[] };
type Receipt = { method: string; reference: string; paidAt: string; pending?: boolean };
type DokuCheckout = { invoice_number?: string; checkout_url?: string; pending?: boolean; status?: string; creating?: boolean };
type ManualMethods = { topup_bank_name?: string; topup_bank_account?: string; topup_bank_holder?: string; topup_qris_image_url?: string; topup_qris_merchant?: string };

function PaymentContent() {
  const { t, lang } = useT();
  const params = useSearchParams();
  const { user, refresh } = useAuth();
  const confirm = useConfirm();
  const refreshRef = useRef(refresh);
  refreshRef.current = refresh;
  const kind = params.get('kind') as Kind;
  const id = params.get('id') || '';
  const qty = Math.max(1, Number(params.get('qty')) || 1);
  const amount = Number(params.get('amount'));
  const qrisFee = Math.ceil(amount * 0.007);
  const qrisTotal = amount + qrisFee;
  const label = params.get('label') || t('pay.defaultLabel');
  const valid = ['product', 'package', 'cart', 'store_verify', 'boost_pack', 'topup'].includes(kind) && Number.isFinite(amount) && amount > 0 && (kind === 'cart' || kind === 'topup' || id);
  const [busy, setBusy] = useState<'wallet' | 'usdt' | 'doku' | 'settling' | null>(null);
  const [cryptoInvoice, setCryptoInvoice] = useState<{ id: string; address: string; amount: string } | null>(null);
  const [dokuInvoice, setDokuInvoice] = useState('');
  const [dokuCheckoutURL, setDokuCheckoutURL] = useState('');
  const [dokuChecked, setDokuChecked] = useState(false);
  const [error, setError] = useState('');
  const [receipt, setReceipt] = useState<Receipt | null>(null);
  const [dokuReturning, setDokuReturning] = useState(() => params.get('doku') === 'finish' && Boolean(params.get('doku_invoice')));
  const paymentMethods = useFetch<{ methods?: ManualMethods }>('/wallet/payment-methods', kind !== 'topup');
  const [manualMethod, setManualMethod] = useState<'qris' | 'transfer'>('qris');
  const [proofURL, setProofURL] = useState('');
  const finishing = useRef(false);
  const purchaseKey = useRef('');
  const dokuStorageKey = `doku:payment:${kind}:${id || 'cart'}:${qty}:${amount}`;
  const dokuRequestKey = `${dokuStorageKey}:request`;

  useEffect(() => {
    const saved = sessionStorage.getItem(dokuStorageKey);
    if (saved) setDokuInvoice(saved);
    void (async () => {
      try {
        const { data } = await (await import('axios')).default.get<DokuCheckout>('/api/wallet/doku/pending', { withCredentials: true });
        if (data.invoice_number) {
          sessionStorage.setItem(dokuStorageKey, data.invoice_number);
          setDokuInvoice(data.invoice_number);
          setDokuCheckoutURL(data.checkout_url || '');
        }
      } catch { /* authenticated status check may be unavailable during logout */ }
    })().finally(() => setDokuChecked(true));
  }, [dokuStorageKey]);

  const packageRequestKey = () => {
    const storageKey = `package-buy:${id}`;
    if (!purchaseKey.current) {
      purchaseKey.current = sessionStorage.getItem(storageKey) || crypto.randomUUID();
      sessionStorage.setItem(storageKey, purchaseKey.current);
    }
    return { storageKey, key: purchaseKey.current };
  };

  const marketRequestKey = () => {
    const storageKey = `market-order:${kind}:${id || 'cart'}:${qty}`;
    if (!purchaseKey.current) {
      purchaseKey.current = sessionStorage.getItem(storageKey) || crypto.randomUUID();
      sessionStorage.setItem(storageKey, purchaseKey.current);
    }
    return { storageKey, key: purchaseKey.current };
  };

  const finishPurchase = useCallback(async (method: Receipt['method'], providerReference = '-') => {
    if (!valid || finishing.current) return false;
    finishing.current = true;
    let request: { storageKey: string; key: string } | null = null;
    try {
      if (kind === 'topup') {
        setReceipt({ method, reference: providerReference, paidAt: new Date().toLocaleString(lang === 'id' ? 'id-ID' : 'en-US') });
        return true;
      }
      let result: PurchaseResult = {};
      if (kind === 'package') {
        request = packageRequestKey();
        result = (await post<PurchaseResult>(`/packages/${encodeURIComponent(id)}/buy`, {}, { 'Idempotency-Key': request.key })).data;
      }
      else if (kind === 'store_verify') result = (await post<PurchaseResult>('/store/verify', {})).data;
      else if (kind === 'boost_pack') result = (await post<PurchaseResult>(`/store/boost-packs/${encodeURIComponent(id)}/buy`, {})).data;
      else if (kind === 'product') {
        request = marketRequestKey();
        result = (await post<PurchaseResult>('/market/orders', { product_id: id, qty }, { 'Idempotency-Key': request.key })).data;
      }
      else {
        request = marketRequestKey();
        result = (await post<PurchaseResult>('/market/checkout', {}, { 'Idempotency-Key': request.key })).data;
      }
      await refreshRef.current();
      if (request) {
        sessionStorage.removeItem(request.storageKey);
        purchaseKey.current = '';
      }
      const reference = result.order_id || result.id || result.orders?.[0]?.id || '-';
      setError('');
      setBusy(null);
      setReceipt({ method, reference, paidAt: new Date().toLocaleString(lang === 'id' ? 'id-ID' : 'en-US') });
      return true;
    } catch (e) {
      setError(pesanError(e, t('pay.fail')));
      return false;
    } finally {
      finishing.current = false;
    }
  }, [id, kind, qty, valid, t, lang]);


  const payWallet = async () => {
    if (busy || !(await confirm({
      title: t('pay.confirmH'),
      body: t('pay.confirmP', { order: label, amount: rupiah(amount) }),
      confirmLabel: t('pay.confirm'),
      variant: 'default',
    }))) return;
    setBusy('wallet');
    setError('');
    await finishPurchase(t('pay.mWallet'));
    setBusy(null);
  };


  const payDoku = async () => {
    if (dokuInvoice) return;
    setBusy('doku');
    setError('');
    try {
      const next = new URLSearchParams(params.toString());
      next.delete('doku');
      next.delete('doku_invoice');
      const requestKey = sessionStorage.getItem(dokuRequestKey) || crypto.randomUUID();
      sessionStorage.setItem(dokuRequestKey, requestKey);
      const { data } = await post<DokuCheckout>('/wallet/doku/checkout', { amount, finish_path: `/payment?${next.toString()}` }, { 'Idempotency-Key': requestKey });
      if (!data?.invoice_number) throw new Error(t('pay.noUrl'));
      setDokuInvoice(data.invoice_number);
      setDokuCheckoutURL(data.checkout_url || '');
      sessionStorage.setItem(dokuStorageKey, data.invoice_number);
      if (!data.checkout_url) {
        setBusy('settling');
        return;
      }
      window.location.assign(data.checkout_url);
    } catch (e) {
      sessionStorage.removeItem(dokuRequestKey);
      setError(pesanError(e, t('pay.gatewayFail')));
      setBusy(null);
    }
  };

  const reopenDoku = () => {
    if (!dokuCheckoutURL) return;
    setBusy('doku');
    window.location.assign(dokuCheckoutURL);
  };

  const payUSDT = async () => {
    setBusy('usdt'); setError('');
    try {
      const { data } = await post<{ invoice_id: string; address: string; amount_usdt: string }>('/wallet/crypto/onchain', { amount });
      setCryptoInvoice({ id: data.invoice_id, address: data.address, amount: data.amount_usdt });
    } catch (e) { setError(pesanError(e, t('pay.cryptoDisabled'))); setBusy(null); }
  };

  const submitManualTopup = async () => {
    if (!proofURL || busy) return;
    setBusy('settling'); setError('');
    try {
      const { data } = await post<{ id: string }>('/wallet/topup', { amount, method: manualMethod, proof_url: proofURL });
      setReceipt({ method: manualMethod === 'qris' ? t('pay.manualQris') : t('pay.bankTransfer'), reference: data.id, paidAt: new Date().toLocaleString('id-ID'), pending: true });
    } catch (e) { setError(pesanError(e, t('pay.proofFail'))); }
    finally { setBusy(null); }
  };

  useEffect(() => {
    const invoice = dokuInvoice || (params.get('doku') === 'finish' ? params.get('doku_invoice') : '');
    if (!invoice || !valid) return;
    setBusy('settling');
    const timer = window.setInterval(async () => {
      try {
        const { data } = await (await import('axios')).default.get<{ status?: string; checkout_url?: string }>(`/api/wallet/doku/${encodeURIComponent(invoice)}`, { withCredentials: true });
        if (data.status === 'PENDING' && data.checkout_url) {
          setDokuCheckoutURL(data.checkout_url);
          setBusy('settling');
        }
        if (data.status === 'PAID') {
          window.clearInterval(timer);
          sessionStorage.removeItem(dokuStorageKey); sessionStorage.removeItem(dokuRequestKey);
          setDokuInvoice(''); setDokuCheckoutURL(''); setDokuReturning(false); await finishPurchase(t('pay.dokuCheckout'), invoice); setBusy(null);
        }
        if (data.status === 'FAILED' || data.status === 'EXPIRED') {
          window.clearInterval(timer);
          sessionStorage.removeItem(dokuStorageKey); sessionStorage.removeItem(dokuRequestKey);
          setDokuInvoice(''); setDokuCheckoutURL(''); setDokuReturning(false); setBusy(null); setError(t('pay.dokuFailed'));
        }
      } catch { /* provider settlement may still be pending */ }
    }, 3000);
    return () => window.clearInterval(timer);
  }, [dokuInvoice, dokuRequestKey, dokuStorageKey, finishPurchase, params, t, valid]);

  useEffect(() => {
    if (!cryptoInvoice) return;
    const timer = window.setInterval(async () => {
      try {
        const { data } = await (await import('axios')).default.get<{ status?: string }>(`/api/wallet/crypto/${cryptoInvoice.id}`, { withCredentials: true });
        if (data.status === 'PAID') { window.clearInterval(timer); await finishPurchase('USDT TRC-20'); setBusy(null); }
      } catch { /* watcher may still be processing */ }
    }, 5000);
    return () => window.clearInterval(timer);
  }, [cryptoInvoice, finishPurchase]);

  if (receipt) {
    const receiptPrimary = kind === 'package' ? '/dashboard' : kind === 'topup' ? '/wallet' : '/pesanan';
    const receiptSecondary = kind === 'package' ? '/pricing' : kind === 'topup' ? '/wallet' : '/market';
    return (
    <div className="mx-auto max-w-xl p-6 md:p-10">
      <section role="status" aria-live="polite" className="card p-6 text-center">
        <CheckCircle2 className="mx-auto mb-4 h-12 w-12 text-emerald-300" aria-hidden="true" />
        <p className={`label mb-2 ${receipt.pending ? 'text-amber-300' : 'text-emerald-300'}`}>{receipt.pending ? t('pay.pendingKicker') : t(kind === 'package' ? 'pay.packageActiveKicker' : 'pay.receiptKicker')}</p>
        <h1 className="text-2xl font-semibold tracking-tight">{receipt.pending ? t('pay.pendingH') : t(kind === 'package' ? 'pay.packageActiveH' : 'pay.receiptH')}</h1>
        <p className="mt-2 text-sm text-mute">{receipt.pending ? t('pay.pendingP') : t(kind === 'package' ? 'pay.packageActiveP' : 'pay.receiptP')}</p>
        <dl className="my-6 divide-y divide-border rounded-xl border border-border text-left text-sm">
          <div className="flex justify-between gap-4 p-3"><dt className="text-mute">{t('pay.order')}</dt><dd className="text-right font-medium">{label}</dd></div>
          <div className="flex justify-between gap-4 p-3"><dt className="text-mute">{t('pay.total')}</dt><dd className="font-semibold tabular-nums">{rupiah(amount)}</dd></div>
          <div className="flex justify-between gap-4 p-3"><dt className="text-mute">{t('pay.method')}</dt><dd>{receipt.method}</dd></div>
          <div className="flex justify-between gap-4 p-3"><dt className="text-mute">{t('pay.ref')}</dt><dd className="max-w-[65%] break-all font-mono text-xs">{receipt.reference}</dd></div>
          <div className="flex justify-between gap-4 p-3"><dt className="text-mute">{t('pay.time')}</dt><dd>{receipt.paidAt}</dd></div>
          <div className="flex justify-between gap-4 p-3"><dt className="text-mute">{t('pay.status')}</dt><dd className={`font-medium ${receipt.pending ? 'text-amber-300' : 'text-emerald-300'}`}>{receipt.pending ? t('pay.pendingStatus') : t('pay.paid')}</dd></div>
        </dl>
        <div className="flex flex-wrap justify-center gap-3">
          <Link href={receiptPrimary}><Button>{kind === 'package' ? t('pay.openDash') : kind === 'topup' ? t('pay.openWallet') : t('pay.viewOrders')}</Button></Link>
          <Link href={receiptSecondary}><Button variant="outline">{t('pay.back')}</Button></Link>
        </div>
      </section>
    </div>
  );
  }

  if (!valid) return <div className="card p-8 text-center"><p role="alert" className="text-danger">{t('pay.invalid')}</p></div>;

  if (dokuReturning || dokuInvoice || busy === 'settling') return (
    <div className="mx-auto max-w-xl p-6 md:p-10">
      <section role="status" aria-live="polite" aria-busy="true" className="card p-6 text-center">
        <LoaderCircle className="mx-auto mb-4 h-12 w-12 animate-spin text-cyan-300" aria-hidden="true" />
        <p className="label mb-2 text-cyan-300">{t('pay.dokuCheckout')}</p>
        <h1 className="text-2xl font-semibold tracking-tight">{t('pay.settlingShort')}</h1>
        <p className="mt-2 text-sm text-mute">{t('pay.dokuPreparing')}</p>
      </section>
    </div>
  );

  return (
    <div className="mx-auto max-w-2xl p-6 md:p-10">
      <header className="mb-8">
        <p className="label mb-2">{t('pay.kicker')}</p>
        <h1 className="text-2xl font-semibold tracking-tight">{t('pay.h')}</h1>
        <p className="mt-2 text-sm text-mute">{t('pay.sub')}</p>
      </header>

      <section className="card mb-6 p-5" aria-label={t('pay.summaryAria')}>
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <p className="text-xs uppercase tracking-wide text-mute">{t('pay.order')}</p>
            <p className="mt-1 truncate font-medium">{label}</p>
            {kind === 'product' && <p className="mt-1 text-xs text-mute">{t('pay.qty', { n: qty })}</p>}
          </div>
          <strong className="shrink-0 text-lg tabular-nums text-cyan-300">{rupiah(amount)}</strong>
        </div>
      </section>

      {error && <div role="alert" aria-live="assertive" className="mb-5 flex items-start gap-3 rounded-lg border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger"><XCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" /><span><strong className="block">{t('pay.failH')}</strong>{error}</span></div>}
      <div className="space-y-3">
        {kind === 'topup' && <details className="card group">
          <summary className="flex cursor-pointer list-none items-center gap-4 p-5"><span className="flex h-12 w-12 items-center justify-center rounded-full bg-amber-400/15 text-amber-300"><Wallet className="h-6 w-6" /></span><span className="min-w-0 flex-1"><span className="block font-semibold">{t('pay.manualTitle')}</span><span className="mt-1 block text-sm text-mute">{t('pay.manualSub')}</span></span><ChevronDown className="h-5 w-5 text-mute transition group-open:rotate-180" /></summary>
          <div className="space-y-4 border-t border-border p-5">
            <div className="grid grid-cols-2 gap-2"><Button type="button" variant={manualMethod === 'qris' ? 'default' : 'outline'} onClick={() => { setManualMethod('qris'); setProofURL(''); }}>QRIS</Button><Button type="button" variant={manualMethod === 'transfer' ? 'default' : 'outline'} onClick={() => { setManualMethod('transfer'); setProofURL(''); }}>{t('pay.bankTransfer')}</Button></div>
            {manualMethod === 'qris' ? <div className="rounded-lg border border-border p-4 text-center">{paymentMethods.data?.methods?.topup_qris_image_url ? <img src={paymentMethods.data.methods.topup_qris_image_url} alt="QRIS" className="mx-auto max-h-72 rounded-lg object-contain" /> : <p className="text-sm text-mute">{t('pay.qrisNotConfigured')}</p>}<p className="mt-3 text-sm font-medium">{paymentMethods.data?.methods?.topup_qris_merchant || t('pay.merchant')}</p><p className="mt-1 text-sm text-mute">{t('pay.qrisTotal', { amount: rupiah(qrisTotal), fee: rupiah(qrisFee) })}</p></div> : <dl className="rounded-lg border border-border p-4 text-sm"><div className="flex justify-between gap-4"><dt className="text-mute">{t('wal.bank')}</dt><dd className="font-medium">{paymentMethods.data?.methods?.topup_bank_name || '-'}</dd></div><div className="mt-2 flex justify-between gap-4"><dt className="text-mute">{t('pay.accountNo')}</dt><dd className="font-mono">{paymentMethods.data?.methods?.topup_bank_account || '-'}</dd></div><div className="mt-2 flex justify-between gap-4"><dt className="text-mute">{t('pay.accountHolder')}</dt><dd>{paymentMethods.data?.methods?.topup_bank_holder || '-'}</dd></div><div className="mt-2 flex justify-between gap-4 border-t border-border pt-2"><dt className="text-mute">{t('pay.nominal')}</dt><dd className="font-semibold text-cyan-300">{rupiah(amount)} · {t('pay.zeroFee')}</dd></div></dl>}
            {paymentMethods.err && <div role="alert" className="flex items-center justify-between gap-3 rounded-lg border border-danger/30 bg-danger/10 px-3 py-2 text-sm text-danger"><span>{paymentMethods.err}</span><button type="button" onClick={() => void paymentMethods.reload()} className="shrink-0 rounded-lg border border-danger/40 px-3 py-1.5 text-xs font-medium hover:bg-danger/10">{t('pay.retry')}</button></div>}
            <ImageField label={t('pay.proof')} value={proofURL} onChange={setProofURL} />
            <Button type="button" className="w-full" disabled={!proofURL || busy !== null} onClick={submitManualTopup}>{busy ? t('pay.sendingProof') : t('pay.submitProof')}</Button>
          </div>
        </details>}
        {kind !== 'topup' && <button type="button" onClick={payWallet} disabled={busy !== null || Boolean(dokuInvoice) || !dokuChecked} className="card lift flex w-full items-center gap-4 p-5 text-left disabled:opacity-50">
          <span className="flex h-12 w-12 items-center justify-center rounded-full bg-amber-400/15 text-amber-300"><Wallet className="h-6 w-6" /></span>
          <span className="min-w-0 flex-1"><span className="block font-semibold">{t('pay.n2mCoin')}</span><span className="mt-1 block text-sm text-mute">{t('pay.balanceAvailable', { amount: rupiah(user?.balance) })}</span></span>
          {busy === 'wallet' ? <LoaderCircle className="h-5 w-5 animate-spin text-cyan-300" /> : <span className="text-sm font-medium text-cyan-300">{t('pay.pay')}</span>}
        </button>}

        <button type="button" onClick={payUSDT} disabled={busy !== null || Boolean(dokuInvoice) || !dokuChecked} className="card lift flex w-full items-center gap-4 p-5 text-left disabled:opacity-50">
          <span className="flex h-12 w-12 items-center justify-center rounded-full bg-emerald-400/15"><img src="/payment-logos/tether-usdt.svg" alt="Tether USDT" className="h-7 w-7" /></span>
          <span className="min-w-0 flex-1"><span className="block font-semibold">USDT TRC-20</span><span className="mt-1 block text-sm text-mute">{t('pay.onchainP')}</span></span>
          {busy === 'usdt' ? <LoaderCircle className="h-5 w-5 animate-spin text-cyan-300" /> : <span className="text-sm font-medium text-cyan-300">{t('pay.pay')}</span>}
        </button>
        <UsdtInvoiceDialog invoice={cryptoInvoice} onClose={() => {}} />

        <details className="card group" open={busy === 'doku' || Boolean(dokuInvoice)}>
          <summary className="flex cursor-pointer list-none items-center gap-4 p-5"><span className="flex h-12 w-12 items-center justify-center rounded-full bg-cyan-400/15 text-cyan-300"><CreditCard className="h-6 w-6" /></span><span className="min-w-0 flex-1"><span className="block font-semibold">{t('pay.dokuCheckout')}</span><span className="mt-1 block text-sm text-mute">{t('pay.dokuNotice')}</span></span><ChevronDown className="h-5 w-5 text-mute transition group-open:rotate-180" /></summary>
          <div className="border-t border-border px-5 pb-5 pt-4"><p className="mb-4 text-sm text-mute">{t('pay.dokuChannels')}</p><Button onClick={payDoku} disabled={busy !== null || Boolean(dokuInvoice) || !dokuChecked} className="w-full">{!dokuChecked ? t('pay.activeCheck') : dokuInvoice || busy === 'doku' ? t('pay.settlingShort') : t('pay.openDoku')}</Button>{dokuInvoice && !dokuCheckoutURL && <p role="status" className="mt-3 text-sm text-mute">{t('pay.dokuPreparing')}</p>}{dokuInvoice && <Button variant="outline" onClick={reopenDoku} disabled={!dokuCheckoutURL || busy !== null} className="mt-3 w-full">{dokuCheckoutURL ? t('pay.reopen') : t('pay.oldInvoice')}</Button>}</div>
        </details>
      </div>
      <p className="mt-6 flex items-center justify-center gap-2 text-xs text-mute"><ShieldCheck className="h-4 w-4" aria-hidden="true" /> {t('pay.secure')}</p>
    </div>
  );
}

export default function PaymentPage() {
  const { t } = useT();
  return <Suspense fallback={<div className="p-10 text-center text-sm text-mute">{t('pay.loading')}</div>}><PaymentContent /></Suspense>;
}
