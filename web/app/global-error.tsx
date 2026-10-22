'use client';

import Link from 'next/link';
import { LocaleProvider, useT } from '@/lib/i18n';

function GlobalErrorContent({ reset }: { reset: () => void }) {
  const { t } = useT();
  return <main style={{ display: 'grid', minHeight: '100dvh', placeItems: 'center', padding: 24 }}>
    <div style={{ maxWidth: 420, textAlign: 'center' }}>
      <p style={{ fontSize: 12, letterSpacing: '.12em', color: '#9aa3b8', textTransform: 'uppercase' }}>{t('e5.code')}</p>
      <h1 style={{ fontSize: 20, margin: '8px 0' }}>{t('e5.h')}</h1>
      <p style={{ fontSize: 14, lineHeight: 1.6, color: '#9aa3b8' }}>{t('e5.p')}</p>
      <p style={{ marginTop: 24, display: 'flex', gap: 12, justifyContent: 'center' }}>
        <button onClick={reset} style={{ cursor: 'pointer', borderRadius: 12, border: 0, background: '#67e8f9', color: '#071015', padding: '10px 16px', fontSize: 14, fontWeight: 600 }}>{t('e5.retry')}</button>
        <Link href="/" style={{ borderRadius: 12, border: '1px solid rgb(255 255 255 / .15)', color: '#f4f7fb', padding: '10px 16px', fontSize: 14, textDecoration: 'none' }}>{t('e5.home')}</Link>
      </p>
    </div>
  </main>;
}

export default function GlobalError({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <html lang="id" className="dark">
      <body style={{ margin: 0, background: '#08090e', color: '#f4f7fb', fontFamily: 'system-ui, sans-serif' }}>
        <LocaleProvider><GlobalErrorContent reset={reset} /></LocaleProvider>
      </body>
    </html>
  );
}