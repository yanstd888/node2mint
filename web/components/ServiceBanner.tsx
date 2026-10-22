'use client';
// Banner status global: muncul hanya saat ada gangguan. Menempel di kiri-bawah
// (toast ada di kanan-bawah) supaya tidak menabrak header yang position:fixed
// di PublicHeader/AppShell.
import Link from 'next/link';
import { AlertTriangle } from 'lucide-react';
import { useFetch } from '@/lib/useApi';
import { useT, useUrlPrefix } from '@/lib/i18n';

type Status = { overall?: string; availability_24h?: number; maintenance?: boolean | string };

export default function ServiceBanner() {
  const { t } = useT();
  const prefix = useUrlPrefix();
  // Poll 10 menit: status layanan tidak perlu realtime per menit.
  const { data } = useFetch<Status>('/status', false, 600_000);
  const overall = String(data?.overall ?? '').toUpperCase();
  const maint = data?.maintenance === true || String(data?.maintenance ?? '').toLowerCase() === 'true';
  const availability = Number(data?.availability_24h);
  // ponytail: DEGRADED hanya memberi notifikasi <90%; DOWN/OUTAGE tetap selalu terlihat.
  const degraded = overall === 'DEGRADED' && Number.isFinite(availability) && availability < 90;
  const down = ['DOWN', 'OUTAGE'].includes(overall) || degraded;
  const label = maint ? t('st.maint') : degraded ? t('st.degraded') : t('st.outage');
  if (!maint && !down) return null;

  return (
    <aside
      role="status"
      aria-label={t('st.bannerAria')}
      className={`fixed bottom-4 left-4 z-30 flex max-w-[calc(100vw-2rem)] items-center gap-3 rounded-xl border px-4 py-3 text-sm shadow-xl backdrop-blur-md ${
        maint ? 'border-gold/40 bg-gold/15 text-gold' : 'border-danger/40 bg-danger/15 text-danger'
      }`}
    >
      <AlertTriangle className="h-4 w-4 shrink-0" aria-hidden="true" />
      <span className="font-medium">{label}</span>
      <Link href={`${prefix}/status`} className="underline underline-offset-2 hover:opacity-80">{t('st.bannerMore')}</Link>
    </aside>
  );
}
