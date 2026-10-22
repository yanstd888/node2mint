'use client';

import { useEffect } from 'react';
import { Activity, AlertCircle, Bell, Box, Cpu, Gift, HeartPulse, LayoutDashboard, Newspaper, Package, ScrollText, Settings, ShieldAlert, Store, Users, Wallet } from 'lucide-react';
import { en as adminEn, id as adminId } from '@/lib/dict/admin';
import { registerAdminDictionary } from '@/lib/i18n';
import SidebarFrame, { type SidebarFrameProps, type SidebarGroup, type SidebarItem } from '@/components/SidebarFrame';

type Props = Omit<SidebarFrameProps, 'mode' | 'brand' | 'top' | 'groups' | 'bottom'>;

const TOP: SidebarItem[] = [
  { href: '/admin', label: 'sb.adminDash', icon: LayoutDashboard },
];

const GROUPS: SidebarGroup[] = [
  { nama: 'sb.gOps', items: [
    { href: '/admin/orders', label: 'sb.orders', icon: Package },
    { href: '/admin/disputes', label: 'sb.disputes', icon: AlertCircle },
    { href: '/admin/wallet', label: 'sb.wallet', icon: Wallet },
    { href: '/admin/redeem', label: 'sb.redeem', icon: Gift },
    { href: '/admin/packages', label: 'sb.packages', icon: Box },
    { href: '/admin/products', label: 'sb.products', icon: Store },
  ] },
  { nama: 'sb.gAi', items: [
    { href: '/admin/users', label: 'sb.users', icon: Users },
    { href: '/admin/providers', label: 'sb.providers', icon: Store },
    { href: '/admin/models', label: 'sb.models', icon: Cpu },
  ] },
  { nama: 'sb.gInfra', items: [
    { href: '/admin/uptime', label: 'sb.uptime', icon: Activity },
    { href: '/admin/health', label: 'sb.health', icon: HeartPulse },
    { href: '/admin/notifications', label: 'sb.notifications', icon: Bell },
    { href: '/admin/audit', label: 'sb.auditlog', icon: ScrollText },
    { href: '/admin/fraud', label: 'sb.fraud', icon: ShieldAlert },
  ] },
];

const BOTTOM: SidebarItem[] = [
  { href: '/admin/artikel', label: 'sb.articles', icon: Newspaper },
  { href: '/admin/pengaturan', label: 'sb.platform', icon: Settings },
];

export default function AdminSidebar(props: Props) {
  useEffect(() => { registerAdminDictionary({ en: adminEn, id: adminId }); }, []);
  return <SidebarFrame {...props} mode="admin" brand="sb.admin" top={TOP} groups={GROUPS} bottom={BOTTOM} />;
}
