'use client';
// Next 16 memakai berkas ini untuk URL yang tidak cocok (lihat experimental.globalNotFound).
// Berkas ini MENGGANTIKAN root layout, jadi <html>/<body> ditulis di sini dan
// LocaleProvider harus dipasang sendiri sebelum useT() bisa dipakai.
import { LocaleProvider, useT } from '@/lib/i18n';
import './globals.css';

function Pesan() {
  const { t } = useT();
  return (
    <main className="grid min-h-dvh place-items-center p-6">
      <div className="card w-full max-w-md p-10 text-center">
        <p className="label mb-2">{t('nf.code')}</p>
        <h1 className="text-xl font-semibold tracking-tight">{t('nf.h')}</h1>
        <p className="mt-2 text-sm leading-6 text-mute">{t('nf.p')}</p>
        {/* <a> biasa, bukan next/link: halaman ini tidak selalu punya router context. */}
        <p className="mt-6 flex flex-wrap items-center justify-center gap-2">
          <a href="/" className="rounded-xl bg-cyan-500 px-4 py-2 text-sm font-medium text-bg hover:bg-cyan-400">{t('nf.home')}</a>
          <a href="/bantuan" className="rounded-xl border border-border px-4 py-2 text-sm font-medium text-text hover:bg-surface-hi">{t('nf.help')}</a>
        </p>
      </div>
    </main>
  );
}

export default function GlobalNotFound() {
  return (
    <html lang="id" className="dark">
      <body className="min-h-dvh bg-bg font-sans text-text antialiased">
        <LocaleProvider><Pesan /></LocaleProvider>
      </body>
    </html>
  );
}
