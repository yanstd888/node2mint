'use client';
// F-06 (juri QA): jalur bantuan yang benar-benar bisa dipakai. Help, Refund,
// lupa-password, Terms, Privacy, dan 404 semuanya menunjuk ke sini supaya tidak
// ada lagi "hubungi admin" tanpa tujuan. Satu kartu email + satu daftar isi yang
// harus disertakan; tidak ada form/tiket baru karena tidak ada mailbox inbound.
import { useState } from 'react';
import Link from 'next/link';
import { Mail, Copy, Check, Clock, Lock, LifeBuoy } from 'lucide-react';
import PublicHeader from '@/components/PublicHeader';
import PublicFooter from '@/components/PublicFooter';
import { Button } from '@/components/ui/button';
import { useT, useUrlPrefix } from '@/lib/i18n';
import { SUPPORT_EMAIL, supportMailto } from '@/lib/support';

export default function ContactPage() {
  const { t } = useT();
  const prefix = useUrlPrefix();
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(SUPPORT_EMAIL);
      setCopied(true);
      setTimeout(() => setCopied(false), 1600);
    } catch {
      // Clipboard bisa ditolak (izin/koneksi tidak aman) — emailnya tetap terlihat sebagai teks.
      setCopied(false);
    }
  };

  return (
    <div className="min-h-dvh">
      <PublicHeader />
      <main className="mx-auto max-w-3xl px-6 pb-16 pt-28 md:px-10">
        <p className="label mb-1">{t('sup.kicker')}</p>
        <h1 className="text-3xl font-semibold tracking-tight">{t('sup.h')}</h1>
        <p className="mt-2 text-sm leading-6 text-mute">{t('sup.sub')}</p>

        <section className="card mt-8 p-6">
          <h2 className="flex items-center gap-2 font-medium text-text">
            <Mail className="h-4 w-4 text-cyan-300" aria-hidden="true" />{t('sup.emailH')}
          </h2>
          <p className="mt-2 text-sm leading-6 text-mute">{t('sup.emailP', { email: SUPPORT_EMAIL })}</p>
          <p className="mt-4">
            <a href={supportMailto('Permintaan dukungan Node2Mint')} className="font-mono text-sm text-cyan-300 hover:text-cyan-200 break-all">{SUPPORT_EMAIL}</a>
          </p>
          <div className="mt-4 flex flex-wrap gap-3">
            <a href={supportMailto('Permintaan dukungan Node2Mint')}><Button>{t('sup.emailCta')}</Button></a>
            <Button variant="outline" onClick={copy}>
              {copied ? <Check className="h-4 w-4 text-emerald-300" aria-hidden="true" /> : <Copy className="h-4 w-4" aria-hidden="true" />}
              {t('sup.copyEmail')}
            </Button>
          </div>
          <p role="status" aria-live="polite" className="mt-2 min-h-5 text-xs text-emerald-300">{copied ? t('sup.copied') : ''}</p>
        </section>

        <section className="card mt-6 p-6">
          <h2 className="flex items-center gap-2 font-medium text-text">
            <LifeBuoy className="h-4 w-4 text-cyan-300" aria-hidden="true" />{t('sup.needH')}
          </h2>
          <ul className="mt-3 list-disc space-y-1 pl-5 text-sm leading-6 text-mute">
            <li>{t('sup.need1')}</li>
            <li>{t('sup.need2')}</li>
            <li>{t('sup.need3')}</li>
            <li>{t('sup.need4')}</li>
          </ul>
        </section>

        <section className="card mt-6 p-6">
          <h2 className="flex items-center gap-2 font-medium text-text">
            <Clock className="h-4 w-4 text-cyan-300" aria-hidden="true" />{t('sup.hoursH')}
          </h2>
          <p className="mt-2 text-sm leading-6 text-mute">{t('sup.hoursP')}</p>
          <h3 className="mt-6 font-medium text-text">{t('sup.escalateH')}</h3>
          <p className="mt-2 text-sm leading-6 text-mute">{t('sup.escalateP')}</p>
        </section>

        <section className="card mt-6 p-6">
          <h2 className="flex items-center gap-2 font-medium text-text">
            <Lock className="h-4 w-4 text-cyan-300" aria-hidden="true" />{t('sup.resetH')}
          </h2>
          <p className="mt-2 text-sm leading-6 text-mute">{t('sup.resetP')}</p>
          <a href={supportMailto('Permintaan pengaturan ulang kata sandi Node2Mint')} className="mt-4 inline-block"><Button variant="outline">{t('sup.resetBtn')}</Button></a>
        </section>

        <p className="mt-8 flex flex-wrap gap-x-4 gap-y-2 text-sm">
          <Link href={`${prefix}/bantuan`} className="text-cyan-300 hover:text-cyan-200">{t('sup.docs')}</Link>
          <Link href={`${prefix}/faq`} className="text-cyan-300 hover:text-cyan-200">{t('sup.faq')}</Link>
          <Link href={`${prefix}/status`} className="text-cyan-300 hover:text-cyan-200">{t('sup.status')}</Link>
        </p>
      </main>
      <PublicFooter />
    </div>
  );
}
