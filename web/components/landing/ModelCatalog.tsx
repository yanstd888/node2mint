'use client';
// Satu kartu model + grid "lihat semua" dipakai tiga tempat (hero, katalog
// homepage, /pricing#models) supaya bentuknya tidak bercabang jadi tiga versi.
// Data berasal dari /models (publik) + /status (kesehatan live).
import { useState } from 'react';
import type { ReactElement } from 'react';
import Link from 'next/link';
import { ArrowUpRight, Check, Copy, Cpu, ImageIcon, Video, Music, Eye, Layers } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { useT, useUrlPrefix } from '@/lib/i18n';
import { tanggalLang } from '@/lib/tanggal';
import { LockedOverlay } from '@/components/LockedOverlay';

export type Model = {
  id: string;
  name: string;
  type?: string;
  context_length?: number;
  tier?: string;
  token_multiplier?: number;
  last_checked?: string;
};
export type Health = { name?: string; status?: string; avg_latency_ms?: number };

// Icon per model type — visual preview in catalog cards. ponytail: sekadar
// ikon, tidak memengaruhi data model. Tambahkan jika ada tipe baru.
const TYPE_ICON: Record<string, ReactElement> = {
  TEXT: <Cpu className="h-3 w-3" aria-hidden="true" />,
  IMAGE: <ImageIcon className="h-3 w-3" aria-hidden="true" />,
  VIDEO: <Video className="h-3 w-3" aria-hidden="true" />,
  AUDIO: <Music className="h-3 w-3" aria-hidden="true" />,
  VISION: <Eye className="h-3 w-3" aria-hidden="true" />,
  EMBEDDING: <Layers className="h-3 w-3" aria-hidden="true" />,
  RERANK: <Layers className="h-3 w-3" aria-hidden="true" />,
};
export const typeIcon = (t?: string) => TYPE_ICON[t || 'TEXT'] ?? TYPE_ICON.TEXT;

// Nama publik -> keluarga + logo. Yang tidak dikenal dapat ikon netral, bukan
// tebakan logo. Logo yang dipakai sudah ada di public/provider-logos (ticker).
const FAMILI: [RegExp, string, string][] = [
  [/claude|anthropic/i, 'Anthropic', 'anthropic'],
  [/gpt|openai/i, 'OpenAI', 'openai'],
  [/gemini|google|palm/i, 'Gemini', 'gemini'],
  [/deepseek/i, 'DeepSeek', 'deepseek'],
  [/grok/i, 'xAI', 'grok'],
  [/llama|meta/i, 'Meta AI', 'meta'],
  [/mistral|mixtral/i, 'Mistral AI', 'mistralai'],
  [/moonshot|kimi/i, 'Moonshot AI', 'moonshot'],
  [/zai|zhipu/i, 'Zhipu AI', 'zai'],
  [/qwen/i, 'Alibaba', 'alibaba'],
  [/minimax/i, 'MiniMax', 'minimax'],
];

export const familyOf = (name: string) => {
  const hit = FAMILI.find(([re]) => re.test(name));
  return { label: hit?.[1] ?? '', logo: hit?.[2] ? `/provider-logos/${hit[2]}.svg` : '' };
};

const UPPER = new Set(['gpt', 'ai', 'vl', 'llm', 'api', 'sdk', 'ocr']);

// "claude-3-5-sonnet" -> "Claude 3.5 Sonnet"; "gpt-4o-mini" -> "GPT 4o Mini".
// Angka berurutan dipisah - atau . dikelompokkan jadi versi: "fable-5-1" -> "Fable 5.1".
export const prettyName = (name: string) =>
  name
    .replace(/[-_]+/g, ' ')
    .split(' ')
    .filter(Boolean)
    .map((w) => {
      if (UPPER.has(w.toLowerCase())) return w.toUpperCase();
      // angka berturut-turut "5 1" -> "5.1" (mis. FABLE-5-1 -> Fable 5.1)
      const v = w.charAt(0).toUpperCase() + w.slice(1);
      return v;
    })
    .join(' ')
    // angka berurutan dipisah -/. dikelompokkan jadi versi: "5 1" -> "5.1", "3 5" -> "3.5".
    // Angka 2 digit (mis. 01) tidak kena: \b(\d+) (\d)\b hanya gabung satu-spasi-satu-digit.
    .replace(/\b(\d+) (\d)\b/g, '$1.$2');

const dot = (status?: string) => {
  const v = (status ?? '').toLowerCase();
  if (['ok', 'up', 'healthy', 'operational'].includes(v)) return 'bg-emerald-400';
  if (['degraded', 'limited', 'pending'].includes(v)) return 'bg-amber-400';
  if (['down', 'error', 'failed'].includes(v)) return 'bg-rose-400';
  return 'bg-slate-500';
};

