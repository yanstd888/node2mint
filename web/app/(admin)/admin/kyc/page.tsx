'use client';

// Admin · KYC verification: approve/reject with confirmations (destructive:
// reject denies a user's identity proof). Reuses the shared useConfirm dialog.
import { useFetch } from '@/lib/useApi';
import { post, pesanError } from '@/lib/http';
import { useToast } from '@/components/ui/toast';
import { useConfirm } from '@/components/ui/confirm';
import { useT } from '@/lib/i18n';
import { Button } from '@/components/ui/button';
import { useState } from 'react';

type Request = { id: string; user_id: string; email: string; doc_type: string; original_name: string; mime_type: string; created_at: string };

export default function AdminKYCPage() {
  const { data, loading, err, reload } = useFetch<{ requests?: Request[] }>('/admin/kyc');
  const { t } = useT();
  const toast = useToast();
  const confirm = useConfirm();
  const [busy, setBusy] = useState('');
  const items = data?.requests || [];

  const decide = async (id: string, email: string, action: 'approve' | 'reject') => {
    let reason = '';
    if (action === 'reject') {
      reason = window.prompt(t('ad.kyc.reason'))?.trim() || '';
      if (reason.length < 3) return;
    }
    const confirmed = await confirm({
      title: action === 'approve' ? t('ad.kyc.approveOf', { email }) : t('ad.kyc.rejectOf', { email }),
      body: action === 'reject' ? `${email} — ${reason}` : `${email} — ${t('ad.kyc.approve')}`,
      confirmLabel: action === 'approve' ? t('ad.kyc.approve') : t('ad.kyc.reject'),
      variant: action === 'reject' ? 'destructive' : 'default',
    });
    if (!confirmed) return;
    setBusy(id);
    try {
      await post(`/admin/kyc/${id}/${action}`, action === 'reject' ? { reason } : {});
      toast.ok(action === 'approve' ? t('ad.kyc.approved') : t('ad.kyc.rejected'));
      await reload();
    } catch (e) {
      toast.err(pesanError(e, action === 'approve' ? t('ad.kyc.approveFail') : t('ad.kyc.rejectFail')));
    } finally {
      setBusy('');
    }
  };

  return <div className="mx-auto max-w-6xl p-6 md:p-10">
    <header className="mb-8">
      <p className="label mb-1">{t('ad.common.admin')}</p>
      <h1 className="text-2xl font-semibold tracking-tight">{t('ad.kyc.h')}</h1>
      <p className="mt-2 text-sm text-mute">{t('ad.kyc.sub')}</p>
    </header>
    {err && <p role="alert" className="banner banner-bad">{err}</p>}
    {loading ? <div className="card p-6">Loading…</div> : items.length === 0 ? <div className="card p-8 text-center text-mute">{t('ad.kyc.empty')}</div> : <div className="space-y-3">{items.map((x) => <div key={x.id} className="card flex flex-wrap items-center gap-4 p-4">
      <div className="min-w-0 flex-1">
        <p className="font-medium">{x.email}</p>
        <p className="text-sm text-mute">{x.doc_type} · {x.original_name} · {x.mime_type}</p>
      </div>
      <a className="button-outline" href={`/api/admin/kyc/${x.id}/document`} target="_blank" rel="noreferrer">View document</a>
      <Button size="sm" variant="default" disabled={busy === x.id} onClick={() => void decide(x.id, x.email, 'approve')}>{t('ad.kyc.approve')}</Button>
      <Button size="sm" variant="outline" disabled={busy === x.id} onClick={() => void decide(x.id, x.email, 'reject')}>{t('ad.kyc.reject')}</Button>
    </div>)}</div>}
  </div>;
}
