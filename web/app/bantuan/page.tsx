'use client';
// Pusat Bantuan: satu tempat untuk Docs, Artikel, dan Status Layanan (brief §1A).
// Isi dipinjam dari halaman publiknya (components/sections/*); route lama tetap jalan.
// F-06: kartu kontak di bawah supaya halaman ini bukan jalan buntu — juri QA
// menemukan 0 form/input/mailto di sini, jadi tak ada cara menghubungi siapa pun.
import { useState } from 'react';
import Link from 'next/link';
import { BookOpen, Newspaper, Activity, Mail } from 'lucide-react';
import { DocsSection } from '@/components/sections/DocsSection';
import { ArtikelSection } from '@/components/sections/ArtikelSection';
import { StatusSection } from '@/components/sections/StatusSection';
import { useT, useUrlPrefix, type Key } from '@/lib/i18n';
import PublicHeader from '@/components/PublicHeader';
import PublicFooter from '@/components/PublicFooter';
import { SUPPORT_EMAIL, supportMailto } from '@/lib/support';

const TABS = [
  { id: 'docs', label: 'nav.docs', icon: BookOpen },
  { id: 'artikel', label: 'art.h', icon: Newspaper },
  { id: 'status', label: 'st.h', icon: Activity },
] as const satisfies readonly { id: string; label: Key; icon: typeof BookOpen }[];

type TabId = (typeof TABS)[number]['id'];

export default function BantuanPage() {
  const { t } = useT();
  const prefix = useUrlPrefix();
  const [tab, setTab] = useState<TabId>('docs');
  return (
    <div className="min-h-dvh">
      <PublicHeader />
      <main className="p-6 pt-24 md:p-10 md:pt-28 max-w-4xl mx-auto">
        <p className="label mb-1">{t('hlp.kicker')}</p>
        <h1 className="text-2xl font-semibold tracking-tight mb-6">{t('hlp.h')}</h1>

        <div role="tablist" aria-label={t('hlp.tabsAria')}
          className="flex gap-1 mb-6 bg-surface border border-border rounded-xl p-1 w-fit">
          {TABS.map((x) => (
            <button key={x.id} role="tab" aria-selected={tab === x.id} onClick={() => setTab(x.id)}
              className={`flex items-center gap-2 px-4 h-9 rounded-lg text-sm font-medium transition-colors
                focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60 ${
                  tab === x.id ? 'bg-cyan-500/15 text-cyan-300' : 'text-mute hover:text-text'
                }`}>
              <x.icon className="w-4 h-4" aria-hidden="true" /> {t(x.label)}
            </button>
          ))}
        </div>

        {tab === 'docs' && <DocsSection />}
        {tab === 'artikel' && <ArtikelSection />}
        {tab === 'status' && <StatusSection />}

        <section id="kontak" className="card mt-10 p-6">
          <h2 className="flex items-center gap-2 font-medium text-text">
            <Mail className="h-4 w-4 text-cyan-300" aria-hidden="true" />{t('sup.h')}
          </h2>
          <p className="mt-2 text-sm leading-6 text-mute">{t('sup.sub')}</p>
          <p className="mt-3 text-sm leading-6 text-mute">{t('sup.emailP', { email: SUPPORT_EMAIL })}</p>
          <p className="mt-2 text-xs leading-6 text-mute">{t('sup.hoursP')}</p>
          <p className="mt-4 flex flex-wrap items-center gap-3">
            <a href={supportMailto('Permintaan dukungan Node2Mint')} className="font-mono text-sm text-cyan-300 hover:text-cyan-200 break-all">{SUPPORT_EMAIL}</a>
            <Link href={`${prefix}/contact`} className="text-sm text-cyan-300 hover:text-cyan-200">{t('sup.resetBtn')}</Link>
          </p>
        </section>
      </main>
      <PublicFooter />
    </div>
  );
}