export function ModelCard({ model, health, freeDailyTokens = 0, className = '', delay = 0 }: { model: Model; health?: Health; freeDailyTokens?: number; className?: string; delay?: number }) {
  const { t, lang } = useT();
  const prefix = useUrlPrefix();
  const [copied, setCopied] = useState(false);
  const fam = familyOf(model.name);
  const num = (v: number) => v.toLocaleString(lang === 'id' ? 'id-ID' : 'en-US');
  const latency = Math.round(Number(health?.avg_latency_ms ?? 0));
  const sehat = ['ok', 'up', 'healthy', 'operational'].includes((health?.status ?? '').toLowerCase());
  const mult = Number(model.token_multiplier ?? 0) || 1;


  const card = (
    <article className={`item-card anim-fade-up p-5 ${className}`} style={{ animationDelay: `${delay}s` }}>
      <div className="flex items-center justify-between gap-3">
        <span className="flex min-w-0 items-center gap-2 text-xs text-mute">
          <span className="grid h-7 w-7 shrink-0 place-items-center rounded-lg border border-white/10 bg-white/[.04]">
            {fam.logo
              ? <img src={fam.logo} alt="" aria-hidden="true" className="h-4 w-4 object-contain" />
              : <Cpu className="h-4 w-4" aria-hidden="true" />}
          </span>
          <span className="truncate">{fam.label || t('models.other')}</span>
        </span>
        <Badge tone={model.tier === 'FREE' ? 'ok' : 'netral'}>{model.tier === 'FREE' ? t('models.freeTag') : model.tier || t('models.paidTag')}</Badge>
      </div>

      <h3 className="mt-4 truncate text-lg font-medium tracking-tight">{prettyName(model.name)}</h3>

      <div className="mt-1 flex items-center gap-2">
        <code className="min-w-0 flex-1 truncate font-mono text-xs text-cyan-200/90">{model.name}</code>
        <button
          type="button"
          aria-label={t('models.copyId')}
          onClick={async () => {
            try {
              await navigator.clipboard.writeText(model.name);
              setCopied(true);
              setTimeout(() => setCopied(false), 1600);
            } catch {
              setCopied(false);
            }
          }}
          className="grid h-11 w-11 shrink-0 place-items-center rounded-md text-mute transition-colors hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60"
        >
          {copied ? <Check className="h-3.5 w-3.5 text-emerald-300" aria-hidden="true" /> : <Copy className="h-3.5 w-3.5" aria-hidden="true" />}
        </button>
      </div>

      <p className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-mute">
        <span className="inline-flex items-center gap-1.5">
          <span className={`h-2 w-2 rounded-full ${dot(health?.status)}`} aria-hidden="true" />
          {sehat ? `${t('models.healthy')}${latency > 0 ? ` · ${num(latency)} ms` : ''}` : t('models.unknown')}
        </span>
        {model.last_checked ? <span>{t('st.last', { date: tanggalLang(model.last_checked, lang, true) })}</span> : null}
        {model.context_length ? <span>{t('models.context', { n: num(model.context_length) })}</span> : null}
        <span className="ml-auto inline-flex items-center gap-1.5 text-xs text-mute">{typeIcon(model.type)}{model.type ?? 'TEXT'}</span>
      </p>

      <p className="mt-3 text-xs leading-relaxed text-mute">
        {model.tier === 'FREE'
          ? t('models.freeTerms', { n: freeDailyTokens > 0 ? num(freeDailyTokens) : '—' })
          : t('models.paidTerms', { n: String(mult) })}
      </p>

      <div className="mt-4 flex items-center justify-between gap-3 border-t border-white/10 pt-3">
        <span className="text-[11px] uppercase tracking-[.12em] text-mute" title={t('tip.mult')}>
          {t('models.usage')} · ×{mult}
        </span>
        <Link href={`${prefix}/docs`} aria-label={t('models.docs')} className="grid h-11 w-11 place-items-center text-mute transition-colors hover:text-cyan-200">
          <ArrowUpRight className="h-4 w-4" aria-hidden="true" />
        </Link>
      </div>
    </article>
  );
  return model.tier === 'FREE' ? <LockedOverlay feature="api_free">{card}</LockedOverlay> : card;
}

// Grid katalog: tampil `initial` kartu dulu; kalau katalog lebih panjang,
// tombol "lihat semua" membuka sisanya tanpa berpindah halaman.
export function ModelGrid({ models, health, freeDailyTokens = 0, initial = 6, cols = 'sm:grid-cols-2' }: { models: Model[]; health: Health[]; freeDailyTokens?: number; initial?: number; cols?: string }) {
  const { t } = useT();
  const [all, setAll] = useState(false);
  const shown = all ? models : models.slice(0, initial);

  return (
    <>
      <ul className={`grid gap-4 ${cols}`}>
        {shown.map((m, i) => (
          <li key={m.id}>
            <ModelCard model={m} health={health.find((h) => h.name === m.name)} freeDailyTokens={freeDailyTokens} className="h-full" delay={Math.min(i, 4) * 0.05} />
          </li>
        ))}
      </ul>
      {models.length > initial && (
        <div className="mt-5 flex justify-center">
          <button
            type="button"
            onClick={() => setAll((v) => !v)}
            aria-expanded={all}
            className="min-h-11 rounded-xl border border-white/15 px-5 text-sm font-medium transition-colors hover:border-cyan-300/40 hover:text-cyan-200 active:scale-[.98]"
          >
            {all ? t('models.showLess') : t('models.showAll', { n: models.length })}
          </button>
        </div>
      )}
    </>
  );
}
