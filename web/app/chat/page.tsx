'use client';
// Chat / Playground: pilih model, kirim pesan, tampilkan balasan + biaya. Riwayat di state.
import { useEffect, useRef, useState } from 'react';
import Image from 'next/image';
import { useFetch } from '@/lib/useApi';
import { useToast } from '@/components/ui/toast';
import { rupiah, asList } from '@/lib/utils';
import { familyOf, prettyName } from '@/components/landing/ModelCatalog';
import { useT } from '@/lib/i18n';
import { apiErrorMessage } from '@/lib/api-errors';
import { ArrowUp, ChevronDown, Square } from 'lucide-react';

type Model = { name: string; type?: string };
type Msg = { role: 'user' | 'assistant'; content: string; cost?: number };
type History = { id: string; title: string; model: string; messages: Msg[] };

export default function ChatPage() {
  const { t } = useT();
  const { data, loading, err, reload } = useFetch<unknown>('/models');
  const historyFetch = useFetch<unknown>('/chat/history');
  const models = asList<Model>(data, 'models');
  const histories = asList<History>(historyFetch.data, 'histories');
  const [model, setModel] = useState('');
  const [input, setInput] = useState('');
  const [msgs, setMsgs] = useState<Msg[]>([]);
  const [busy, setBusy] = useState(false);
  const toast = useToast();
  const endRef = useRef<HTMLLIElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => { if (models.length && !model) setModel(models[0].name); }, [models, model]);
  useEffect(() => { endRef.current?.scrollIntoView({ block: 'end' }); }, [msgs]);
  useEffect(() => { if (!input && inputRef.current) inputRef.current.style.height = '58px'; }, [input]);

  const saveHistory = async (messages: Msg[]) => {
    await fetch('/api/chat/history', { method: 'POST', headers: { 'Content-Type': 'application/json' }, credentials: 'same-origin', body: JSON.stringify({ model, messages: messages.map(({ role, content }) => ({ role, content })) }) });
  };

  const kirim = async () => {
    const isi = input.trim();
    if (!isi || !model) return;
    // Riwayat ikut dikirim supaya model punya konteks. Balasan diisi bertahap
    // saat stream (SSE); placeholder assistant ditambahkan lebih dulu.
    const riwayat: Msg[] = [...msgs, { role: 'user', content: isi }];
    setMsgs([...riwayat, { role: 'assistant', content: '' }]);
    setInput('');
    setBusy(true);

    const ac = new AbortController();
    const timeout = window.setTimeout(() => ac.abort(), 60000);
    abortRef.current = ac;
    // Selalu menulis ke pesan assistant terakhir (placeholder streaming).
    const tambah = (teks: string) =>
      setMsgs((ms) => ms.map((m, i) => (i === ms.length - 1 ? { ...m, content: m.content + teks } : m)));
    const setBiaya = (cost: number) =>
      setMsgs((ms) => ms.map((m, i) => (i === ms.length - 1 ? { ...m, cost } : m)));

    let assistantText = '';
    try {
      // fetch (bukan axios): axios di browser tidak memberi ReadableStream per-chunk.
      // URL relatif: lewat rewrite Next, cookie sesi HttpOnly ikut otomatis.
      const res = await fetch('/api/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'same-origin',
        body: JSON.stringify({
          model,
          messages: riwayat.map((m) => ({ role: m.role, content: m.content })),
          stream: true,
        }),
        signal: ac.signal,
      });

      if (!res.ok) {
        let code = '';
        try { const d = await res.json(); code = typeof d?.error === 'object' ? d.error.code ?? '' : ''; } catch { /* use status */ }
        const pesan = code
          ? apiErrorMessage(code, t('cht.sendFailHttp', { n: res.status }))
          : t('cht.sendFailHttp', { n: res.status });
        throw new Error(pesan);
      }

      const ct = res.headers.get('content-type') || '';
      if (!ct.includes('text/event-stream')) {
        // Fallback: respons JSON biasa (gateway belum mendukung stream).
        const d = await res.json();
        const teks = d?.choices?.[0]?.message?.content ?? d?.reply ?? d?.content ?? '';
        const biaya = Number(d?.usage?.cost ?? d?.cost);
        setMsgs((ms) => ms.map((m, i) => (i === ms.length - 1
          ? { ...m, content: teks || t('cht.emptyReply'), cost: Number.isFinite(biaya) ? biaya : undefined }
          : m)));
        assistantText = teks || t('cht.emptyReply');
        await saveHistory([...riwayat, { role: 'assistant', content: assistantText }]);
        return;
      }

      // SSE: baris "data: {choices:[{delta:{content}}]}" sampai "data: [DONE]".
      const reader = res.body?.getReader();
      if (!reader) throw new Error(t('cht.noStream'));
      const dec = new TextDecoder();
      let buf = '';
      let selesai = false;
      while (!selesai) {
        const { done, value } = await reader.read();
        if (done) break;
        buf += dec.decode(value, { stream: true });
        const baris = buf.split('\n');
        buf = baris.pop() || '';
        for (const b of baris) {
          const s = b.trim();
          if (!s.startsWith('data:')) continue;
          const payload = s.slice(5).trim();
          if (payload === '[DONE]') { selesai = true; break; }
          try {
            const j = JSON.parse(payload);
            const delta = j?.choices?.[0]?.delta?.content ?? j?.choices?.[0]?.text ?? '';
            if (delta) { assistantText += delta; tambah(delta); }
            const biaya = Number(j?.usage?.cost);
            if (j?.usage?.cost !== undefined && Number.isFinite(biaya)) setBiaya(biaya);
          } catch { /* keepalive/komentar SSE: lewati */ }
        }
      }
      setMsgs((ms) => ms.map((m, i) => (i === ms.length - 1 && !m.content ? { ...m, content: t('cht.emptyReply') } : m)));
      if (!assistantText) assistantText = t('cht.emptyReply');
      await saveHistory([...riwayat, { role: 'assistant', content: assistantText }]);
    } catch (e) {
      // Stop oleh user = bukan error: teks yang sudah masuk tetap ditampilkan.
      if ((e as Error)?.name === 'AbortError') {
        setMsgs((ms) => ms.map((m, i) => (i === ms.length - 1 && !m.content ? { ...m, content: t('cht.timeout') } : m)));
      } else {
        toast.err((e as Error)?.message || t('cht.sendFail'));
        // Balasan kosong dibuang supaya user tidak melihat gelembung hampa.
        setMsgs((ms) => (ms.length && ms[ms.length - 1].role === 'assistant' && !ms[ms.length - 1].content ? ms.slice(0, -1) : ms));
      }
    } finally {
      window.clearTimeout(timeout);
      abortRef.current = null;
      setBusy(false);
    }
  };

  const streamingMsg = busy && msgs.length > 0 && msgs[msgs.length - 1].role === 'assistant' && !!msgs[msgs.length - 1].content;
  const selectedFamily = familyOf(model);

  return (
    <div className="flex min-h-[calc(100dvh-3.5rem)] gap-4 px-4 sm:px-6">
      <aside className="hidden w-56 shrink-0 border-r border-white/10 py-6 pr-4 lg:block">
        <p className="mb-3 text-xs font-semibold uppercase tracking-wider text-mute">History</p>
        {historyFetch.loading && <p className="text-xs text-mute">Loading…</p>}
        <div className="space-y-1">{histories.map((h) => <button key={h.id} type="button" onClick={() => { setMsgs(h.messages || []); setModel(h.model || model); }} className="block w-full truncate rounded-lg px-3 py-2 text-left text-xs text-mute hover:bg-white/10 hover:text-text">{h.title || 'Untitled chat'}</button>)}</div>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
      {err && (
        <div role="alert" className="mx-auto mt-5 flex w-full max-w-3xl items-center justify-between gap-3 rounded-xl border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger">
          <span>{err}</span>
          <button type="button" onClick={() => void reload()} className="shrink-0 rounded-lg border border-danger/40 px-3 py-1.5 text-xs font-medium hover:bg-danger/10">{t('cht.retry')}</button>
        </div>
      )}

      <section aria-label={t('cht.ariaLog')} className="mx-auto flex w-full max-w-3xl flex-1 flex-col">
        <div className={`flex flex-1 flex-col ${msgs.length === 0 ? 'justify-center pb-[12vh]' : ''}`}>
          {msgs.length === 0 ? (
            <h1 className="text-center text-2xl font-medium tracking-tight sm:text-3xl">{t('chat.start')}</h1>
          ) : (
            <ul aria-live="polite" className="space-y-6 py-8">
              {msgs.map((m, i) => (
                <li key={i} className={`flex ${m.role === 'user' ? 'justify-end' : 'justify-start'}`}>
                  <div className={m.role === 'user'
                    ? 'max-w-[min(86%,38rem)] rounded-2xl rounded-br-md bg-white/[.07] px-4 py-3 text-sm leading-relaxed'
                    : 'max-w-[min(92%,42rem)] text-sm leading-7 text-text'}>
                    <p className="mb-1.5 text-[11px] font-medium uppercase tracking-[.12em] text-mute">
                      {m.role === 'user' ? t('chat.user') : t('chat.assistant')}
                    </p>
                    <p className="whitespace-pre-wrap break-words">{m.content}</p>
                    {m.cost !== undefined && (
                      <p className="mt-2 font-mono text-xs tabular-nums text-mute">{t('chat.cost', { n: rupiah(m.cost) })}</p>
                    )}
                  </div>
                </li>
              ))}
              {busy && (
                <li className="flex items-center gap-3 text-sm text-mute" role="status">
                  <span className="dots" aria-hidden="true"><span /><span /><span /></span>
                  {streamingMsg ? t('chat.writing') : t('chat.waiting')}
                </li>
              )}
              <li ref={endRef} aria-hidden="true" />
            </ul>
          )}

          <form
            className={msgs.length === 0 ? 'mt-8 w-full' : 'sticky bottom-0 mt-auto bg-gradient-to-t from-bg via-bg/95 to-transparent pb-4 pt-8'}
            onSubmit={(e) => { e.preventDefault(); if (!busy) kirim(); }}
          >
            <div className="relative rounded-[28px] border border-white/10 bg-[#15161a] shadow-[inset_0_1px_0_rgb(255_255_255/.06),0_16px_50px_rgb(0_0_0/.22)] transition-colors focus-within:border-white/20">
              <textarea
                ref={inputRef}
                id="pesan"
                rows={1}
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onInput={(e) => {
                  e.currentTarget.style.height = 'auto';
                  e.currentTarget.style.height = `${Math.min(e.currentTarget.scrollHeight, 176)}px`;
                }}
                placeholder={t('chat.placeholder')}
                aria-label={t('chat.placeholder')}
                className="block min-h-[58px] max-h-44 w-full resize-none overflow-y-auto bg-transparent py-[18px] pl-5 pr-16 text-sm leading-[22px] text-text outline-none placeholder:text-mute"
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); if (!busy) kirim(); }
                }}
              />
              <button
                type={busy ? 'button' : 'submit'}
                onClick={busy ? () => abortRef.current?.abort() : undefined}
                disabled={!busy && (!input.trim() || !model)}
                aria-label={busy ? t('chat.stop') : t('chat.send')}
                className="absolute bottom-3 right-3 grid h-9 w-9 place-items-center rounded-full bg-text text-bg transition-[opacity,transform] hover:scale-[1.03] active:scale-95 disabled:cursor-not-allowed disabled:opacity-25"
              >
                {busy ? <Square className="h-3.5 w-3.5 fill-current" aria-hidden="true" /> : <ArrowUp className="h-4 w-4" aria-hidden="true" />}
              </button>
            </div>

            <div className="mt-3 flex flex-col gap-2 text-xs text-mute sm:flex-row sm:items-center sm:justify-between">
              <details className="relative w-fit max-w-full">
                <summary aria-label={t('chat.model')} className="flex h-8 max-w-[min(78vw,21rem)] cursor-pointer list-none items-center gap-2 rounded-full border border-white/10 bg-white/[.055] py-1 pl-3 pr-8 text-xs text-text outline-none transition-colors hover:border-white/20 focus-visible:ring-1 focus-visible:ring-cyan-300/40 [&::-webkit-details-marker]:hidden">
                  {selectedFamily.logo
                    ? <Image src={selectedFamily.logo} alt="" width={14} height={14} className="h-3.5 w-3.5 shrink-0 object-contain" unoptimized />
                    : model
                      ? <span aria-hidden="true" className="grid h-3.5 w-3.5 shrink-0 place-items-center rounded-full bg-white/10 font-mono text-[9px] font-bold text-mute">{prettyName(model).charAt(0)}</span>
                      : null}
                  <span className="truncate">{prettyName(model)}{models.find((m) => m.name === model)?.type ? ` · ${models.find((m) => m.name === model)?.type}` : ''}</span>
                  <ChevronDown className="absolute right-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2" aria-hidden="true" />
                </summary>
                <div role="listbox" aria-label={t('chat.model')} className="absolute bottom-10 left-0 z-20 max-h-72 w-[min(78vw,21rem)] overflow-y-auto rounded-lg border border-border bg-[#15161a] p-1 shadow-xl">
                  {loading && <p className="px-3 py-2">{t('chat.loadingModels')}</p>}
                  {!loading && models.length === 0 && <p className="px-3 py-2">{t('chat.noModels')}</p>}
                  {models.map((m) => {
                    const family = familyOf(m.name);
                    return <button key={m.name} type="button" role="option" aria-selected={m.name === model} onClick={(e) => { setModel(m.name); e.currentTarget.closest('details')?.removeAttribute('open'); }} className={`flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-xs hover:bg-white/10 ${m.name === model ? 'bg-white/10 text-text' : 'text-mute'}`}>
                    {family.logo
                      ? <Image src={family.logo} alt="" width={14} height={14} className="h-3.5 w-3.5 shrink-0 object-contain" unoptimized />
                      : <span aria-hidden="true" className="grid h-3.5 w-3.5 shrink-0 place-items-center rounded-sm bg-white/10 font-mono text-[9px] font-bold text-mute">{prettyName(m.name).charAt(0) || '?'}</span>}
                      <span className="truncate">{prettyName(m.name)}{m.type ? ` · ${m.type}` : ''}</span>
                    </button>;
                  })}
                </div>
              </details>
              <p className="text-center sm:text-right">{t('chat.hint')}</p>
            </div>
          </form>
        </div>
      </section>
    </div>
    </div>
  );
}
