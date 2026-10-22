'use client';



import Link from 'next/link';

import { Activity, ArrowRight, Braces, CircleDollarSign } from 'lucide-react';

import { useFetch } from '@/lib/useApi';

import { asList } from '@/lib/utils';

import { useT } from '@/lib/i18n';

import { prettyName } from '@/components/landing/ModelCatalog';



type Model = { id: string; name: string; type?: string; context_length?: number };

type Health = { name?: string; status?: string };



export default function HomeHero({ signedIn = false }: { signedIn?: boolean }) {

  const { t, lang } = useT();

  const modelsReq = useFetch<unknown>('/models', false, 60_000);

  const healthReq = useFetch<unknown>('/status', false, 600_000);

  // Hero = panel sampel, bukan katalog: tampil 4 baris lalu tombol "lihat semua"

  // yang melompat ke #models. Ambangnya 4 (bukan 8) supaya panel hero tidak

  // pernah memanjang walau katalog 30+.

  const semua = asList<Model>(modelsReq.data, 'models');

  const health = asList<Health>(healthReq.data, 'models');

  const terbuka = semua.length <= 4;

  const models = terbuka ? semua : semua.slice(0, 4);

  const format = (value?: number) => Number(value ?? 0).toLocaleString(lang === 'id' ? 'id-ID' : 'en-US');



  return (

    <section className="relative grid min-h-[min(760px,100dvh)] items-center gap-12 overflow-hidden pb-16 pt-28 lg:grid-cols-[1.02fr_.98fr] lg:pb-20">

      <div className="pointer-events-none absolute inset-0 -z-10 bg-grid opacity-20 [mask-image:radial-gradient(ellipse_70%_70%_at_65%_35%,black,transparent)]" />

      <div className="relative z-10">

        <p className="anim-fade-up font-mono text-xs tracking-[.12em] text-cyan-200">{t('hero.kicker')}</p>

        <h1 className="mt-5 max-w-3xl text-5xl font-medium leading-[.98] tracking-[-.055em] sm:text-6xl lg:text-7xl">

          <span className="metal-text anim-fade-up block" style={{ animationDelay: '.04s' }}>{t('hero.h1')}</span>

        </h1>

        <p className="anim-fade-up mt-6 max-w-xl text-base leading-relaxed text-mute sm:text-lg" style={{ animationDelay: '.16s' }}>{t('hero.sub')}</p>

        <div className="anim-fade-up mt-8 flex flex-col gap-3 sm:flex-row" style={{ animationDelay: '.2s' }}>

          <Link href={signedIn ? '/dashboard' : '/masuk?mode=daftar'} className="shimmer inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-cyan-300 px-5 py-3 text-sm font-semibold text-[#071015] active:scale-[.98]">

            <CircleDollarSign className="h-4 w-4" aria-hidden="true" />

            {signedIn ? t('hero.ctaIn') : t('hero.cta')}

          </Link>

          <a href="#models" className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl border border-white/15 px-5 py-3 text-sm font-medium transition-colors hover:border-cyan-300/40 hover:text-cyan-200 active:scale-[.98]">

            {t('hero.explore')} <ArrowRight className="h-4 w-4" aria-hidden="true" />

          </a>

        </div>

      </div>



      <div className="anim-fade-up relative mx-auto w-full max-w-xl" style={{ animationDelay: '.18s' }}>

        <div className="absolute -inset-8 -z-10 rounded-full bg-cyan-400/[.07] blur-3xl" aria-hidden="true" />

        <div className="spotlight overflow-hidden rounded-[24px] border border-white/15 bg-[#0d0f16]/95 shadow-[0_28px_80px_rgba(0,0,0,.35)]">

          <div className="flex items-center justify-between border-b border-white/10 px-5 py-4">

            <div>

              <p className="text-sm font-medium">{t('hero.card')}</p>

              <p className="mt-1 text-xs text-mute">{semua.length ? t('models.available', { n: format(semua.length) }) : t('models.unknown')}</p>

            </div>

            <span className="inline-flex items-center gap-2 rounded-lg border border-emerald-300/20 bg-emerald-400/10 px-2.5 py-1.5 text-xs text-emerald-200">

              <Activity className="h-3.5 w-3.5" aria-hidden="true" /> {t('hero.live')}

            </span>

          </div>

          <div className="min-h-[260px] p-3 sm:p-4">

            {modelsReq.loading ? (

              <div className="space-y-2" aria-busy="true">{[0, 1, 2, 3].map((i) => <div key={i} className="skel h-14 w-full" />)}</div>

            ) : modelsReq.err ? (

              <p role="status" className="rounded-xl border border-white/10 p-4 text-sm text-mute">{t('models.unknown')}</p>

            ) : (

              <ul className="space-y-2">

                {models.map((model) => {

                  const state = health.find((item) => item.name === model.name);

                  const ok = ['ok', 'up', 'healthy', 'operational'].includes((state?.status ?? '').toLowerCase());

                  return (

                    <li key={model.id} className="grid grid-cols-[1fr_auto] items-center gap-4 rounded-xl border border-white/[.07] bg-white/[.025] px-4 py-3 transition-colors hover:bg-white/[.05]">

                      <div className="min-w-0">

                        <p className="truncate font-mono text-sm text-text">{prettyName(model.name)}</p>

                        <p className="mt-1 text-xs text-mute">{model.type ?? 'TEXT'}{model.context_length ? ` · ${format(model.context_length)} ctx` : ''}</p>

                      </div>

                      <span className={`text-xs ${ok ? 'text-emerald-300' : 'text-mute'}`}>{ok ? t('models.healthy') : t('models.unknown')}</span>

                    </li>

                  );

                })}

              </ul>

            )}

            {/* Katalog panjang (30+): sisanya dibuka tombol, bukan memanjangkan hero. */}

            {!terbuka && (

              <a href="#models" className="mt-3 flex min-h-11 items-center justify-center rounded-xl border border-white/15 text-sm font-medium transition-colors hover:border-cyan-300/40 hover:text-cyan-200">

                {t('hero.seeAll', { n: format(semua.length) })}

              </a>

            )}

          </div>

          <div className="flex items-center gap-3 border-t border-white/10 bg-black/15 px-5 py-4">

            <Braces className="h-4 w-4 shrink-0 text-cyan-300" aria-hidden="true" />

            <code className="min-w-0 flex-1 truncate text-xs text-mute">/v1/chat/completions</code>

            <span className="text-xs text-cyan-200">{t('hero.statBase')}</span>

          </div>

        </div>

      </div>

    </section>

  );

}

