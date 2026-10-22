'use client';

import Link from 'next/link';
import { Activity, ArrowRight, Braces, Package, ShoppingBag } from 'lucide-react';
import { useFetch } from '@/lib/useApi';
import { asList, money, productUrl } from '@/lib/utils';
import { useT, useUrlPrefix } from '@/lib/i18n';
import { prettyName } from '@/components/landing/ModelCatalog';

type Model = { id: string; name: string; type?: string; context_length?: number };
type Health = { name?: string; status?: string };
type Product = { id: string; title: string; price: number; image_url?: string; store_name?: string };

export default function HomeHero({ signedIn = false, gatewayLive = false, initialModels = null, initialProducts = null }: {
  signedIn?: boolean; gatewayLive?: boolean; initialModels?: unknown; initialProducts?: unknown;
}) {
  const { t, lang } = useT();
  const prefix = useUrlPrefix();
  const modelsReq = useFetch<unknown>('/models', false, 60_000, initialModels);
  const productsReq = useFetch<unknown>('/market/featured?per_page=4', false, 60_000, initialProducts);
  const healthReq = useFetch<unknown>('/status', false, 600_000);
  const semuaModel = asList<Model>(modelsReq.data, 'models');
  const liveProducts = asList<Product>(productsReq.data, 'products');
  const products = (liveProducts.length ? liveProducts : asList<Product>(initialProducts, 'products')).slice(0, 2);
  const health = asList<Health>(healthReq.data, 'models');
  const models = semuaModel.slice(0, 2);
  const format = (value?: number) => Number(value ?? 0).toLocaleString(lang === 'id' ? 'id-ID' : 'en-US');

  return (
    <section className="relative grid w-full min-w-0 max-w-full min-h-[min(720px,100dvh)] items-center gap-6 overflow-hidden pb-8 pt-24 lg:grid-cols-[.9fr_1.1fr] lg:gap-10 lg:pb-12 lg:pt-24">
      <div className="pointer-events-none absolute inset-0 -z-10 bg-grid opacity-20 [mask-image:radial-gradient(ellipse_70%_70%_at_65%_35%,black,transparent)]" />
      <div className="relative z-10">
        <p className="anim-fade-up font-mono text-[11px] tracking-[.12em] text-cyan-200 sm:text-xs">{t('hero.kicker')}</p>
        <h1 className="mt-4 max-w-3xl break-words text-[2rem] font-medium leading-[.96] tracking-[-.05em] sm:text-6xl lg:text-[4.25rem]">
          <span className="metal-text anim-fade-up block" style={{ animationDelay: '.04s' }}>{t('hero.h1')}</span>
        </h1>
        <p className="anim-fade-up mt-5 max-w-xl text-sm leading-relaxed text-mute sm:text-base" style={{ animationDelay: '.16s' }}>{t('hero.sub')}</p>
        <div className="anim-fade-up mt-6 grid grid-cols-2 gap-2 sm:flex sm:gap-3" style={{ animationDelay: '.2s' }}>
          <Link href={`${prefix}/market`} className="shimmer inline-flex min-h-11 min-w-[44px] items-center justify-center gap-2 rounded-xl bg-cyan-300 px-2 py-3 text-xs font-semibold text-[#071015] active:scale-[.98] sm:px-5 sm:text-sm">
            <ShoppingBag className="h-4 w-4" aria-hidden="true" /> {t('hero.marketCta')}
          </Link>
          <Link href={gatewayLive ? (signedIn ? '/dashboard' : '/masuk?mode=daftar') : '#waitlist'} className="inline-flex min-h-11 min-w-[44px] items-center justify-center gap-2 rounded-xl border border-white/15 px-2 py-3 text-xs font-medium transition-colors hover:border-cyan-300/40 hover:text-cyan-200 active:scale-[.98] sm:px-5 sm:text-sm">
            {gatewayLive ? t('hero.apiCta') : 'API gateway segera hadir'} <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Link>
        </div>
      </div>

      <div className="anim-fade-up relative mx-auto w-full min-w-0 max-w-full" style={{ animationDelay: '.18s' }}>
        <div className="absolute -inset-8 -z-10 rounded-full bg-cyan-400/[.07] blur-3xl" aria-hidden="true" />
        <div className="spotlight overflow-hidden rounded-[24px] border border-white/15 bg-[#0d0f16]/95 shadow-[0_28px_80px_rgba(0,0,0,.35)]">
          <div className="grid grid-cols-2 divide-x divide-white/10">
            <section aria-labelledby="hero-market-title" className="min-w-0 overflow-hidden p-3 sm:p-4">
              <div className="mb-3 flex items-center justify-between gap-2">
                <div className="min-w-0"><p id="hero-market-title" className="truncate text-xs font-semibold sm:text-sm">{t('hero.marketCard')}</p><p className="mt-1 truncate text-[10px] text-mute sm:text-xs">{t('show.featured')}</p></div>
                <ShoppingBag className="h-4 w-4 shrink-0 text-cyan-300" aria-hidden="true" />
              </div>
              <div className="space-y-2">
                {productsReq.loading && products.length === 0 ? [0, 1].map((i) => <div key={i} className="skel h-20 w-full" />) : products.map((product) => (
                  <Link key={product.id} href={`${prefix}${productUrl(product.store_name, product.title, product.id)}`} className="group grid grid-cols-[48px_1fr] gap-2 rounded-xl border border-white/[.07] bg-white/[.025] p-2 transition-colors hover:bg-white/[.05] sm:grid-cols-[56px_1fr]">
                    <div className="aspect-square overflow-hidden rounded-lg bg-cyan-500/10">{product.image_url ? <img src={product.image_url} alt={product.title} className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105" /> : <span className="grid h-full place-items-center"><Package className="h-5 w-5 text-mute" aria-hidden="true" /></span>}</div>
                    <div className="min-w-0 self-center"><p className="line-clamp-2 text-[11px] font-medium leading-tight sm:text-xs">{product.title}</p><p className="mt-1 truncate font-mono text-[10px] font-semibold text-cyan-200 sm:text-xs">{money(product.price, lang)}</p></div>
                  </Link>
                ))}
                {!productsReq.loading && products.length === 0 && <div className="grid h-28 place-items-center rounded-xl border border-white/[.07] px-2 text-center text-[10px] text-mute">{t('models.unknown')}</div>}
              </div>
              <Link href={`${prefix}/market`} className="mt-3 flex min-h-11 items-center justify-center gap-1 rounded-lg border border-white/10 text-[11px] font-medium hover:border-cyan-300/40 hover:text-cyan-200 sm:text-xs">{t('show.all')} <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" /></Link>
            </section>

            <section aria-labelledby="hero-api-title" className="min-w-0 overflow-hidden p-3 sm:p-4">
              <div className="mb-3 flex items-center justify-between gap-2">
                <div className="min-w-0"><p id="hero-api-title" className="truncate text-xs font-semibold sm:text-sm">{t('hero.apiCard')}</p><p className="mt-1 truncate text-[10px] text-mute sm:text-xs">{t('models.available', { n: format(semuaModel.length) })}</p></div>
                <span className="inline-flex items-center rounded-md border border-emerald-300/20 bg-emerald-400/10 p-1.5 text-emerald-200" title={t('hero.live')}><Activity className="h-3.5 w-3.5" aria-hidden="true" /></span>
              </div>
              <ul className="space-y-2">
                {modelsReq.loading && models.length === 0 ? [0, 1].map((i) => <li key={i} className="skel h-20 w-full" />) : models.map((model) => {
                  const state = health.find((item) => item.name === model.name);
                  const ok = ['ok', 'up', 'healthy', 'operational'].includes((state?.status ?? '').toLowerCase());
                  return <li key={model.id} className="flex h-20 flex-col justify-center rounded-xl border border-white/[.07] bg-white/[.025] px-3 transition-colors hover:bg-white/[.05]"><p className="truncate font-mono text-[11px] text-text sm:text-xs">{prettyName(model.name)}</p><p className="mt-1 truncate text-[10px] text-mute sm:text-[11px]">{model.type ?? 'TEXT'}{model.context_length ? ` · ${format(model.context_length)} ctx` : ''}</p><p className={`mt-1 text-[10px] ${ok ? 'text-emerald-300' : 'text-mute'}`}>{ok ? t('models.healthy') : t('models.unknown')}</p></li>;
                })}
                {!modelsReq.loading && models.length === 0 && <li className="grid h-28 place-items-center rounded-xl border border-white/[.07] px-2 text-center text-[10px] text-mute">{t('models.unknown')}</li>}
              </ul>
              <a href="#models" className="mt-3 flex min-h-11 items-center justify-center gap-1 rounded-lg border border-white/10 text-[11px] font-medium hover:border-cyan-300/40 hover:text-cyan-200 sm:text-xs">{t('hero.explore')} <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" /></a>
            </section>
          </div>
          <div className="flex items-center gap-3 border-t border-white/10 bg-black/15 px-4 py-3"><Braces className="h-4 w-4 shrink-0 text-cyan-300" aria-hidden="true" /><code className="min-w-0 flex-1 truncate text-[10px] text-mute sm:text-xs">{t('hero.walletCode')}</code><span className="hidden text-xs text-cyan-200 sm:block">{t('hero.statBase')}</span></div>
        </div>
      </div>
    </section>
  );
}