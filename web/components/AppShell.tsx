'use client';
// Shell (brief §1 + Fanny #2/#3/#4): sidebar tetap di desktop + header atas permanen
// (toggle collapse kiri, lang toggle + avatar dropdown kanan). Mode admin otomatis di
// /admin/* — sidebar admin aksen amber + guard role. Off-canvas + topbar di mobile.
// Sembunyi di /masuk dan landing /.
import { useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { Menu, PanelLeft, ChevronDown, LogOut, Settings, Tag, Wallet } from 'lucide-react';
import { useAuth } from '@/lib/auth';
import { useT, stripLocale } from '@/lib/i18n';
import { rupiah } from '@/lib/utils';
import dynamic from 'next/dynamic';
import Sidebar from '@/components/Sidebar';

const AdminSidebar = dynamic(() => import('@/components/AdminSidebar'), { ssr: false });
import CartMenu from '@/components/CartMenu';
import NotificationBell from '@/components/NotificationBell';
import LanguageMenu from '@/components/LanguageMenu';

const COLLAPSE_KEY = 'ui.sidebar.collapsed';

export default function AppShell({ children }: { children: React.ReactNode }) {
  // usePathname() memuat prefix locale (/id, /en) — daftar path di bawah tanpa
  // prefix. Tanpa stripLocale, /id tidak cocok '/' sehingga halaman publik
  // dianggap terproteksi: guard melempar ke /masuk, dan /masuk (juga bukan
  // path PUBLIK) dirender null. Itu penyebab layar blank di mobile.
  const pathname = stripLocale(usePathname());
  const router = useRouter();
  const { user, loading, logout } = useAuth();
  const { t } = useT();
  const [open, setOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(false);
  const menuButtonRef = useRef<HTMLButtonElement>(null);
  const avatarRef = useRef<HTMLDetailsElement>(null);
  // Publik tanpa shell: halaman auth dan halaman status publik (dipakai pengunjung
  // yang belum punya akun). Baris guard ini dipatok oleh kontrak tugas — jangan diubah.
  const PUBLIK = ['/masuk', '/', '/lupa-password', '/reset-password', '/bantuan', '/status', '/docs', '/pricing', '/artikel', '/market', '/about', '/guides', '/faq', '/changelog', '/terms', '/privacy', '/refund', '/contact', '/auth/callback'];
  const isPublicArticle = pathname.startsWith('/artikel/') || pathname.startsWith('/market/');
  // Pricing/market tetap publik untuk tamu, tetapi pengguna login membukanya
  // di dalam shell dashboard seperti Pusat Bantuan.
  const embeddedCommerce = !!user && (pathname === '/pricing' || pathname === '/market' || pathname.startsWith('/market/'));
  const showShell = embeddedCommerce || (!PUBLIK.includes(pathname) && !isPublicArticle);
  // Mode admin (Fanny #2): /admin/* punya sidebar + guard sendianya, tanpa state mode global.
  const adminMode = pathname.startsWith('/admin');
  const isAdmin = user?.role === 'ADMIN' || user?.role === 'SUPER_ADMIN';

  // Collapse (Fanny #3): persist di localStorage; tablet auto-collapse sekali di mount
  // TANPA menulis storage, supaya pilihan desktop tidak ketimpa.
  useEffect(() => {
    const tablet = window.matchMedia('(min-width: 768px) and (max-width: 1024px)');
    setCollapsed(localStorage.getItem(COLLAPSE_KEY) === '1' || tablet.matches);
  }, []);
  const toggleCollapse = () => setCollapsed((c) => {
    localStorage.setItem(COLLAPSE_KEY, c ? '0' : '1');
    return !c;
  });

  // Guard tunggal: route selain /masuk & landing wajib login, biar tidak blank page.
  // Admin route: role non-admin dipantulkan ke dashboard.
  useEffect(() => {
    if (!showShell || loading || !user) {
      if (showShell && !loading && !user) router.replace('/masuk');
      return;
    }
    if (adminMode && !isAdmin) router.replace('/dashboard');
  }, [showShell, loading, user, adminMode, isAdmin, router]);

  // Drawer mobile terbuka: overlay menutupi halaman, kunci scroll body supaya
  // konten di belakang tidak ikut bergerak.
  useEffect(() => {
    document.body.classList.toggle('overflow-hidden', open);
    return () => document.body.classList.remove('overflow-hidden');
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const previousFocus = document.activeElement as HTMLElement | null;
    const closeOnKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { event.preventDefault(); setOpen(false); return; }
      if (event.key !== 'Tab') return;
      const drawer = document.querySelector<HTMLElement>('[data-mobile-drawer="true"]');
      const focusable = drawer?.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), input:not([disabled]), select, textarea, [tabindex]:not([tabindex="-1"])',
      );
      if (!focusable || focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener('keydown', closeOnKey);
    const drawer = document.querySelector<HTMLElement>('[data-mobile-drawer="true"]');
    drawer?.querySelector<HTMLElement>('button, a[href]')?.focus();
    return () => {
      document.removeEventListener('keydown', closeOnKey);
      (previousFocus && document.contains(previousFocus) ? previousFocus : menuButtonRef.current)?.focus();
    };
  }, [open]);

  if (!showShell) return <>{children}</>;
  if (loading) return <LoadingShell />;
  if (!user) return null;

  return (
    <div className="min-h-dvh">
      {/* Skip-link: sidebar punya 5+ link sebelum konten; keyboard user butuh lompatan ini. */}
      <a
        href="#konten"
        className="sr-only focus:not-sr-only focus:absolute focus:left-3 focus:top-3 focus:z-50 focus:rounded-lg focus:bg-surface focus:px-4 focus:py-2 focus:text-sm focus:text-cyan-300 focus:ring-2 focus:ring-cyan-400/60"
      >
        {t('sb.skip')}
      </a>
      {/* Header desktop permanen (brief §1A-4): kiri = collapse + logo, kanan = notif + cart + lang + avatar. */}
      {/* left di-offset selebar sidebar: sidebar z-50 menutupi header z-30 kalau keduanya mulai di x=0,
          jadi tombol collapse + logo header tidak bisa diklik. Sama seperti main. */}
      <header className={`hidden md:flex fixed top-0 right-0 h-14 z-30 items-center gap-2 px-4 bg-bg/80 backdrop-blur-md border-b border-border transition-[left] duration-200 ${collapsed ? 'md:left-16' : 'md:left-64'}`}>
        <button
          onClick={toggleCollapse}
          aria-label={collapsed ? t('sb.expand') : t('sb.collapse')}
          aria-pressed={collapsed}
          className="h-11 w-11 flex items-center justify-center text-mute hover:text-text hover:bg-surface-hi rounded-lg transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60"
        >
          <PanelLeft className="w-5 h-5" aria-hidden="true" />
        </button>
        {collapsed && (
          <Link href={adminMode ? '/admin' : '/dashboard'} className="font-semibold tracking-tight">
            {t('sb.user')}
          </Link>
        )}
        {adminMode && (
          <span className="ml-1 text-[10px] font-semibold uppercase tracking-wider px-2 py-0.5 rounded-full bg-amber-500/15 text-amber-300 ring-1 ring-inset ring-amber-500/25">{t('ad.common.admin')}</span>
        )}

        <div className="ml-auto flex items-center gap-3">
          <NotificationBell />
          <CartMenu />
          <span className="h-6 w-px bg-border" aria-hidden="true" />
          <LanguageMenu tone="app" />

          {/* Avatar dropdown (Fanny #4): Saldo, Paket, Pengaturan, Keluar. Tanpa search global
              sampai endpoint-nya ada — jangan tampilkan dummy disabled. */}
          <details ref={avatarRef} className="relative">
            <summary className="h-9 w-9 rounded-full bg-cyan-500/15 border border-cyan-400/30 flex items-center justify-center text-sm font-medium text-cyan-300 cursor-pointer list-none [&::-webkit-details-marker]:hidden focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60"
              aria-label={t('sb.account')}>
              {user.email?.[0]?.toUpperCase()}
            </summary>
            <div className="absolute right-0 top-11 w-56 card p-2 shadow-xl">
              <div className="px-3 py-2 mb-1 border-b border-border">
                <span className="text-sm text-text block truncate">{user.email}</span>
                <span className="text-xs text-mute tabular-nums">{t('sb.walletAmt', { amount: rupiah(user.balance) })}</span>
              </div>
              {[
                { href: '/wallet', key: 'sb.wallet' as const, icon: Wallet },
                { href: '/pricing', key: 'sb.pricing' as const, icon: Tag },
                { href: '/pengaturan', key: 'sb.settings' as const, icon: Settings },
              ].map((it) => (
                <Link key={it.href} href={it.href} onClick={() => avatarRef.current?.removeAttribute('open')}
                  className="flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-mute hover:bg-surface-hi hover:text-text">
                  <it.icon className="w-4 h-4" aria-hidden="true" /> {t(it.key)}
                </Link>
              ))}
              <button onClick={() => { avatarRef.current?.removeAttribute('open'); logout(); }}
                className="flex items-center gap-3 w-full px-3 py-2 rounded-lg text-sm text-mute hover:text-danger hover:bg-surface-hi">
                <LogOut className="w-4 h-4" aria-hidden="true" /> {t('sb.logout')}
              </button>
            </div>
          </details>
        </div>
      </header>

      {/* Topbar mobile */}
      <header className="md:hidden fixed top-0 inset-x-0 h-14 z-30 flex items-center gap-2 px-3 bg-bg/80 backdrop-blur-md border-b border-border">
        <button
          ref={menuButtonRef}
          onClick={() => setOpen(true)}
          aria-label={t('sb.openMenu')}
          aria-expanded={open}
          className="p-2 -ml-1 text-text hover:text-cyan-300 rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60"
        >
          <Menu className="w-5 h-5" />
        </button>
        <Link href={adminMode ? '/admin' : '/dashboard'} className="flex items-center gap-2 font-semibold tracking-tight">
          <img src="/node2mint-icon.ico" alt="" aria-hidden="true" className="w-6 h-6 rounded-md object-contain" />
          {t('sb.user')}{adminMode && <span className="ml-2 text-[10px] font-semibold uppercase tracking-wider px-2 py-0.5 rounded-full bg-amber-500/15 text-amber-300 ring-1 ring-inset ring-amber-500/25 align-middle">{t('ad.common.admin')}</span>}
        </Link>
        <div className="ml-auto flex items-center gap-2">
          <NotificationBell />
          <CartMenu />
          <LanguageMenu tone="app" />
        </div>
      </header>

      {adminMode && isAdmin
        ? <AdminSidebar collapsed={collapsed} mobileOpen={open} onClose={() => setOpen(false)} />
        : <Sidebar collapsed={collapsed} mobileOpen={open} onClose={() => setOpen(false)} />}

      {/* key=pathname: transisi masuk ulang tiap pindah halaman, tanpa library. */}
      <main id="konten" key={pathname} className={`page-in pt-14 md:pt-14 min-h-dvh transition-[margin] duration-200 ${collapsed ? 'md:ml-16' : 'md:ml-64'}`}>
        {children}
      </main>
    </div>
  );
}

// Skeleton shell: muncul sesaat sambil token/API diverifikasi.
function LoadingShell() {
  return (
    <div className="min-h-dvh md:ml-64 pt-14 md:pt-14 p-6 md:p-10">
      <div className="max-w-5xl mx-auto space-y-6">
        <div className="skel h-4 w-24" />
        <div className="skel h-8 w-64" />
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 pt-2">
          {[0, 1, 2].map((i) => <div key={i} className="skel h-28" />)}
        </div>
      </div>
    </div>
  );
}