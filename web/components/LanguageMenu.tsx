'use client';

import { useRef } from 'react';
import { ChevronDown } from 'lucide-react';
import { type Lang, useT } from '@/lib/i18n';

const LANGUAGES: { code: Lang; flag: string }[] = [
  { code: 'en', flag: '/flags/us.svg' },
  { code: 'id', flag: '/flags/id.svg' },
  { code: 'cn', flag: '/flags/cn.svg' },
  { code: 'ru', flag: '/flags/ru.svg' },
  { code: 'jp', flag: '/flags/jp.svg' },
];

export default function LanguageMenu({ tone = 'public', mobile = false }: { tone?: 'public' | 'app'; mobile?: boolean }) {
  const { lang, setLang, t } = useT();
  const ref = useRef<HTMLDetailsElement>(null);
  const active = LANGUAGES.find((item) => item.code === lang) ?? LANGUAGES[0];
  const app = tone === 'app';

  // Mobile nav lives inside an overflow-hidden header, so an absolute dropdown
  // is clipped. Render the choices inline instead.
  if (mobile) return (
    <div role="group" aria-label={t('nav.lang')} className="grid grid-cols-5 gap-1">
      {LANGUAGES.map((item) => (
        <button key={item.code} type="button" onClick={() => setLang(item.code)} aria-pressed={lang === item.code}
          className={`flex min-h-11 items-center justify-center gap-1 rounded-lg border text-xs font-semibold uppercase focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60 ${lang === item.code ? 'border-cyan-300 bg-cyan-300 text-[#071015]' : 'border-white/10 text-mute hover:bg-white/10 hover:text-text'}`}>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={item.flag} alt="" aria-hidden="true" className="h-3.5 w-5 rounded-[2px] object-cover" />
          {item.code}
        </button>
      ))}
    </div>
  );

  return (
    <details ref={ref} className="group relative">
      <summary aria-label={t('nav.lang')} className={`flex h-9 cursor-pointer list-none items-center gap-2 rounded-lg border px-2.5 text-xs font-semibold uppercase tracking-wide [&::-webkit-details-marker]:hidden focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60 ${app ? 'border-border text-text hover:bg-surface-hi' : 'border-white/10 text-text hover:bg-white/10'}`}>
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img src={active.flag} alt="" aria-hidden="true" className="h-3.5 w-5 rounded-[2px] object-cover shadow-sm" />
        {active.code}
        <ChevronDown aria-hidden="true" className="h-3.5 w-3.5 text-mute transition-transform group-open:rotate-180" />
      </summary>
      <div className={`absolute right-0 top-11 z-50 w-32 rounded-lg border p-1 shadow-xl ${app ? 'border-border bg-surface' : 'border-white/10 bg-[#0b0d13]'}`}>
        {LANGUAGES.map((item) => (
          <button key={item.code} type="button" onClick={() => { setLang(item.code); ref.current?.removeAttribute('open'); }} aria-current={lang === item.code ? 'true' : undefined}
            className={`flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-xs font-semibold uppercase transition-colors ${lang === item.code ? (app ? 'bg-white/10 text-text' : 'bg-cyan-300 text-[#071015]') : 'text-mute hover:bg-white/10 hover:text-text'}`}>
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src={item.flag} alt="" aria-hidden="true" className="h-3.5 w-5 rounded-[2px] object-cover shadow-sm" />
            {item.code}
          </button>
        ))}
      </div>
    </details>
  );
}
