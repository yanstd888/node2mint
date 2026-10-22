'use client';
// Admin Dashboard: ringkasan metrik utama (brief §1B "Overview") + analytics grafik.
// Dulu menempel di dashboard user; sekarang jadi halaman pertama Mode Admin.
import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import { useT } from '@/lib/i18n';
import { rupiah } from '@/lib/utils';
import AnalyticsSection from '@/components/AnalyticsSection';

type AdminSummary = { users: number; orders: number; produk: number; revenue: number };

export default function AdminDashboardPage() {
  const { user } = useAuth();
  const { t } = useT();
  const [admin, setAdmin] = useState<AdminSummary | null>(null);
  const [err, setErr] = useState(false);
  const isAdmin = user?.role === 'ADMIN' || user?.role === 'SUPER_ADMIN';

  useEffect(() => {
    if (!user || !isAdmin) return;
    let alive = true;
    api.get<AdminSummary>('/admin/summary')
      .then(({ data }) => { if (alive) setAdmin(data); })
      .catch(() => { if (alive) setErr(true); });
    return () => { alive = false; };
  }, [user, isAdmin]);

  if (!user || !isAdmin) return null;

  return (
    <div className="p-6 md:p-10 max-w-6xl mx-auto">
      <header className="mb-8">
        <p className="label mb-1">{t('adm.mode')}</p>
        <h1 className="text-2xl md:text-3xl font-semibold tracking-tight">{t('adm.h')}</h1>
      </header>

      <section aria-label={t('adm.overview')} className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4">
        {err ? (
          <p role="alert" className="card p-5 text-sm text-danger sm:col-span-2 xl:col-span-4">{t('adm.loadErr')}</p>
        ) : !admin ? (
          [0, 1, 2, 3].map((i) => (
            <div key={i} className="card p-5 h-24">
              <div className="skel h-4 w-16 mb-3" />
              <div className="skel h-6 w-20" />
            </div>
          ))
        ) : (
          <>
            <Stat title={t('adm.users')} value={String(admin.users ?? 0)} />
            <Stat title={t('adm.orders')} value={String(admin.orders ?? 0)} />
            <Stat title={t('adm.products')} value={String(admin.produk ?? 0)} />
            <Stat title={t('adm.revenue')} value={rupiah(admin.revenue)} highlight />
          </>
        )}</section>

      <AnalyticsSection />
    </div>
  );
}

// Kartu stat seragam: aksen tunggal (cyan) cuma di nilai yang penting.
function Stat({ title, value, highlight }: { title: string; value: string; highlight?: boolean }) {
  return (
    <div className="card lift p-6">
      <span className="label">{title}</span>
      <span className={`block mt-2 text-2xl font-semibold tabular-nums tracking-tight ${highlight ? 'text-cyan-300' : 'text-text'}`}>
        {value}
      </span>
    </div>
  );
}