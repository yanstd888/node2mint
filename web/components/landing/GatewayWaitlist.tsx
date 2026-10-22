'use client';

import { FormEvent, useState } from 'react';
import { CheckCircle2, Mail } from 'lucide-react';
import { api } from '@/lib/api';

export default function GatewayWaitlist() {
  const [email, setEmail] = useState('');
  const [state, setState] = useState<'idle' | 'busy' | 'done' | 'error'>('idle');
  const [message, setMessage] = useState('');
  async function submit(e: FormEvent) {
    e.preventDefault(); setState('busy'); setMessage('');
    try { await api.post('/waitlist', { email }); setState('done'); setMessage('Email tersimpan. Kami akan memberi tahu saat gateway siap.'); }
    catch { setState('error'); setMessage('Masukkan email yang valid lalu coba lagi.'); }
  }
  return <section id="waitlist" className="scroll-mt-24 py-16 sm:py-24">
    <div className="mx-auto max-w-2xl rounded-2xl border border-cyan-300/20 bg-cyan-400/[.06] p-6 text-center sm:p-10">
      <Mail className="mx-auto h-8 w-8 text-cyan-300" aria-hidden="true" />
      <p className="label mt-4">API GATEWAY</p>
      <h2 className="mt-2 text-3xl font-semibold tracking-tight">API gateway segera hadir</h2>
      <p className="mx-auto mt-3 max-w-lg text-sm leading-relaxed text-mute">Gateway sedang disiapkan. Tinggalkan email untuk mendapat pemberitahuan saat akses model dibuka.</p>
      {state === 'done' ? <p role="status" className="mt-6 inline-flex items-center gap-2 text-sm text-emerald-300"><CheckCircle2 className="h-4 w-4" />{message}</p> : <form onSubmit={submit} className="mx-auto mt-6 flex max-w-md flex-col gap-2 sm:flex-row"><label className="sr-only" htmlFor="gateway-waitlist-email">Email</label><input id="gateway-waitlist-email" type="email" required value={email} onChange={(e) => setEmail(e.target.value)} placeholder="email@contoh.com" className="min-h-11 min-w-0 flex-1 rounded-lg border border-white/15 bg-black/20 px-3 text-base text-white outline-none focus:border-cyan-300 md:text-sm" /><button type="submit" disabled={state === 'busy'} className="min-h-11 rounded-lg bg-cyan-300 px-5 text-sm font-semibold text-[#071015]">{state === 'busy' ? 'Menyimpan…' : 'Daftar waitlist'}</button></form>}
      {message && state === 'error' && <p role="alert" className="mt-3 text-sm text-rose-300">{message}</p>}
    </div>
  </section>;
}
