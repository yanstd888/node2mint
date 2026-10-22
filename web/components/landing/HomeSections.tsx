'use client';

import Link from 'next/link';
import { ArrowRight, BadgeDollarSign, BookOpen, Coins, UserPlus, WalletCards } from 'lucide-react';
import { useT, useUrlPrefix } from '@/lib/i18n';

export function TokenWalletSection() {
  const { t } = useT();
  const prefix = useUrlPrefix();

  return (
    <section aria-labelledby="ledger-title" className="border-y border-white/10 py-16 sm:py-24">
      <div className="grid gap-10 lg:grid-cols-[1.05fr_.95fr] lg:items-center lg:gap-16">
        <div>
          <p className="label mb-2">{t('home.ledgerKicker')}</p>
          <h2 id="ledger-title" className="max-w-xl text-3xl font-semibold tracking-tight sm:text-4xl">{t('home.ledgerH')}</h2>
          <p className="mt-4 max-w-xl text-sm leading-relaxed text-mute sm:text-base">{t('home.ledgerP')}</p>
          <Link href={`${prefix}/pricing`} className="mt-7 inline-flex min-h-11 items-center gap-2 text-sm font-medium text-cyan-300 hover:text-cyan-200">
            {t('pack.cta')} <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Link>
        </div>

      <div className="mx-auto grid max-w-[640px] divide-y divide-white/10 border-y border-white/10">
        <div className="grid gap-3 py-6 sm:grid-cols-[auto_1fr] sm:gap-5">
          <span className="grid h-11 w-11 place-items-center rounded-xl bg-emerald-400/10 text-emerald-300">
            <WalletCards className="h-5 w-5" aria-hidden="true" />
          </span>
          <div>
            <div className="font-medium text-text">{t('home.walletH')}</div>
            <div className="mt-1 text-sm leading-relaxed text-mute">{t('home.walletP')}</div>
          </div>
        </div>
        <div className="grid gap-3 py-6 sm:grid-cols-[auto_1fr] sm:gap-5">
          <span className="grid h-11 w-11 place-items-center rounded-xl bg-cyan-400/10 text-cyan-300">
            <Coins className="h-5 w-5" aria-hidden="true" />
          </span>
          <div>
            <div className="font-medium text-text">{t('home.tokensH')}</div>
            <div className="mt-1 text-sm leading-relaxed text-mute">{t('home.tokensP')}</div>
          </div>
        </div>
        <div className="grid gap-3 py-6 sm:grid-cols-[auto_1fr] sm:gap-5">
          <span className="grid h-11 w-11 place-items-center rounded-xl bg-amber-400/10 text-amber-300">
            <BadgeDollarSign className="h-5 w-5" aria-hidden="true" />
          </span>
          <div>
            <div className="font-medium text-text">{t('home.cacheH')}</div>
            <div className="mt-1 text-sm leading-relaxed text-mute">{t('home.cacheP')}</div>
          </div>
        </div>
      </div>
      </div>
    </section>
  );
}

export function EarnSection({ signedIn }: { signedIn: boolean }) {
  const { t } = useT();
  const entry = signedIn ? null : '/masuk?mode=daftar';

  return (
    <section id="earn" aria-labelledby="earn-title" className="scroll-mt-24 py-16 sm:py-24">
      <div className="max-w-3xl">
        <p className="label mb-2">{t('home.earnKicker')}</p>
        <h2 id="earn-title" className="text-3xl font-semibold tracking-tight sm:text-4xl">{t('home.earnH')}</h2>
        <p className="mt-4 max-w-2xl text-sm leading-relaxed text-mute sm:text-base">{t('home.earnP')}</p>
      </div>

      <div className="mt-10 grid border-y border-white/10 lg:grid-cols-[1.15fr_.85fr] lg:divide-x lg:divide-white/10">
        <article className="py-8 lg:pr-10">
          <BadgeDollarSign className="h-7 w-7 text-emerald-300" aria-hidden="true" />
          <h3 className="mt-7 text-2xl font-semibold">{t('home.affH')}</h3>
          <p className="mt-3 max-w-xl text-sm leading-relaxed text-mute">{t('home.affP')}</p>
          <p className="mt-5 text-sm font-medium text-emerald-300">{t('home.affReward')}</p>
          <Link href={entry ?? '/afiliasi'} className="mt-7 inline-flex min-h-11 items-center gap-2 rounded-lg border border-emerald-300/30 px-4 text-sm font-medium text-emerald-100 hover:bg-emerald-300/10">
            {t('home.affCta')} <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Link>
        </article>

        <article className="border-t border-white/10 py-8 lg:border-t-0 lg:pl-10">
          <UserPlus className="h-7 w-7 text-cyan-300" aria-hidden="true" />
          <h3 className="mt-7 text-2xl font-semibold">{t('home.refH')}</h3>
          <p className="mt-3 max-w-xl text-sm leading-relaxed text-mute">{t('home.refP')}</p>
          <p className="mt-5 text-sm font-medium text-cyan-300">{t('home.refReward')}</p>
          <Link href={entry ?? '/referral'} className="mt-7 inline-flex min-h-11 items-center gap-2 rounded-lg border border-cyan-300/30 px-4 text-sm font-medium text-cyan-100 hover:bg-cyan-300/10">
            {t('home.refCta')} <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Link>
        </article>
      </div>
    </section>
  );
}

export function FinalCta({ signedIn }: { signedIn: boolean }) {
  const { t } = useT();
  const prefix = useUrlPrefix();
  return (
    <section aria-labelledby="final-cta-title" className="py-16 sm:py-24">
      <div className="border-y border-cyan-300/20 py-12 sm:py-16">
        <div className="grid gap-8 lg:grid-cols-[1fr_auto] lg:items-end">
          <div>
            <h2 id="final-cta-title" className="max-w-3xl text-3xl font-semibold tracking-tight sm:text-5xl">{t('home.finalH')}</h2>
            <p className="mt-4 max-w-2xl text-sm leading-relaxed text-mute sm:text-base">{t('home.finalP')}</p>
          </div>
          <div className="flex flex-col gap-3 sm:flex-row">
            <Link href={signedIn ? '/dashboard' : '/masuk?mode=daftar'} className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-cyan-400 px-5 py-3 text-sm font-semibold text-slate-950 transition-colors hover:bg-cyan-300">
              {signedIn ? t('hero.ctaIn') : t('home.finalCta')} <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </Link>
            <Link href={`${prefix}/docs`} className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl border border-white/15 px-5 py-3 text-sm font-medium text-text transition-colors hover:border-cyan-300/40">
              <BookOpen className="h-4 w-4" aria-hidden="true" /> {t('home.finalDocs')}
            </Link>
          </div>
        </div>
      </div>
    </section>
  );
}
