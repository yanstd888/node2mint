'use client';
// Gateway selesai OAuth -> redirect ke sini. Sesi sudah berbentuk cookie
// HttpOnly yang dipasang gateway, jadi halaman ini tidak menyimpan apa pun —
// cukup memuat ulang profil lalu lanjut.
import { useEffect, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/lib/auth';
import { useT } from '@/lib/i18n';

export default function AuthCallbackPage() {
  const { refresh } = useAuth();
  const { t } = useT();
  const router = useRouter();
  const [fail, setFail] = useState(false);

  useEffect(() => {
    const next = new URLSearchParams(location.search).get('next');
    const tujuan = next && next.startsWith('/') && !next.startsWith('//') ? next : '/dashboard';
    // Token di URL (sisa tautan lama/OAuth lama) dibuang dari riwayat sebelum apa pun.
    if (location.hash) history.replaceState(null, '', location.pathname + location.search);
    refresh().then((u) => {
      // refresh() menelan galat dan mengembalikan user; tanpa user = gagal masuk.
      if (u) router.replace(tujuan);
      else setFail(true);
    });
  }, [refresh, router]);

  if (!fail) return <main className="min-h-dvh grid place-items-center"><div className="skel h-8 w-40" /></main>;
  return (
    <main className="min-h-dvh grid place-items-center p-6">
      <div className="card p-6 max-w-sm text-center">
        <p role="alert" className="text-sm text-danger">{t('auth.cbFail')}</p>
        <Link href="/masuk" className="mt-4 inline-block text-sm text-cyan-300 hover:text-cyan-400">{t('auth.cbBack')}</Link>
      </div>
    </main>
  );
}
