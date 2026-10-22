'use client';

import { BookOpen, Gift, Key, LayoutDashboard, Link2, MessageSquare, Package, Store, Tag, Wallet } from 'lucide-react';
import SidebarFrame, { type SidebarFrameProps, type SidebarGroup, type SidebarItem } from '@/components/SidebarFrame';

type Props = Omit<SidebarFrameProps, 'mode' | 'brand' | 'top' | 'groups' | 'bottom'>;

const TOP: SidebarItem[] = [
  { href: '/dashboard', label: 'sb.dashboard', icon: LayoutDashboard },
  { href: '/pricing', label: 'sb.pricing', icon: Tag },
  { href: '/market', label: 'sb.market', icon: Store },
  { href: '/chat', label: 'sb.playground', icon: MessageSquare },
];

const GROUPS: SidebarGroup[] = [
  { nama: 'sb.gTransaksi', items: [
    { href: '/pesanan', label: 'sb.orders', icon: Package },
    { href: '/library', label: 'sb.library', icon: BookOpen },
    { href: '/favorit', label: 'sb.wishlist', icon: Gift },
    { href: '/wallet', label: 'sb.wallet', icon: Wallet },
  ] },
  { nama: 'sb.gKemitraan', items: [
    { href: '/toko', label: 'sb.store', icon: Store },
    { href: '/afiliasi', label: 'sb.affiliate', icon: Link2 },
    { href: '/referral', label: 'sb.referral', icon: Gift },
    { href: '/api-keys', label: 'sb.apikeys', icon: Key },
  ] },
];

const BOTTOM: SidebarItem[] = [
  { href: '/bantuan', label: 'sb.help', icon: BookOpen },
  { href: '/pengaturan', label: 'sb.settings', icon: Tag },
];

export default function Sidebar(props: Props) {
  return <SidebarFrame {...props} mode="user" brand="sb.user" top={TOP} groups={GROUPS} bottom={BOTTOM} />;
}