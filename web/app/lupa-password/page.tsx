'use client';
import { useState } from 'react';
import Link from 'next/link';
import { post, pesanError } from '@/lib/http';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useT } from '@/lib/i18n';

export default function LupaPasswordPage() {
  const { t } = useT();
  const [email, setEmail] = useState('');
  const [busy, setBusy] = useState(false);
  const [sent, setSent] = useState(false);
  const [err, setErr] = useState('');

  const kirim = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr('');
    try {
      await post('/auth/forgot', { email: email.trim() });
      setSent(true);
    } catch (e) {
      setErr(pesanError(e, t('forgot.fail')));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="min-h-dvh flex items-center justify-center p-6">
      <div className="w-full max-w-sm">
        <div className="flex items-center gap-3 font-semibold tracking-tight mb-8">
          <img src="/node2mint-icon.ico" alt="" aria-hidden="true" className="w-8 h-8 rounded-lg object-contain" />
          Node2Mint
        </div>

        <h1 className="text-2xl font-semibold tracking-tight">{t('forgot.h')}</h1>
        <p className="mt-1 text-sm text-mute mb-8">{t('forgot.sub')}</p>

        {sent ? (
          <div className="space-y-6">
            <p role="status" className="text-sm text-cyan-200 bg-cyan-500/10 border border-cyan-400/30 rounded-lg px-4 py-3">
              {t('forgot.sent')}
            </p>
            <Link href="/masuk" className="block text-sm text-cyan-300 hover:text-cyan-400 font-medium">
              {t('forgot.signIn')}
            </Link>
            <Link href="/contact" className="block text-sm text-mute hover:text-text">{t('forgot.help')}</Link>
          </div>
        ) : (
          <form onSubmit={kirim} className="space-y-5">
            <div className="space-y-2">
              <label htmlFor="email" className="label">{t('forgot.email')}</label>
              <Input id="email" type="email" autoComplete="email" placeholder={t('forgot.placeholder')}
                value={email} onChange={(e) => setEmail(e.target.value)} required />
            </div>

            {err && (
              <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-3 py-2">{err}</p>
            )}

            <Button type="submit" className="w-full" disabled={busy || !email.trim()}>
              {busy ? t('forgot.sending') : t('forgot.send')}
            </Button>
            <Link href="/masuk" className="block text-sm text-mute hover:text-text text-center">
              {t('forgot.signIn')}
            </Link>
          </form>
        )}
      </div>
    </div>
  );
}
