'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { ArrowRight, Bot, ShieldCheck, Store, WalletCards } from 'lucide-react';
import PublicFooter from '@/components/PublicFooter';
import PublicHeader from '@/components/PublicHeader';
import { useT, useUrlPrefix } from '@/lib/i18n';

const pillars = [
  { icon: Bot, title: 'about.apiH', body: 'about.apiP' },
  { icon: Store, title: 'about.marketH', body: 'about.marketP' },
  { icon: WalletCards, title: 'about.accountH', body: 'about.accountP' },
] as const;

export default function AboutPage() {
  const { t } = useT();
  const prefix = useUrlPrefix();
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);

  return (
    <div className="min-h-dvh overflow-hidden">
      {mounted && <PublicHeader />}
      <main className="mx-auto max-w-6xl px-6 pb-20 pt-32 md:px-10 md:pt-36">
        <section className="max-w-3xl">
          <p className="label mb-3">{t('about.kicker')}</p>
          <h1 className="text-4xl font-semibold leading-[.96] tracking-[-.045em] sm:text-6xl">{t('about.h')}</h1>
          <p className="mt-6 max-w-2xl text-base leading-7 text-mute sm:text-lg">{t('about.sub')}</p>
        </section>

        <section className="mt-16 grid gap-4 md:grid-cols-3" aria-label={t('about.pillarsAria')}>
          {pillars.map(({ icon: Icon, title, body }) => (
            <article key={title} className="card p-6">
              <Icon className="h-5 w-5 text-cyan-300" aria-hidden="true" />
              <h2 className="mt-6 text-lg font-semibold">{t(title)}</h2>
              <p className="mt-3 text-sm leading-6 text-mute">{t(body)}</p>
            </article>
          ))}
        </section>

        <section className="mt-16 grid gap-10 border-y border-white/10 py-12 lg:grid-cols-[.75fr_1.25fr] lg:gap-20">
          <div>
            <p className="label mb-2">{t('about.trustKicker')}</p>
            <h2 className="max-w-md text-3xl font-semibold tracking-[-.035em] sm:text-4xl">{t('about.trustH')}</h2>
          </div>
          <div className="max-w-2xl">
            <div className="flex items-start gap-4">
              <ShieldCheck className="mt-1 h-5 w-5 shrink-0 text-cyan-300" aria-hidden="true" />
              <p className="text-base leading-7 text-mute">{t('about.trustP')}</p>
            </div>
            <div className="mt-8 flex flex-wrap gap-3">
              <Link href={`${prefix}/market`} className="inline-flex items-center gap-2 rounded-lg bg-cyan-300 px-4 py-2.5 text-sm font-semibold text-[#071015] hover:bg-cyan-200">
                {t('about.marketCta')} <ArrowRight className="h-4 w-4" aria-hidden="true" />
              </Link>
              <Link href={`${prefix}/contact`} className="inline-flex items-center gap-2 rounded-lg border border-border px-4 py-2.5 text-sm font-medium hover:bg-surface-hi">
                {t('about.contactCta')}
              </Link>
            </div>
          </div>
        </section>
      </main>
      {mounted && <PublicFooter />}
    </div>
  );
}
