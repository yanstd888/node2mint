'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { ArrowRight, Check, Copy } from 'lucide-react';
import { useFetch } from '@/lib/useApi';
import { asList } from '@/lib/utils';
import { useT, useUrlPrefix } from '@/lib/i18n';
import { useToast } from '@/components/ui/toast';
import { ModelGrid, type Health, type Model } from '@/components/landing/ModelCatalog';

export default function ModelCards({ initialModels = null }: { initialModels?: unknown }) {
  const { t, lang } = useT();
  const prefix = useUrlPrefix();
  const toast = useToast();
  // initialModels = katalog dari HTML server. Katalog polling 60 detik;
  // status kesehatan polling 10 menit.
  const modelsReq = useFetch<unknown>('/models', false, 60_000, initialModels);
  const statusReq = useFetch<unknown>('/status', false, 600_000);
  const [base, setBase] = useState('/v1');
  const [copied, setCopied] = useState(false);

  useEffect(() => setBase(`${window.location.origin}/v1`), []);

  // Seluruh katalog, bukan 6 pertama: kartu yang tidak tampil dibuka tombol
  // "lihat semua" di ModelGrid (satu tempat, bukan batas keras di sini).
  const models = asList<Model>(modelsReq.data, 'models');
  const freeDailyTokens = Number((modelsReq.data as { billing?: { free_daily_tokens?: number } } | null)?.billing?.free_daily_tokens ?? 0);
  const health = asList<Health>(statusReq.data, 'models');
  const jumlah = (n: number) => n.toLocaleString(lang === 'id' ? 'id-ID' : 'en-US');

  return (
    <section id="models" aria-labelledby="models-title" className="scroll-mt-24 py-16 sm:py-24">
      <div className="grid gap-10 lg:grid-cols-[.72fr_1.28fr] lg:gap-16">
        <div className="lg:sticky lg:top-28 lg:self-start">
          <p className="label mb-2">{t('models.kicker')}</p>
          <h2 id="models-title" className="max-w-lg text-3xl font-semibold tracking-tight sm:text-4xl">{t('models.h')}</h2>
          <p className="mt-4 max-w-lg text-sm leading-relaxed text-mute sm:text-base">{t('models.sub')}</p>
          {models.length > 0 && (
            <p className="mt-4 inline-flex items-center gap-2 text-sm text-cyan-200">
              <span className="h-2 w-2 rounded-full bg-emerald-400" aria-hidden="true" />
              {t('models.available', { n: jumlah(models.length) })}
            </p>
          )}
          <div className="mt-7 rounded-xl border border-white/10 bg-white/[.035] p-3">
            <span className="text-xs text-mute">{t('models.base')}</span>
            <div className="mt-2 flex items-center gap-2">
              <code className="min-w-0 flex-1 truncate text-xs text-cyan-200">{base}</code>
              <button
                type="button"
                onClick={async () => {
                  try {
                    await navigator.clipboard.writeText(base);
                    setCopied(true);
                    toast.ok(t('models.copied'));
                    setTimeout(() => setCopied(false), 1600);
                  } catch {
                    setCopied(false);
                  }
                }}
                aria-label={t('models.copy')}
                className="grid h-11 w-11 shrink-0 place-items-center rounded-lg border border-white/10 text-mute transition-colors hover:text-text"
              >
                {copied ? <Check className="h-4 w-4 text-emerald-300" aria-hidden="true" /> : <Copy className="h-4 w-4" aria-hidden="true" />}
              </button>
            </div>
          </div>
          <Link href={`${prefix}/docs`} className="mt-6 inline-flex min-h-11 items-center gap-2 text-sm font-medium text-cyan-300 hover:text-cyan-200">
            {t('models.all')} <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Link>
        </div>

        <div>
          {modelsReq.loading ? (
            <div className="grid gap-4 sm:grid-cols-2" aria-busy="true">{[0, 1, 2, 3].map((i) => <div key={i} className="card p-5"><div className="skel h-32" /></div>)}</div>
          ) : modelsReq.err ? (
            <p role="alert" className="rounded-xl border border-rose-300/20 bg-rose-400/10 p-4 text-sm text-rose-200">{t('err.load')}</p>
          ) : models.length === 0 ? (
            <p className="rounded-xl border border-white/10 p-5 text-sm text-mute">{t('price.noModels')}</p>
          ) : (
            <ModelGrid models={models} health={health} freeDailyTokens={freeDailyTokens} initial={6} />
          )}
        </div>
      </div>
    </section>
  );
}
