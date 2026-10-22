'use client';
// URL yang tidak ada. Sebelumnya tidak ada berkas not-found sama sekali →
// Next memakai halaman bawaan yang polos dan tidak berbahasa.
import Link from 'next/link';
import { Button } from '@/components/ui/button';
import StatusMessage from '@/components/StatusMessage';
import { useT } from '@/lib/i18n';
import { SUPPORT_EMAIL, supportMailto } from '@/lib/support';

export default function NotFound() {
  const { t } = useT();
  return (
    <StatusMessage code={t('nf.code')} title={t('nf.h')} body={t('nf.p')}>
      <Link href="/"><Button>{t('nf.home')}</Button></Link>
      <Link href="/bantuan"><Button variant="outline">{t('nf.help')}</Button></Link>
      <a href={supportMailto(t('nf.subject'))} className="w-full text-sm text-mute">{SUPPORT_EMAIL}</a>
      </StatusMessage>
  );
}
