'use client';

import { useState, type ReactNode } from 'react';
import { ArrowRight, ChevronDown } from 'lucide-react';
import { useAuth } from '@/lib/auth';
import { useT, useUrlPrefix, type Key } from '@/lib/i18n';
import { asList } from '@/lib/utils';
import HomeHero from '@/components/landing/HomeHero';
import ModelCards from '@/components/landing/ModelCards';

import Showcase from '@/components/landing/Showcase';
import LatestArticles from '@/components/landing/LatestArticles';
import Ticker from '@/components/landing/Ticker';
import { EarnSection, FinalCta, TokenWalletSection } from '@/components/landing/HomeSections';
import PublicFooter from '@/components/PublicFooter';
import PublicHeader from '@/components/PublicHeader';
import GatewayWaitlist from '@/components/landing/GatewayWaitlist';

const FAQ: [Key, Key][] = [
  ['faq.q1', 'faq.a1'],
  ['faq.q2', 'faq.a2'],
  ['faq.q3', 'faq.a3'],
  ['faq.q4', 'faq.a4'],
  ['faq.q5', 'faq.a5'],
  ['faq.q6', 'faq.a6'],
];

function FaqSection() {
  const { t } = useT();
  const prefix = useUrlPrefix();
  const [open, setOpen] = useState<number | null>(0);
  const faqSchema = {
    '@context': 'https://schema.org',
    '@type': 'FAQPage',
    mainEntity: FAQ.map(([q, a]) => ({
      '@type': 'Question',
      name: t(q),
      acceptedAnswer: { '@type': 'Answer', text: t(a) },
    })),
  };

  return (
    <section id="faq" aria-labelledby="faq-title" className="scroll-mt-24 py-16 sm:py-24">
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(faqSchema).replaceAll('<', '\\u003c') }}
      />
      <div className="grid gap-12 lg:grid-cols-[.72fr_1.28fr] lg:gap-16">
        <div className="lg:sticky lg:top-28 lg:self-start">
          <h2 id="faq-title" className="max-w-sm text-4xl font-semibold leading-[.92] tracking-[-.04em] sm:text-5xl">{t('faq.h')}</h2>
          <div className="mt-7 max-w-sm border-t border-white/10 pt-5">
            <p className="text-sm leading-relaxed text-mute">{t('faq.sub')}</p>
            <div className="mt-5 flex flex-wrap gap-x-6 gap-y-3 text-sm">
              <a href={`${prefix}/docs`} className="inline-flex items-center gap-2 font-medium hover:text-cyan-200">
                {t('faq.docs')} <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
              </a>
              <a href={`${prefix}/status`} className="inline-flex items-center gap-2 text-mute hover:text-text">
                {t('faq.status')} <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
              </a>
            </div>
          </div>
        </div>
        <div className="border-t border-white/10">
          {FAQ.map(([q, a], index) => {
            const active = open === index;
            const panelId = `faq-panel-${index}`;
            return (
              <div className="border-b border-white/10" key={q}>
                <button
                  type="button"
                  onClick={() => setOpen(active ? null : index)}
                  className="flex min-h-16 w-full items-center justify-between gap-5 py-5 text-left text-sm font-semibold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60 sm:text-base"
                  aria-expanded={active}
                  aria-controls={panelId}
                >
                  <span>{t(q)}</span>
                  <span className={`grid h-11 w-11 shrink-0 place-items-center rounded-full border transition-colors ${active ? 'border-white bg-white text-black' : 'border-white/15 text-mute'}`}>
                    <ChevronDown className={`h-4 w-4 transition-transform duration-300 ${active ? 'rotate-180' : ''}`} aria-hidden="true" />
                  </span>
                </button>
                <div id={panelId} className={`grid transition-[grid-template-rows] duration-300 ${active ? 'grid-rows-[1fr]' : 'grid-rows-[0fr]'}`}>
                  <div className="min-h-0 overflow-hidden">
                    <p className="max-w-2xl pb-6 pr-14 text-sm leading-relaxed text-mute">{t(a)}</p>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}

// Bagian interaktif homepage. `initialModels` = katalog yang sudah dirender server
// (app/page.tsx) supaya HTML pertama memuat nama model + context window. Kedua
// komponen tetap poll /models sendiri, jadi perubahan live tidak hilang.
export default function HomeClient({ initialModels = null, initialFeatured = null, initialBestSellers = null, initialStatus = null, pricingCards }: { initialModels?: unknown; initialFeatured?: unknown; initialBestSellers?: unknown; initialStatus?: unknown; pricingCards: ReactNode }) {
  const { user } = useAuth();
  const signedIn = Boolean(user);
  const live = Boolean((initialStatus as { gateway_live?: boolean } | null)?.gateway_live);
  type HeroProduct = { is_demo?: boolean };

  const heroProducts = { products: [
    ...asList<HeroProduct>(initialFeatured, 'products'),
    ...asList<HeroProduct>(initialBestSellers, 'products'),
  ].slice(0, 2) };

  return (
    <div className="min-h-dvh overflow-hidden">
      <PublicHeader />
      <main className="mx-auto max-w-6xl px-4 sm:px-6">
        <HomeHero signedIn={signedIn} gatewayLive={live} initialModels={initialModels} initialProducts={heroProducts} />
        {live ? <>
          <Ticker />
          <Showcase initialBestSellers={initialBestSellers} />
          <ModelCards initialModels={initialModels} />
        </> : <GatewayWaitlist />}
        <TokenWalletSection />
        {pricingCards}
        <EarnSection signedIn={signedIn} />
        <LatestArticles />
        <FaqSection />
        <FinalCta signedIn={signedIn} />
      </main>
      <PublicFooter />
    </div>
  );
}
