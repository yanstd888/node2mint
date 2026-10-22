'use client';
// Error route (500 saat render client). `reset` mengulang render segmen ini —
// tidak ada permintaan uang di jalur ini, jadi aman untuk ditawarkan.
import Link from 'next/link';
import { Button } from '@/components/ui/button';
import StatusMessage from '@/components/StatusMessage';
import { useT } from '@/lib/i18n';

export default function ErrorRoute({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  const { t } = useT();
  return (
    <StatusMessage code={t('e5.code')} title={t('e5.h')} body={t('e5.p')} danger>
      <Button onClick={reset}>{t('e5.retry')}</Button>
      <Link href="/"><Button variant="outline">{t('e5.home')}</Button></Link>
    </StatusMessage>
  );
}
