'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { Menu } from 'lucide-react';
import { useAuth } from '@/lib/auth';
import { useT, stripLocale, useUrlPrefix, type Key } from '@/lib/i18n';
import LanguageMenu from '@/components/LanguageMenu';
import CartMenu from '@/components/CartMenu';

const NAV: [Key, string][] = [
  ['nav.market', '/market'],
  ['nav.about', '/about'],
  ['nav.pricing', '/pricing'],
  ['nav.blog', '/artikel'],
  ['nav.docs', '/docs'],
  ['nav.faq', '/faq'],
];

function active(path: string, href: string, hash: string) {
  const [targetPath, targetHash] = href.split('#');
  if (targetHash) return path === targetPath && hash === `#${targetHash}`;
  return path === href && !hash;
}

export default function PublicHeader({ scrolled: controlledScrolled }: { scrolled?: boolean }) {
  const { user } = useAuth();
  const { t } = useT();
  const path = stripLocale(usePathname());
  const prefix = useUrlPrefix();
  const nav = (target: string) => prefix + target;
  const [menu, setMenu] = useState(false);
  const [hash, setHash] = useState('');
  const [pageScrolled, setPageScrolled] = useState(false);
  const scrolled = controlledScrolled ?? pageScrolled;

  useEffect(() => {
    const syncHash = () => setHash(location.hash);
    syncHash();
    addEventListener('hashchange', syncHash);
    return () => removeEventListener('hashchange', syncHash);
  }, [path]);

  useEffect(() => {
    const onScroll = () => setPageScrolled(scrollY > 8);
    addEventListener('scroll', onScroll, { passive: true });
    onScroll();
    return () => removeEventListener('scroll', onScroll);
  }, []);

  // Halaman commerce publik dirender DI DALAM shell dashboard begitu pengguna
  // login (AppShell embeddedCommerce), jadi sidebar + topbar sudah jadi chrome.
  // Header ini akan jadi navbar kedua yang menutupi isi. /market memagari dari
  // halamannya; pagar di sini sekaligus menutup /pricing dan halaman commerce
  // berikutnya. Taruh setelah semua hook — early return sebelum hook = error.
  const embedded = path === '/pricing' || path === '/market' || path.startsWith('/market/');
  if (user && embedded) return null;

  // Saat dropdown menu terbuka header WAJIB solid + overflow-hidden — tanpa itu item menu menimpa teks halaman (bug).
  return (
    <header className={`fixed left-4 right-4 top-3 z-40 mx-auto max-w-6xl rounded-2xl transition-all ${menu ? 'overflow-hidden' : 'overflow-visible'} ${scrolled || menu ? 'border border-white/10 bg-[#08090e]/75 backdrop-blur-xl' : 'border border-transparent bg-transparent'}`}>
      <nav className="flex h-14 items-center justify-between gap-2 px-4 sm:px-5">
        <Link href={nav('/')} className="flex shrink-0 items-center gap-2 font-medium">
          <img src="/node2mint-logo.png" alt="" aria-hidden="true" className="h-7 w-7 rounded-lg object-contain" />
          <span className={`overflow-hidden whitespace-nowrap transition-all duration-300 ${scrolled ? 'max-w-0 opacity-0' : 'max-w-24 opacity-100'}`}>Node2Mint</span>
        </Link>
        <div className="hidden gap-5 xl:flex">
          {NAV.map(([key, href]) => (
            <Link className={`nav-link ${active(path, href, hash) ? 'nav-link-on' : ''}`} href={nav(href)} key={key} aria-current={active(path, href, hash) ? 'page' : undefined}>{t(key)}</Link>
          ))}
        </div>
        <div className="flex items-center gap-2">
          <CartMenu />
          <span className="hidden h-6 w-px bg-white/10 sm:block" aria-hidden="true" />
          <span className="hidden sm:block"><LanguageMenu /></span>
          <Link href={user ? '/dashboard' : '/masuk?mode=daftar'} className="shimmer inline-flex min-h-11 items-center rounded-lg bg-cyan-300 px-3 py-2 text-sm font-semibold text-[#071015]">{user ? t('nav.dashboard') : t('nav.signin')}</Link>
          <button type="button" onClick={() => setMenu(!menu)} aria-label={t('nav.menu')} aria-expanded={menu} className="grid h-11 w-11 place-items-center xl:hidden"><Menu className="h-5 w-5" /></button>
        </div>
      </nav>
      {menu && <div className="border-t border-white/10 bg-[#08090e]/95 p-3 backdrop-blur-xl xl:hidden">
        {NAV.map(([key, href]) => <Link onClick={() => setMenu(false)} aria-current={active(path, href, hash) ? 'page' : undefined} className={`flex min-h-11 items-center rounded-lg px-3 py-2 text-sm hover:bg-white/5 hover:text-text ${active(path, href, hash) ? 'text-text' : 'text-mute'}`} href={nav(href)} key={key}>{t(key)}</Link>)}
        <div className="border-t border-white/10 px-3 pt-3 sm:hidden">
          <p className="mb-2 text-xs font-medium text-mute">{t('nav.lang')}</p>
          <LanguageMenu mobile />
        </div>
      </div>}
    </header>
  );
}
