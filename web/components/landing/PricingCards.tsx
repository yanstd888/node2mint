'use client';

import Link from 'next/link';
import { Check } from 'lucide-react';
import { angka, money, paymentUrl } from '@/lib/utils';
import { useT, useUrlPrefix } from '@/lib/i18n';

type Package = { id: string; name: string; tokens: number; bonus_tokens?: number; price: number; days: number; effective_days?: number | null; is_signup?: boolean };
type Billing = { free_daily_tokens?: number; cached_token_discount?: number; usage_surcharge_rate?: number };

const packagesFrom = (data: unknown) => {
  const raw = (data as { packages?: unknown } | null)?.packages;
  return Array.isArray(raw) ? raw.filter((item): item is Package => {
    const p = item as Partial<Package>;
    return typeof p.id === 'string' && typeof p.name === 'string' && Number.isFinite(p.tokens) && Number.isFinite(p.price) && Number(p.price) >= 0 && Number.isFinite(p.days);
  }) : [];
};

export default function PricingCards({ data, compact = false }: { data: unknown | null; compact?: boolean }) {
  const { t } = useT();
  const prefix = useUrlPrefix();
  const packages = packagesFrom(data).sort((a, b) => a.price - b.price);
  const visiblePackages = compact
    ? packages.filter((p) => {
      const key = p.name.trim().toLowerCase().replace(/[^a-z]/g, '');
      return p.is_signup || key === 'start' || key === 'starter' || key === 'freejustsignup' || key === 'mini' || key === 'pro';
    }).slice(0, 3)
    : packages;
  return <section id="packages" aria-labelledby="packages-title" className={compact ? 'py-16 sm:py-20 scroll-mt-16' : 'mb-16'}>
    <div className="mb-8">
      <p className="label mb-1">{t('price.kicker')}</p>
      <h2 id="packages-title" className="text-2xl font-semibold tracking-tight sm:text-3xl">{t('pricingPage.packagesH')}</h2>
      <p className="mt-2 max-w-2xl text-sm text-mute">{t('pricingPage.packagesP')}</p>
    </div>
    {visiblePackages.length === 0 ? <p className="rounded-xl border border-white/10 p-5 text-sm text-mute">{t('pricingPage.packagesUnavailable')}</p> : <ul className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
      {visiblePackages.map((p) => <li key={p.id} className="item-card flex min-h-72 flex-col p-6">
        <h3 className="text-lg font-semibold">{p.name}</h3>
        <p className="mt-3 text-3xl font-semibold tabular-nums">{money(p.price)}</p>
        <ul className="mt-4 flex-1 space-y-2 text-sm text-mute">
          <li key="tokens" className="flex gap-2"><Check className="mt-0.5 h-4 w-4 shrink-0 text-cyan-300" aria-hidden="true" /><span>{p.bonus_tokens ? t('price.tokensBonus', { base: angka(p.tokens - p.bonus_tokens), bonus: angka(p.bonus_tokens), total: angka(p.tokens) }) : t('price.tokens', { n: angka(p.tokens) })}</span></li>
          <li key="days" className="flex gap-2"><Check className="mt-0.5 h-4 w-4 shrink-0 text-cyan-300" aria-hidden="true" /><span>{t('price.days', { n: p.effective_days ?? p.days })}</span></li>
          <li key="models" className="flex gap-2"><Check className="mt-0.5 h-4 w-4 shrink-0 text-cyan-300" aria-hidden="true" /><span>{t('price.allModels')}</span></li>
        </ul>
        {p.is_signup ? <span className="mt-6 inline-flex min-h-11 items-center justify-center rounded-lg border border-emerald-300/30 bg-emerald-400/10 px-4 py-3 text-sm font-semibold text-emerald-100">{t('pricingPage.signupOneTime')}</span> : <Link href={compact ? `${prefix}/pricing` : paymentUrl('package', p.price, p.name, { id: p.id })} className="mt-6 inline-flex min-h-11 items-center justify-center rounded-lg border border-cyan-300/40 px-4 py-3 text-sm font-semibold text-cyan-100 transition-colors hover:bg-cyan-300 hover:text-slate-950">{compact ? t('pricingPage.details') : t('price.buy')}</Link>}
      </li>)}
    </ul>}
  </section>;
}

export function BillingExplanation({ models }: { models: unknown | null }) {
  const { t } = useT();
  const billing = (models as { billing?: Billing } | null)?.billing;
  const free = Number(billing?.free_daily_tokens);
  const cached = Number(billing?.cached_token_discount);
  const surcharge = Number(billing?.usage_surcharge_rate);
  const hasFree = Number.isFinite(free) && free > 0;
  const hasCached = Number.isFinite(cached) && cached >= 0 && cached <= 1;
  const hasSurcharge = Number.isFinite(surcharge) && surcharge >= 0;
  return <section aria-labelledby="billing-title" className="mb-16 scroll-mt-24">
    <p className="label mb-2">{t('pricingPage.billingKicker')}</p>
    <h2 id="billing-title" className="text-2xl font-semibold tracking-tight sm:text-3xl">{t('pricingPage.billingH')}</h2>
    <div className="mt-6 grid gap-4 md:grid-cols-2">
      <div className="rounded-xl border border-white/10 bg-white/[.025] p-5 text-sm leading-relaxed text-mute"><h3 className="font-medium text-text">{t('pricingPage.freePaidH')}</h3><p className="mt-2">{t('pricingPage.freePaidP', { free: hasFree ? t('pricingPage.freeAllowance', { n: angka(free) }) : '' })}</p></div>
      <div className="rounded-xl border border-white/10 bg-white/[.025] p-5 text-sm leading-relaxed text-mute"><h3 className="font-medium text-text">{t('pricingPage.multiplierH')}</h3><p className="mt-2">{t('pricingPage.multiplierP')}</p></div>
    </div>
    <div className="mt-4 rounded-xl border border-cyan-300/25 bg-cyan-300/[.05] p-5 text-sm leading-relaxed text-cyan-50"><p><strong>{t('pricingPage.billable', { cached: hasCached ? ` × ${cached}` : '', surcharge: hasSurcharge ? ` + ${surcharge}` : '' })}</strong></p><p className="mt-2 text-cyan-100/80">{t('pricingPage.billableP')}</p></div>
    <p className="mt-4 text-xs leading-relaxed text-mute">{t('pricingPage.currencyP')}</p>
  </section>;
}