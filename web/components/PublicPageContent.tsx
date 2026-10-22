'use client';

import Link from 'next/link';
import PublicFooter from '@/components/PublicFooter';
import PublicHeader from '@/components/PublicHeader';
import { useT, useUrlPrefix, type Key } from '@/lib/i18n';

const guideKeys: readonly [Key, Key, string][] = [
  ['guides.apiH', 'guides.apiP', '/docs'],
  ['guides.balanceH', 'guides.balanceP', '/pricing'],
  ['guides.marketH', 'guides.marketP', '/market'],
  ['guides.sellH', 'guides.sellP', '/toko'],
  ['guides.partnerH', 'guides.partnerP', '/referral'],
  ['guides.securityH', 'guides.securityP', '/pengaturan'],
];

export function PricingPageIntro() {
  const { t } = useT();

  return <>
    <header className="mb-12 max-w-3xl">
      <p className="label mb-2">{t('pricingPage.kicker')}</p>
      <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">{t('pricingPage.h')}</h1>
      <p className="mt-4 text-sm leading-relaxed text-mute sm:text-base">{t('pricingPage.sub')}</p>
    </header>
    <section aria-labelledby="how-it-works-title" className="mb-16">
      <p className="label mb-2">{t('pricingPage.howKicker')}</p>
      <h2 id="how-it-works-title" className="text-2xl font-semibold tracking-tight sm:text-3xl">{t('pricingPage.howH')}</h2>
      <ol className="mt-6 grid gap-4 md:grid-cols-3">
        <li className="rounded-xl border border-white/10 p-5 text-sm leading-relaxed text-mute"><strong className="block text-text">{t('pricingPage.step1H')}</strong><span className="mt-2 block">{t('pricingPage.step1P')}</span></li>
        <li className="rounded-xl border border-white/10 p-5 text-sm leading-relaxed text-mute"><strong className="block text-text">{t('pricingPage.step2H')}</strong><span className="mt-2 block">{t('pricingPage.step2P')}</span></li>
        <li className="rounded-xl border border-white/10 p-5 text-sm leading-relaxed text-mute"><strong className="block text-text">{t('pricingPage.step3H')}</strong><span className="mt-2 block">{t('pricingPage.step3P', { endpoint: '/v1/chat/completions' })}</span></li>
      </ol>
    </section>
    <section aria-labelledby="use-cases-title" className="mb-16">
      <p className="label mb-2">{t('pricingPage.useCasesKicker')}</p>
      <h2 id="use-cases-title" className="text-2xl font-semibold tracking-tight sm:text-3xl">{t('pricingPage.useCasesH')}</h2>
      <div className="mt-6 grid gap-4 md:grid-cols-3">
        <p className="rounded-xl border border-white/10 p-5 text-sm text-mute"><strong className="block text-text">{t('pricingPage.case1H')}</strong><span className="mt-2 block">{t('pricingPage.case1P')}</span></p>
        <p className="rounded-xl border border-white/10 p-5 text-sm text-mute"><strong className="block text-text">{t('pricingPage.case2H')}</strong><span className="mt-2 block">{t('pricingPage.case2P')}</span></p>
        <p className="rounded-xl border border-white/10 p-5 text-sm text-mute"><strong className="block text-text">{t('pricingPage.case3H')}</strong><span className="mt-2 block">{t('pricingPage.case3P')}</span></p>
      </div>
    </section>
  </>;
}

export function PricingPageExample() {
  const { t } = useT();

  return <section aria-labelledby="example-title" className="mb-16">
    <p className="label mb-2">{t('pricingPage.exampleKicker')}</p>
    <h2 id="example-title" className="text-2xl font-semibold tracking-tight sm:text-3xl">{t('pricingPage.exampleH')}</h2>
    <pre className="mt-6 overflow-x-auto rounded-xl border border-white/10 bg-black/30 p-5 text-sm leading-relaxed text-cyan-100"><code>{`curl https://www.node2mint.com/v1/chat/completions \\
  -H "Authorization: Bearer $NODE2MINT_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"PUBLISHED_MODEL_ID","messages":[{"role":"user","content":"Hello"}]}'`}</code></pre>
    <p className="mt-3 text-xs text-mute">{t('pricingPage.exampleP')}</p>
  </section>;
}

export function GuidesPageContent() {
  const { t } = useT();
  const prefix = useUrlPrefix();

  return <div className="min-h-dvh">
    <PublicHeader />
    <main className="mx-auto max-w-4xl px-6 pb-16 pt-28 md:px-10">
      <h1 className="text-3xl font-semibold tracking-tight">{t('guides.h')}</h1>
      <p className="mt-3 max-w-2xl text-sm leading-6 text-mute">{t('guides.sub')}</p>
      <ul className="mt-10 divide-y divide-border border-y border-border" role="list">
        {guideKeys.map(([title, description, href]) => <li key={href} className="py-5 first:pt-0 last:pb-0">
          <Link href={`${prefix}${href}`} className="block rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60"><h2 className="font-medium text-text">{t(title)}</h2><p className="mt-1 text-sm leading-6 text-mute">{t(description)}</p></Link>
        </li>)}
      </ul>
      <p className="mt-8 text-sm text-mute">{t('guides.help')} <Link href={`${prefix}/contact`} className="text-cyan-300 hover:text-cyan-200">{t('guides.helpLink')}</Link>.</p>
    </main>
    <PublicFooter />
  </div>;
}

export function ChangelogPageContent() {
  const { t } = useT();

  return <div className="min-h-dvh">
    <PublicHeader />
    <main className="mx-auto max-w-4xl px-6 pb-16 pt-28 md:px-10">
      <h1 className="text-3xl font-semibold tracking-tight">{t('changelog.h')}</h1>
      <section className="mt-10 border-l-2 border-cyan-400/50 pl-5">
        <h2 className="font-medium text-text">{t('changelog.settlementH')}</h2>
        <p className="mt-2 text-sm leading-6 text-mute">{t('changelog.settlementP')}</p>
      </section>
      <section className="mt-8 border-l-2 border-border pl-5">
        <h2 className="font-medium text-text">{t('changelog.referralH')}</h2>
        <p className="mt-2 text-sm leading-6 text-mute">{t('changelog.referralP')}</p>
      </section>
    </main>
    <PublicFooter />
  </div>;
}
