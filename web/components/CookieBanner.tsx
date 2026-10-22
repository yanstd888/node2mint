'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useT, useUrlPrefix, stripLocale } from '@/lib/i18n';

const CONSENT_KEY = 'node2mint.privacy-consent';

export default function CookieBanner() {
  // Setelah stripLocale, /id dan /en sama-sama jadi '/' (banner hanya di home).
  const pathname = stripLocale(usePathname());
  const { t } = useT();
  const prefix = useUrlPrefix();
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    if (pathname === '/') setVisible(localStorage.getItem(CONSENT_KEY) !== 'accepted');
  }, [pathname]);

  if (pathname !== '/' || !visible) return null;

  const accept = () => {
    localStorage.setItem(CONSENT_KEY, 'accepted');
    setVisible(false);
  };

  return (
    <aside
      role="dialog"
      aria-label={t('privacy.cookieTitle')}
      className="fixed inset-x-3 bottom-3 z-50 mx-auto max-w-3xl rounded-2xl border border-white/10 bg-bg/95 p-3 shadow-2xl backdrop-blur-md sm:inset-x-6 sm:bottom-4 sm:p-5"
    >
      <div className="flex items-center gap-3 sm:gap-4 sm:justify-between">
        <div className="min-w-0">
          <p className="text-sm font-medium">{t('privacy.cookieTitle')}</p>
          <p className="mt-1 text-[11px] leading-snug text-mute sm:text-xs sm:leading-relaxed">
            {t('privacy.cookieBody')} <Link href={`${prefix}/privacy`} className="text-cyan-300 underline underline-offset-2">{t('foot.privacy')}</Link>.
          </p>
        </div>
        <button type="button" onClick={accept} className="min-h-10 shrink-0 rounded-lg bg-cyan-300 px-3 text-xs font-medium text-[#071015] hover:bg-cyan-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60 sm:min-h-11 sm:px-4 sm:text-sm">
          {t('privacy.cookieAccept')}
        </button>
      </div>
    </aside>
  );
}
