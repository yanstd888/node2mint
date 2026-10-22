'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { ChevronRight, LogOut, ShieldCheck, X, type LucideIcon } from 'lucide-react';
import { useAuth } from '@/lib/auth';
import { stripLocale, useT, type Key as TKey } from '@/lib/i18n';
import { money } from '@/lib/utils';

export type SidebarItem = { href: string; label: TKey; icon: LucideIcon };
export type SidebarGroup = { nama: TKey; items: SidebarItem[] };

export type SidebarFrameProps = {
  mode: 'user' | 'admin';
  brand: TKey;
  top: SidebarItem[];
  groups: SidebarGroup[];
  bottom: SidebarItem[];
  collapsed?: boolean;
  mobileOpen?: boolean;
  onClose?: () => void;
};

export default function SidebarFrame({
  mode,
  brand,
  top,
  groups,
  bottom,
  collapsed = false,
  mobileOpen = false,
  onClose,
}: SidebarFrameProps) {
  const { user, logout } = useAuth();
  const { t, lang } = useT();
  const pathname = stripLocale(usePathname());
  const compact = collapsed && !mobileOpen;
  const admin = mode === 'admin';
  const accent = admin
    ? { bg: 'bg-amber-500/15 border-amber-400/30 text-amber-300', active: 'bg-amber-500/10 text-amber-300', bar: 'bg-amber-400' }
    : { bg: 'bg-cyan-500/15 border-cyan-400/30 text-cyan-300', active: 'bg-cyan-500/10 text-cyan-300', bar: 'bg-cyan-400' };

  useEffect(() => { onClose?.(); }, [pathname]); // eslint-disable-line react-hooks/exhaustive-deps

  const [open, setOpen] = useState<string[]>(() => groups.map((group) => group.nama as string));
  useEffect(() => { setOpen(groups.map((group) => group.nama as string)); }, [mode]); // eslint-disable-line react-hooks/exhaustive-deps
  const toggle = (name: string) => setOpen((current) => current.includes(name) ? current.filter((value) => value !== name) : [...current, name]);

  const Row = ({ item, tooltip }: { item: SidebarItem; tooltip?: boolean }) => {
    const active = pathname === item.href || pathname.startsWith(item.href + '/');
    return (
      <Link href={item.href} aria-current={active ? 'page' : undefined}
        title={tooltip ? t(item.label) : undefined} aria-label={tooltip ? t(item.label) : undefined}
        className={`relative flex items-center gap-3 rounded-lg text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60 ${compact ? 'justify-center h-11 w-11 mx-auto' : 'px-3 py-3'} ${active ? accent.active : 'text-mute hover:bg-surface-hi hover:text-text'}`}>
        {active && !compact && <span aria-hidden="true" className={`absolute left-0 top-1/2 -translate-y-1/2 h-4 w-0.5 rounded-full ${accent.bar}`} />}
        <item.icon className="w-4 h-4 shrink-0" aria-hidden="true" />
        {!compact && t(item.label)}
      </Link>
    );
  };

  return (
    <>
      {mobileOpen && <div className="fixed inset-0 bg-black/60 z-50 md:hidden" onClick={onClose} aria-hidden="true" />}
      <aside aria-label={t('sb.nav')} role={mobileOpen ? 'dialog' : undefined} aria-modal={mobileOpen ? true : undefined}
        data-mobile-drawer={mobileOpen ? 'true' : undefined}
        className={`fixed left-0 top-0 h-dvh z-[60] bg-bg/85 backdrop-blur-md border-r border-border flex flex-col overflow-x-hidden transition-[width,transform] duration-200 md:translate-x-0 ${compact ? 'md:w-16' : 'md:w-64'} ${mobileOpen ? 'translate-x-0 w-64' : '-translate-x-full max-md:invisible'}`}>
        <div className={`h-14 flex items-center border-b border-border shrink-0 ${compact ? 'md:justify-center justify-between px-4' : 'justify-between px-4'}`}>
          <Link href={admin ? '/admin' : '/dashboard'} className="flex items-center gap-3 font-semibold tracking-tight" title={compact ? t(brand) : undefined}>
            <img src="/node2mint-icon.ico" alt="" aria-hidden="true" className="w-7 h-7 rounded-lg object-contain" />
            {!compact && t(brand)}
          </Link>
          <button onClick={onClose} aria-label={t('sb.close')} className="md:hidden p-2 text-mute hover:text-text rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60">
            <X className="w-5 h-5" aria-hidden="true" />
          </button>
        </div>

        {/* Scroll tetap ada untuk keyboard/mousewheel, tetapi scrollbar native tidak boleh
            mengambil lebar sidebar 64px atau membuat scrollbar horizontal saat ringkas. */}
        <nav className={`sidebar-nav flex-1 min-h-0 space-y-4 overflow-y-auto overflow-x-hidden p-3 pb-6 ${compact ? 'px-2' : ''}`}>
          <div className="space-y-1">{top.map((item) => <Row key={item.href} item={item} tooltip={compact} />)}</div>

          {compact ? (
            <div className="space-y-1 pt-2 border-t border-border">{groups.flatMap((group) => group.items).map((item) => <Row key={item.href} item={item} tooltip />)}</div>
          ) : (
            groups.map((group) => {
              const expanded = open.includes(group.nama);
              return (
                <div key={group.nama}>
                  <button type="button" onClick={() => toggle(group.nama)} aria-expanded={expanded} className="w-full flex items-center justify-between px-3 mb-2 text-left rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60">
                    <span className="label">{t(group.nama)}</span>
                    <ChevronRight aria-hidden="true" className={`w-3.5 h-3.5 text-mute transition-transform duration-200 ${expanded ? 'rotate-90' : ''}`} />
                  </button>
                  <div className={`grid transition-[grid-template-rows] duration-200 ${expanded ? '[grid-template-rows:1fr]' : '[grid-template-rows:0fr]'}`}>
                    <div className="overflow-hidden min-h-0"><div className="space-y-1 pb-1">{group.items.map((item) => <Row key={item.href} item={item} />)}</div></div>
                  </div>
                </div>
              );
            })
          )}

          <div className="space-y-1 pt-2 border-t border-border">{bottom.map((item) => <Row key={item.href} item={item} tooltip={compact} />)}</div>
        </nav>

        {user && <div className={`p-4 border-t border-border shrink-0 ${compact ? 'px-2' : ''}`}>
          <div className={`flex items-center gap-3 mb-3 ${compact ? 'justify-center' : ''}`}>
            <div className={`w-8 h-8 shrink-0 rounded-full flex items-center justify-center text-xs font-medium ${accent.bg}`}>
              {admin ? <ShieldCheck className="w-4 h-4" aria-hidden="true" /> : (user.name || user.email)?.[0]?.toUpperCase()}
            </div>
            {!compact && <div className="flex-1 min-w-0"><span className="text-sm text-text block truncate">{user.name || user.email}</span><span className="text-xs text-mute tabular-nums">{t('sb.walletAmt', { amount: money(user.balance, lang) })}</span></div>}
          </div>
          <button onClick={logout} className={`flex items-center gap-2 w-full text-sm text-mute hover:text-danger hover:bg-surface-hi rounded-lg transition-colors ${compact ? 'justify-center h-11' : 'px-3 py-2'}`} aria-label={t('sb.logout')} title={compact ? t('sb.logout') : undefined}>
            <LogOut className="w-4 h-4" aria-hidden="true" /> {!compact && t('sb.logout')}
          </button>
        </div>}
      </aside>
    </>
  );
}
