'use client';

import { useState, type ReactNode } from 'react';
import { LockKeyhole, Send } from 'lucide-react';
import { post, pesanError } from '@/lib/http';
import { useFetch } from '@/lib/useApi';
import { Button } from '@/components/ui/button';
import Link from 'next/link';
import { useT, useUrlPrefix } from '@/lib/i18n';
import { useAuth } from '@/lib/auth';

type Status = { verified?: boolean; state?: string };

export function LockedOverlay({ children, feature }: { children: ReactNode; feature: string }) {
  const { lang, t } = useT();
  const prefix = useUrlPrefix();
  const { user } = useAuth();
  const status = useFetch<Status>('/telegram/status', !user);
  const [busy, setBusy] = useState<'link' | 'verify' | null>(null);
  const [error, setError] = useState('');
  const locked = !status.data?.verified;
  const id = lang === 'id';
  const copy = status.data?.state === 'locked_left_group'
    ? (id ? 'Kamu keluar dari grup Telegram. Join ulang lalu verifikasi.' : 'You left the Telegram group. Rejoin, then verify again.')
    : status.data?.state === 'locked_tg_taken'
      ? (id ? 'Akun Telegram ini sudah pernah dipakai verifikasi di akun lain.' : 'This Telegram account is already verified on another account.')
      : (id ? 'Kamu belum verifikasi keanggotaan grup Telegram.' : 'You have not verified Telegram group membership.');
  const join = async () => {
    setBusy('link'); setError('');
    try {
      const { data } = await post<{ url?: string }>('/telegram/link', {});
      if (!data?.url) throw new Error('link unavailable');
      window.open(data.url, '_blank', 'noopener,noreferrer');
    } catch (e) { setError(pesanError(e, id ? 'Gagal membuat tautan Telegram.' : 'Could not create the Telegram link.')); }
    finally { setBusy(null); }
  };
  const verify = async () => {
    setBusy('verify'); setError('');
    try { await post('/telegram/verify', {}); await status.reload(); }
    catch (e) { setError(pesanError(e, id ? 'Verifikasi gagal, coba lagi dalam beberapa menit.' : 'Verification failed. Try again in a few minutes.')); await status.reload(); }
    finally { setBusy(null); }
  };
  if (!locked) return <>{children}</>;
  return <div className="relative">
    <div className="pointer-events-none select-none blur-[8px]" aria-hidden="true">{children}</div>
    <section className="absolute inset-0 z-10 grid place-items-center p-4" aria-label={id ? 'Akses Telegram terkunci' : 'Telegram access locked'}>
      <div className="max-w-sm rounded-2xl border border-cyan-400/30 bg-[#15161a]/95 p-5 text-center shadow-2xl backdrop-blur">
        <span className="mx-auto grid h-11 w-11 place-items-center rounded-xl bg-cyan-400/15 text-cyan-200"><LockKeyhole className="h-5 w-5" aria-hidden="true" /></span>
        <h2 className="mt-4 font-semibold">{id ? 'Buka akses token gratis' : 'Unlock free-token access'}</h2>
        <p className="mt-2 text-sm leading-6 text-mute">{id ? 'Join grup Telegram @node2mint untuk buka akses API key gratis.' : 'Join the @node2mint Telegram group to unlock free API access.'}</p>
        {feature === 'api_free' && <p className="mt-3 text-[11px] font-semibold uppercase tracking-[.12em] text-cyan-200">{t('models.joinToUnlock')}</p>}
        <p className="mt-2 text-xs text-amber-200">{copy}</p>
        {error && <p role="alert" className="mt-3 text-xs text-danger">{error}</p>}
        <div className="mt-4 grid gap-2 sm:grid-cols-2">
          {!user ? <Link href={`${prefix}/masuk`} className="button-outline">{id ? 'Masuk untuk unlock' : 'Sign in to unlock'}</Link> : <Button type="button" variant="outline" disabled={busy !== null} onClick={() => void join()}><Send className="h-4 w-4" aria-hidden="true" />{busy === 'link' ? '…' : id ? 'Join Telegram' : 'Join Telegram'}</Button>}
          {user && <Button type="button" disabled={busy !== null} onClick={() => void verify()}>{busy === 'verify' ? '…' : id ? 'Saya Sudah Join — Verifikasi' : 'I Joined — Verify'}</Button>}
        </div>
      </div>
    </section>
  </div>;
}
