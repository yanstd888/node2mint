'use client';

import { useState } from 'react';
import { Check, Copy, LoaderCircle } from 'lucide-react';
import { Dialog } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { useT } from '@/lib/i18n';

type Invoice = { id: string; address: string; amount: string };

export function UsdtInvoiceDialog({ invoice, onClose }: { invoice: Invoice | null; onClose: () => void }) {
  const { t } = useT();
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    if (!invoice) return;
    try { await navigator.clipboard.writeText(invoice.address); setCopied(true); }
    catch { setCopied(false); }
  };
  return <Dialog open={!!invoice} title={t('pay.cryptoOnchain')} onClose={onClose}>
    {invoice && <div className="space-y-4 text-center">
      <p className="text-sm text-mute">{t('pay.cryptoAmount')} <b className="text-text">{invoice.amount} USDT</b> melalui {t('pay.cryptoNetwork')} TRC-20.</p>
      <img src={`/api/wallet/crypto/${encodeURIComponent(invoice.id)}/qr`} alt={t('pay.cryptoQrAlt')} className="mx-auto h-56 w-56 rounded-lg bg-white p-2" />
      <p className="break-all rounded-lg border border-border bg-surface-hi p-3 font-mono text-xs">{invoice.address}</p>
      <Button onClick={copy} variant="outline" className="w-full">{copied ? <Check className="mr-2 h-4 w-4" /> : <Copy className="mr-2 h-4 w-4" />} {copied ? t('pay.copiedAddress') : t('pay.copyAddress')}</Button>
      <p role="status" className="flex items-center justify-center gap-2 text-sm text-cyan-300"><LoaderCircle className="h-4 w-4 animate-spin" /> {t('pay.cryptoWaiting')}</p>
      <p className="text-xs text-mute">{t('pay.cryptoRule')}</p>
    </div>}
  </Dialog>;
}
