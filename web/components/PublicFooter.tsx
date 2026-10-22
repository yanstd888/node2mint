'use client';

import Link from 'next/link';
import { Activity, BookOpen, Heart, Store } from 'lucide-react';
import { useT, useUrlPrefix, type Key } from '@/lib/i18n';
import { SUPPORT_EMAIL, supportMailto } from '@/lib/support';
import { useFetch } from '@/lib/useApi';

type FooterLink = { href: string; label: string; external?: boolean };
type CommunityResponse = { links?: Record<string, string> };

// Judul + label komunitas diketik sebagai Key supaya salah ketik jadi error kompilasi.
const COMMUNITY: [string, Key][] = [
  ['community_discord_url', 'foot.communityDiscord'],
  ['community_telegram_url', 'foot.communityTelegram'],
  ['community_x_url', 'foot.communityX'],
  ['community_youtube_url', 'foot.communityYoutube'],
];

export default function PublicFooter() {
  const { t } = useT();
  const prefix = useUrlPrefix();
  // Tautan internal ikut prefix locale halaman; '/#models' → '/<loc>#models'.
  const nav = (href: string) => (href.startsWith('/#') ? prefix + href.slice(1) : prefix + href);
  const supportEmail = `<!--email_off--><a href="${supportMailto(t('sup.subject'))}" class="font-mono text-white/70 transition-colors hover:text-white break-all">${SUPPORT_EMAIL}</a><!--/email_off-->`;
  const community = useFetch<CommunityResponse>('/public/community');
  const configuredCommunity = COMMUNITY.flatMap(([key, label]) => {
    const href = community.data?.links?.[key];
    return href ? [{ href, label: t(label), external: true }] : [];
  });
  // Grup tanpa tautan dibuang seluruhnya — judul kosong menyesatkan pembaca
  // layar dan menggeser kolom grid.
  const groups: Array<{ title: string; links: FooterLink[] }> = [
    { title: t('foot.product'), links: [
      { href: '/#models', label: t('foot.models') },
      { href: '/pricing', label: t('foot.pricing') }, { href: '/market', label: t('foot.market') },
    ] },
    { title: t('foot.learn'), links: [
      { href: '/guides', label: t('foot.guides') }, { href: '/docs', label: t('foot.docs') },
      { href: '/faq', label: t('foot.faq') }, { href: '/artikel', label: t('foot.articles') },
    ] },
    { title: t('foot.community'), links: configuredCommunity },
    { title: t('foot.company'), links: [
      { href: '/status', label: t('foot.status') }, { href: '/changelog', label: t('foot.changelog') },
      { href: '/terms', label: t('foot.terms') }, { href: '/privacy', label: t('foot.privacy') },
      { href: '/refund', label: t('foot.refund') }, { href: '/bantuan', label: t('foot.help') },
      { href: '/contact', label: t('sup.h') },
    ] },
  ].filter((group) => group.links.length > 0);

  return <footer className="overflow-hidden border-t border-white/10 bg-[#05060a] text-white">
    <div className="mx-auto max-w-6xl px-5 pb-0 pt-14 sm:px-6 sm:pt-16 lg:px-8">
      <div className="grid gap-12 lg:grid-cols-[1.55fr_repeat(4,minmax(0,1fr))] lg:gap-8">
        <section className="max-w-xs" aria-label="Node2Mint">
          <Link href={nav('/')} className="inline-flex min-h-11 items-center gap-2 text-[15px] font-semibold tracking-tight text-white"><img src="/node2mint-logo.png" alt="" aria-hidden="true" className="h-5 w-5 rounded-[5px] object-contain" />Node2Mint</Link>
          <p className="mt-5 text-sm leading-6 text-white/50">{t('foot.about')}</p>
          <div className="mt-7 flex gap-2">
            <Link href={nav('/docs')} aria-label={t('foot.openDocs')} className="grid h-11 w-11 place-items-center rounded-xl border border-white/10 text-white/60 transition-colors hover:border-white/25 hover:text-white"><BookOpen size={16} strokeWidth={1.8} aria-hidden="true" /></Link>
            <Link href={nav('/status')} aria-label={t('foot.openStatus')} className="grid h-11 w-11 place-items-center rounded-xl border border-white/10 text-white/60 transition-colors hover:border-white/25 hover:text-white"><Activity size={16} strokeWidth={1.8} aria-hidden="true" /></Link>
            <Link href={nav('/market')} aria-label={t('foot.openMarket')} className="grid h-11 w-11 place-items-center rounded-xl border border-white/10 text-white/60 transition-colors hover:border-white/25 hover:text-white"><Store size={16} strokeWidth={1.8} aria-hidden="true" /></Link>
          </div>
        </section>
        <nav aria-label={t('foot.nav')} className="grid grid-cols-2 gap-x-7 gap-y-10 sm:grid-cols-4 lg:col-span-4 lg:gap-x-8">
          {groups.map((group) => <div key={group.title}>
            <h2 className="text-[10px] font-bold uppercase tracking-[0.13em] text-white/90">{group.title}</h2>
            <ul className="mt-3 space-y-0" role="list">{group.links.map((link) => <li key={`${group.title}-${link.href}-${link.label}`}>
              {link.external ? <a href={link.href} target="_blank" rel="noreferrer" className="inline-flex min-h-11 items-center text-sm text-white/50 transition-colors hover:text-white">{link.label}</a> : <Link href={nav(link.href)} className="inline-flex min-h-11 items-center text-sm text-white/50 transition-colors hover:text-white">{link.label}</Link>}
            </li>)}</ul>
          </div>)}
        </nav>
      </div>
      <div className="mt-12 border-t border-white/10 py-6 text-xs text-white/55 sm:mt-14 sm:flex sm:items-center sm:justify-between"><span>{t('foot.copyright')}</span><span className="mt-2 flex items-center gap-1 sm:mt-0">{t('foot.signature')} <span aria-label={t('foot.creditAria')} className="inline-flex items-center gap-1">{t('foot.credit')} <Heart size={13} className="fill-rose-400 text-rose-400" aria-hidden="true" /></span></span></div>
      <p className="pb-6 text-xs text-white/55">{t('sup.emailH')}: <span dangerouslySetInnerHTML={{ __html: supportEmail }} /></p>
    </div>
    <div aria-hidden="true" className="mx-auto -mb-[0.21em] max-w-[1320px] select-none px-4 text-center text-[clamp(4.75rem,14vw,12rem)] font-bold leading-none tracking-[-0.095em] text-white/[0.50] sm:px-6">NODE2MINT</div>
  </footer>;
}
