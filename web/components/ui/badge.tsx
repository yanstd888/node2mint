// Badge status semantik untuk SEMUA halaman. Palet Fanny (soft-tone dark):
// emerald=sukses, amber=menunggu, red=bahaya, orange=sengketa, ring inset.
import { ReactNode } from 'react';
import { cn } from '@/lib/utils';

const TONE = {
  ok: 'bg-emerald-500/15 text-emerald-300 ring-1 ring-inset ring-emerald-500/25',
  pending: 'bg-amber-500/15 text-amber-300 ring-1 ring-inset ring-amber-500/25',
  danger: 'bg-red-500/15 text-red-300 ring-1 ring-inset ring-red-500/25',
  warn: 'bg-orange-500/15 text-orange-300 ring-1 ring-inset ring-orange-500/25',
  netral: 'bg-surface-hi text-mute ring-1 ring-inset ring-border',
} as const;

// Status apa pun (huruf bebas) → tone. Yang tidak dikenal = netral, bukan salah warna.
const MAP: Record<string, keyof typeof TONE> = {
  COMPLETED: 'ok', PAID: 'ok', ACTIVE: 'ok', AKTIF: 'ok', SEHAT: 'ok', HEALTHY: 'ok',
  UP: 'ok', OK: 'ok', OPERATIONAL: 'ok', TAYANG: 'ok', APPROVED: 'ok', SUCCESS: 'ok',
  PENDING: 'pending', PROCESSING: 'pending', DEGRADED: 'pending', SLOW: 'pending',
  DISPUTED: 'warn', FLAGGED: 'warn', MAINTENANCE: 'warn',
  CANCELLED: 'danger', REFUNDED: 'danger', FAILED: 'danger', REJECTED: 'danger',
  BERMASALAH: 'danger', DOWN: 'danger', ERROR: 'danger', OUTAGE: 'danger', HABIS: 'danger', 'TIDAK AKTIF': 'danger',
};

export type BadgeTone = keyof typeof TONE;

export function Badge({ children, tone, className }: { children: ReactNode; tone?: BadgeTone; className?: string }) {
  const label = typeof children === 'string' || typeof children === 'number' ? String(children).trim() : '';
  const t = tone ?? MAP[label.toUpperCase()] ?? 'netral';
  return (
    <span className={cn('inline-flex items-center text-[11px] px-2 py-1 rounded-full font-medium uppercase tracking-wide', TONE[t], className)}>
      {children || '-'}
    </span>
  );
}
