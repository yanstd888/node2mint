import type { ReactNode } from 'react';
import { headers } from 'next/headers';
import Link from 'next/link';
import PublicHeader from '@/components/PublicHeader';
import PublicFooter from '@/components/PublicFooter';
import { SUPPORT_EMAIL, supportMailto } from '@/lib/support';

const languageNotice = {
  en: 'This English text is provided for convenience. If it conflicts with the Indonesian text, the Indonesian text controls.',
  ru: 'Юридически обязательной является индонезийская версия этого текста. Русский перевод не опубликован; до использования перевода обратитесь в поддержку.',
} as const;

export default async function LegalLayout({ title, subtitle, children }: { title: string; subtitle?: string; children: ReactNode }) {
  const locale = (await headers()).get('x-locale');
  const notice = locale === 'en' || locale === 'ru' ? languageNotice[locale] : null;

  return <div className="min-h-dvh">
    <PublicHeader />
    <main className="mx-auto max-w-3xl px-6 pb-16 pt-28 md:px-10">
      <h1 className="text-3xl font-semibold tracking-tight">{title}</h1>
      <p className="mt-2 text-sm text-mute">{locale === 'en' ? 'Effective as of September 30, 2026 · Indonesia' : 'Berlaku sejak 30 September 2026 · Indonesia'}</p>
      {notice && <p className="mt-3 rounded-lg border border-border bg-surface p-3 text-sm text-mute">{notice}</p>}
      {subtitle && <p className="mt-1 text-sm text-mute">{subtitle}</p>}
      <p className="mt-3 text-sm">
        <a href={supportMailto(`${title} — ${locale === 'en' ? 'questions' : 'pertanyaan'}`)} className="font-medium text-cyan-300 hover:text-cyan-200 break-all">{SUPPORT_EMAIL}</a>
        {' · '}
        <Link href="/contact" className="text-cyan-300 hover:text-cyan-200">{locale === 'en' ? 'Help Center' : 'Pusat bantuan'}</Link>
      </p>
      <div className="mt-10 space-y-8 text-sm leading-7 text-mute">{children}</div>
    </main>
    <PublicFooter />
  </div>;
}
