'use client';
// Reset password: token dari email + password baru. Tanpa token form tidak berguna,
// jadi keadaan itu ditampilkan sebagai pesan + jalan keluar, bukan form kosong.
import { useState, Suspense } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { post, pesanError } from '@/lib/http';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useT } from '@/lib/i18n';

function ResetForm() {
  const { t } = useT();
  const token = useSearchParams().get('token') || '';
  const [pass, setPass] = useState('');
  const [ulang, setUlang] = useState('');
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  const [err, setErr] = useState('');

  const kirim = async (e: React.FormEvent) => {
    e.preventDefault();
    if (pass !== ulang) { setErr(t('reset.mismatch')); return; }
    setBusy(true);
    setErr('');
    try {
      await post('/auth/reset', { token, password: pass });
      setDone(true);
    } catch (e) {
      setErr(pesanError(e, t('reset.invalid')));
    } finally {
      setBusy(false);
    }
  };

  if (!token) {
    return (
      <div className="space-y-5">
        <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">
          {t('reset.missing')}
        </p>
        <Link href="/lupa-password" className="block text-sm text-cyan-300 hover:text-cyan-400 font-medium">
          {t('reset.request')}
        </Link>
      </div>
    );
  }

  if (done) {
    return (
      <div className="space-y-5">
        <p role="status" className="text-sm text-cyan-200 bg-cyan-500/10 border border-cyan-400/30 rounded-lg px-4 py-3">
          {t('reset.saved')}
        </p>
        <Link href="/masuk" className="block text-sm text-cyan-300 hover:text-cyan-400 font-medium">
          {t('reset.toSignIn')}
        </Link>
      </div>
    );
  }

  return (
    <form onSubmit={kirim} className="space-y-5">
      <div className="space-y-2">
        <label htmlFor="pass" className="label">{t('reset.new')}</label>
        <Input id="pass" type="password" autoComplete="new-password" minLength={8}
          value={pass} onChange={(e) => setPass(e.target.value)} required placeholder={t('reset.newPh')} />
      </div>
      <div className="space-y-2">
        <label htmlFor="pass2" className="label">{t('reset.repeat')}</label>
        <Input id="pass2" type="password" autoComplete="new-password" minLength={8}
          value={ulang} onChange={(e) => setUlang(e.target.value)} required />
      </div>

      {err && (
        <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-3 py-2">{err}</p>
      )}

      <Button type="submit" className="w-full" disabled={busy || pass.length < 8 || ulang.length < 8}>
        {busy ? t('reset.saving') : t('reset.save')}
      </Button>
    </form>
  );
}

export default function ResetPasswordPage() {
  const { t } = useT();
  return (
    <div className="min-h-dvh flex items-center justify-center p-6">
      <div className="w-full max-w-sm">
        <div className="flex items-center gap-3 font-semibold tracking-tight mb-8">
          <img src="/node2mint-icon.ico" alt="" aria-hidden="true" className="w-8 h-8 rounded-lg object-contain" />
          Node2Mint
        </div>
        <h1 className="text-2xl font-semibold tracking-tight">{t('reset.title')}</h1>
        <p className="mt-1 text-sm text-mute mb-8">{t('reset.sub')}</p>
        <Suspense fallback={<div className="skel h-10 w-full" />}>
          <ResetForm />
        </Suspense>
      </div>
    </div>
  );
}
