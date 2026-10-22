'use client';

import Link from 'next/link';
import { useState } from 'react';
import { ChevronDown } from 'lucide-react';
import PublicHeader from '@/components/PublicHeader';
import PublicFooter from '@/components/PublicFooter';
import { useT, useUrlPrefix, type Key } from '@/lib/i18n';

const answers: [Key, Key][] = [
  ['faqPage.q1', 'faqPage.a1'],
  ['faqPage.q2', 'faqPage.a2'],
  ['faqPage.q3', 'faqPage.a3'],
  ['faqPage.q4', 'faqPage.a4'],
  ['faqPage.q5', 'faqPage.a5'],
  ['faqPage.q6', 'faqPage.a6'],
  ['faqPage.q7', 'faqPage.a7'],
  ['faqPage.q8', 'faqPage.a8'],
  ['faqPage.q9', 'faqPage.a9'],
  ['faqPage.q10', 'faqPage.a10'],
  ['faqPage.q11', 'faqPage.a11'],
  ['faqPage.q12', 'faqPage.a12'],
];

export default function FAQPage() {
  const { t } = useT();
  const prefix = useUrlPrefix();
  const [open, setOpen] = useState<number | null>(0);
  return <div className="min-h-dvh">
    <PublicHeader />
    <main className="mx-auto max-w-4xl px-6 pb-16 pt-28 md:px-10">
      <p className="label mb-1">{t('faq.kicker')}</p>
      <h1 className="text-3xl font-semibold tracking-tight">{t('faqPage.h')}</h1>
      <dl className="mt-10 divide-y divide-border border-y border-border">
        {answers.map(([question, answer], index) => {
          const active = open === index;
          const panelId = `faq-page-panel-${index}`;
          return <div key={question} className="border-b border-border last:border-b-0">
            <dt>
              <button
                type="button"
                onClick={() => setOpen(active ? null : index)}
                className="flex min-h-16 w-full items-center justify-between gap-5 py-5 text-left text-sm font-semibold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60 sm:text-base"
                aria-expanded={active}
                aria-controls={panelId}
              >
                <span>{t(question)}</span>
                <span className={`grid h-9 w-9 shrink-0 place-items-center rounded-full border transition-colors ${active ? 'border-white bg-white text-black' : 'border-white/15 text-mute'}`}>
                  <ChevronDown className={`h-4 w-4 transition-transform duration-300 ${active ? 'rotate-180' : ''}`} aria-hidden="true" />
                </span>
              </button>
            </dt>
            <dd id={panelId} aria-hidden={!active} className={`grid transition-[grid-template-rows] duration-300 ${active ? 'grid-rows-[1fr]' : 'grid-rows-[0fr]'}`}>
              <div className="min-h-0 overflow-hidden">
                <p className="max-w-3xl pb-6 pr-14 text-sm leading-6 text-mute">{t(answer)}</p>
              </div>
            </dd>
          </div>;
        })}
      </dl>
      <p className="mt-8 text-sm text-mute">{t('faqPage.help')} <Link href={`${prefix}/bantuan`} className="text-cyan-300 hover:text-cyan-200">{t('faqPage.helpLink')}</Link>.</p>
      {/* F-03/F-06: jalur kontak harus bisa diklik, bukan teks mati. */}
      <p className="mt-3 text-sm text-mute">{t('faqPage.support')}{' '}
        <Link href={`${prefix}/contact`} className="text-cyan-300 hover:text-cyan-200">{t('sup.h')}</Link>
      </p>
    </main>
    <PublicFooter />
  </div>;
}
