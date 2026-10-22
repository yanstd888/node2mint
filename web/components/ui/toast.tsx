'use client';
// Toast global (brief §3 micro-interactions + Fanny #5): fixed bottom-4 right-4,
// success 3 dtk auto, error 7 dtk + tombol close, stack maks 3 (FIFO),
// animasi slide+fade 150ms CSS, hormati prefers-reduced-motion.
// Pemakaian: const toast = useToast(); toast.ok('Tersimpan.'); toast.err('Gagal.');
import { createContext, useCallback, useContext, useRef, useState, ReactNode } from 'react';
import { CheckCircle2, AlertTriangle, X } from 'lucide-react';

type Item = { id: number; kind: 'ok' | 'err'; msg: string; action?: { label: string; run: () => void } };

const Ctx = createContext<{ ok: (msg: string, action?: Item['action']) => void; err: (msg: string) => void }>({
  ok: () => {}, err: () => {},
});

export function useToast() {
  return useContext(Ctx);
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<Item[]>([]);
  const seq = useRef(0);

  const push = useCallback((kind: Item['kind'], msg: string, action?: Item['action']) => {
    const id = ++seq.current;
    // Maks 3: buang paling tua, masukkan yang baru (FIFO). Toast ber-Undo
    // (action) dapat 8 dtk, lebih lama dari 3 dtk biasa — Undo harus sempat
    // diklik.
    setItems((cur) => [...cur.slice(-2), { id, kind, msg, action }]);
    setTimeout(() => setItems((cur) => cur.filter((t) => t.id !== id)), kind === 'ok' ? (action ? 8000 : 3000) : 7000);
  }, []);

  const close = useCallback((id: number) => setItems((cur) => cur.filter((t) => t.id !== id)), []);

  return (
    <Ctx.Provider value={{ ok: (m, a) => push('ok', m, a), err: (m) => push('err', m) }}>
      {children}
      {/* role=alert tiap item sudah assertive; container tidak perlu aria-live ganda */}
      <div className="fixed bottom-4 right-4 z-[60] flex flex-col gap-2 w-80 max-w-[calc(100vw-32px)]">
        {items.map((t) => (
          <div key={t.id}
            role={t.kind === 'err' ? 'alert' : 'status'}
            className={`toast-in flex items-start gap-3 rounded-xl border p-3 pr-2 backdrop-blur-md shadow-lg ${
              t.kind === 'err' ? 'bg-red-500/15 border-red-500/30' : 'bg-emerald-500/15 border-emerald-500/30'
            }`}>
            {t.kind === 'err'
              ? <AlertTriangle className="w-4 h-4 mt-1 shrink-0 text-red-300" aria-hidden="true" />
              : <CheckCircle2 className="w-4 h-4 mt-1 shrink-0 text-emerald-300" aria-hidden="true" />}
            <p className="flex-1 text-sm text-text leading-relaxed">{t.msg}</p>
            {t.action && (
              <button onClick={() => { close(t.id); t.action?.run(); }}
                className="shrink-0 self-center rounded-md border border-border bg-surface-hi px-2.5 py-1 text-xs font-medium text-text hover:bg-surface focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60">
                {t.action.label}
              </button>
            )}
            <button onClick={() => close(t.id)} aria-label="Tutup notifikasi"
              className="p-1 text-mute hover:text-text rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60">
              <X className="w-3.5 h-3.5" aria-hidden="true" />
            </button>
          </div>
        ))}
      </div>
    </Ctx.Provider>
  );
}
