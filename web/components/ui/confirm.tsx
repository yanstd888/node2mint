'use client';
// Konfirmasi destruktif seragam. Menggantikan window.confirm(): memblokir thread,
// tidak bisa di-i18n, tidak bisa bergaya, dan tidak bisa disesuaikan tema.
// Dipasang sekali di app/layout.tsx, dipakai dengan `await ask({ title })`.
import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from 'react';
import { Dialog } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { useT } from '@/lib/i18n';

type Tanya = { title: string; body?: string; confirmLabel?: string; variant?: 'default' | 'destructive' };
type Ask = (t: Tanya) => Promise<boolean>;

const Ctx = createContext<Ask>(() => Promise.resolve(false));

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const { t } = useT();
  const [q, setQ] = useState<Tanya | null>(null);
  const jawab = useRef<(v: boolean) => void>(() => {});

  const ask = useCallback((tanya: Tanya) => {
    setQ(tanya);
    return new Promise<boolean>((res) => { jawab.current = res; });
  }, []);

  const tutup = (v: boolean) => { setQ(null); jawab.current(v); };

  return (
    <Ctx.Provider value={ask}>
      {children}
      {/* Fokus awal jatuh ke tombol pertama = Batal, sisi aman untuk aksi destruktif. */}
      <Dialog open={!!q} title={q?.title ?? ''} onClose={() => tutup(false)}>
        {q?.body && <p className="mb-5 text-sm leading-relaxed text-mute">{q.body}</p>}
        <div className="flex justify-end gap-2">
          <Button variant="outline" onClick={() => tutup(false)}>{t('sel.cancel')}</Button>
          <Button variant={q?.variant ?? 'destructive'} onClick={() => tutup(true)}>{q?.confirmLabel ?? t('ad.common.delete')}</Button>
        </div>
      </Dialog>
    </Ctx.Provider>
  );
}

export const useConfirm = () => useContext(Ctx);
