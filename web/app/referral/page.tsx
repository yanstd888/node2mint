'use client';
// Referral: program undang pendaftaran — reward berupa paket token API.
// Berbeda dari Afiliasi (link produk, reward wallet).
import ReferralSection from '@/components/sections/ReferralSection';
import { useT } from '@/lib/i18n';

export default function ReferralPage() {
  const { t } = useT();
  return (
    <div className="p-6 md:p-10 max-w-5xl mx-auto">
      <header className="mb-8">
        <p className="label mb-1">{t('ref.kicker')}</p>
        <h1 className="text-2xl md:text-3xl font-semibold tracking-tight">{t('sb.referral')}</h1>
      </header>
      <ReferralSection />
    </div>
  );
}
