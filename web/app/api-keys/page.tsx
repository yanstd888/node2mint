'use client';
// API Keys: list, buat (key mentah tampil SEKALI), revoke dua-langkah tanpa confirm() native.
import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Dialog } from '@/components/ui/dialog';
import { Key, Plus, Trash2, Copy, Check, AlertTriangle, AlertOctagon, Wallet } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Pagination, useClientPagination } from '@/components/ui/Pagination';
import { useToast } from '@/components/ui/toast';
import { tanggalLang } from '@/lib/tanggal';
import { money } from '@/lib/utils';
import { useT } from '@/lib/i18n';
import { useAuth } from '@/lib/auth';

type ApiKey = {
  id: string;
  name: string;
  prefix: string;
  rate_limit: number;
  is_active: boolean;
  revoked_at?: string | null;
  last_used: string | null;
  created_at: string;
  models?: string; // whitelist CSV; '' = semua model
};

export default function ApiKeysPage() {
  const { t, lang } = useT();
  const { user } = useAuth();
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [gagal, setGagal] = useState(false);
  const [showNew, setShowNew] = useState(false);
  const [rawKey, setRawKey] = useState('');
  const [newName, setNewName] = useState('');
  const [newModels, setNewModels] = useState('');
  const [creating, setCreating] = useState(false);
  const [confirmId, setConfirmId] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const toast = useToast();
  // Salin ulang key penuh: key disimpan terenkripsi di gateway, reveal kembali
  // hanya untuk pemiliknya. Key lama (sebelum fitur ini) hanya punya hash.
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const keysPage = useClientPagination(keys, 10);
  const copyKey = async (id: string) => {
    try {
      const { data } = await api.get<{ key: string }>(`/keys/${id}/reveal`);
      await navigator.clipboard.writeText(data.key);
      setCopiedId(id);
      toast.ok(t('key.copied'));
    } catch {
      toast.err(t('key.toastReveal'));
    }
  };

  type Usage = {
    totals: { requests: number; input_tokens: number; cached_tokens: number; output_tokens: number; est_tokens: number };
    keys: { id: string; name: string; requests: number; input_tokens: number; output_tokens: number; est_tokens: number }[];
  };
  const [usage, setUsage] = useState<Usage | null>(null);
  const fmt = (n: number) => Math.round(n).toLocaleString(lang === 'id' ? 'id-ID' : 'en-US');

  const load = async () => {
    setLoading(true);
    try {
      const { data } = await api.get<{ keys: ApiKey[] }>('/keys');
      setKeys(data.keys || []);
      setGagal(false);
      // Statistik pemakaian boleh gagal sendiri: jangan blokir daftar key.
      api.get<Usage>('/keys/usage').then(({ data: u }) => setUsage(u)).catch(() => setUsage(null));
    } catch {
      // Dulu tanpa catch: gagal muat tampil sebagai "Belum ada API key" (empty state palsu).
      setGagal(true);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const create = async () => {
    if (!newName.trim()) return;
    setCreating(true);
    try {
      const { data } = await api.post<{ key: string }>('/keys', {
        name: newName.trim(),
        models: newModels.trim(), // whitelist; '' = semua model
      });
      setRawKey(data.key);
      setShowNew(false);
      setNewName('');
      setNewModels('');
      load();
    } catch {
      // Dulu tanpa catch: dialog tetap terbuka tanpa penjelasan apa pun.
      toast.err(t('key.toastCreateFail'));
    } finally {
      setCreating(false);
    }
  };

  const revoke = async (id: string) => {
    try {
      await api.del(`/keys/${id}`);
      setKeys((items) => items.map((item) => item.id === id ? { ...item, is_active: false, revoked_at: new Date().toISOString() } : item));
      toast.ok(t('key.toastDeleted'));
    } catch { toast.err(t('key.toastDeleteFail')); } finally { setConfirmId(null); }
  };
  const toggleStatus = async (id: string, is_active: boolean) => {
    try {
      await api.patch(`/keys/${id}`, { is_active });
      setKeys((items) => items.map((item) => item.id === id ? { ...item, is_active } : item));
    } catch { toast.err(t('key.toastStatusFail')); }
  };
  const deletePermanent = async (id: string) => {
    try {
      await api.del(`/keys/${id}/permanent`);
      setKeys((items) => items.filter((item) => item.id !== id));
      toast.ok(t('key.toastDeleted'));
    } catch { toast.err(t('key.toastDeleteFail')); } finally { setConfirmId(null); }
  };

  const copyRaw = async () => {
    try {
      await navigator.clipboard.writeText(rawKey);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // clipboard hanya tersedia di secure context; key ini tidak bisa dilihat lagi.
      toast.err(t('key.toastCopyFail'));
    }
  };

  return (
    <div className="p-6 md:p-10 max-w-4xl mx-auto">
      <div className="flex items-center justify-between gap-4 mb-8">
        <div>
          <p className="label mb-1">{t('key.kicker')}</p>
          <h1 className="text-2xl font-semibold tracking-tight">{t('key.h')}</h1>
        </div>
        <Button onClick={() => setShowNew(true)}>
          <Plus className="w-4 h-4" aria-hidden="true" /> {t('key.create')}
        </Button>
      </div>

      {gagal && (
        <p role="alert" className="mb-6 text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">
          {t('key.loadFail')}
        </p>
      )}

      {/* Token exhaust vs wallet: dua hal berbeda. Token habis = request ditolak
          (ikon AlertOctagon, merah). Saldo dompet terpisah (ikon Wallet). */}
      {user && (
        <div className="mb-6 space-y-3">
          {Number(user.tokenRemaining) <= 0 ? (
            <div role="alert" className="flex items-start gap-3 rounded-lg border border-danger/40 bg-danger/10 px-4 py-3 text-sm text-danger">
              <AlertOctagon className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
              <span><strong className="font-semibold">{t('key.tokenEmptyTitle')}</strong> — {t('key.tokenEmpty')}</span>
            </div>
          ) : Number(user.tokenRemaining) < 1000 ? (
            <div role="status" className="flex items-start gap-3 rounded-lg border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-sm text-amber-300">
              <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
              <span><strong className="font-semibold">{t('key.tokenLowTitle')}</strong> — {t('key.tokenLow')}</span>
            </div>
          ) : null}
          <div role="status" className="flex items-start gap-3 rounded-lg border border-border bg-surface-hi px-4 py-3 text-sm text-text">
            <Wallet className="mt-0.5 h-4 w-4 shrink-0 text-cyan-300" aria-hidden="true" />
            <span>{t('key.walletSep', { amount: money(user.balance, lang) })}</span>
          </div>
        </div>
      )}

      {usage && (
        <section aria-label={t('key.stats')} className="mb-8">
          <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
            {[
              { label: t('key.totalReq'), value: fmt(usage.totals.requests), cls: 'text-text', note: '' },
              { label: t('key.totalIn'), value: fmt(usage.totals.input_tokens), cls: 'text-orange-400', note: '' },
              { label: t('key.cached'), value: fmt(usage.totals.cached_tokens), cls: 'text-cyan-300', note: '' },
              { label: t('key.totalOut'), value: fmt(usage.totals.output_tokens), cls: 'text-emerald-400', note: '' },
              { label: t('key.est'), value: fmt(usage.totals.est_tokens), cls: 'text-gold', note: t('key.estNote') },
            ].map((c) => (
              <div key={c.label} className="card p-4">
                <p className="label mb-2">{c.label}</p>
                <p className={`text-xl font-semibold tabular-nums ${c.cls}`}>{c.value}</p>
                {c.note && <p className="text-[10px] text-mute mt-1">{c.note}</p>}
              </div>
            ))}
          </div>
        </section>
      )}

      {usage && <section aria-label={t('key.chart')} className="mb-8 card p-5">
        <h2 className="font-semibold mb-4">{t('key.chart')}</h2>
        {usage.keys.length === 0 ? <p className="text-sm text-mute">{t('key.chartEmpty')}</p> : <div className="space-y-3">{usage.keys.map((u) => { const max = Math.max(...usage.keys.map((x) => x.est_tokens), 1); return <div key={u.id}><div className="mb-1 flex justify-between text-xs"><span className="truncate">{u.name}</span><span className="text-mute">{fmt(u.est_tokens)}</span></div><div className="h-2 rounded-full bg-white/10"><div className="h-2 rounded-full bg-cyan-400" style={{ width: `${Math.max(3, (u.est_tokens / max) * 100)}%` }} /></div></div>; })}</div>}
      </section>}
      {gagal ? null : loading ? (
        <div className="space-y-3">
          {[0, 1, 2].map((i) => <div key={i} className="card p-4 h-24"><div className="skel h-4 w-40 mb-3" /><div className="skel h-3 w-56" /></div>)}
        </div>
      ) : keys.length === 0 ? (
        <div className="card p-12 text-center">
          <Key className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />
          <p className="font-medium">{t('key.emptyH')}</p>
          <p className="text-sm text-mute mt-1 mb-6">{t('key.emptyP')}</p>
          <Button onClick={() => setShowNew(true)}>{t('key.create')}</Button>
        </div>
      ) : (
        <>
          <ul className="space-y-3">
            {keysPage.items.map((k) => (
              <li key={k.id} className="card p-4 flex items-center justify-between gap-4">
                <div className="min-w-0 flex-1">
                <div className="flex items-center gap-3 flex-wrap">
                  <span className={`key-mask ${k.is_active ? '' : 'text-mute line-through'}`}>{k.prefix}••••</span>
                  <Badge tone={k.revoked_at ? 'danger' : k.is_active ? 'ok' : 'pending'}>{k.revoked_at ? t('key.revoked') : k.is_active ? t('acc.on') : t('key.disabled')}</Badge>
                  <button type="button" aria-label={t('key.copyAria', { name: k.name })}
                    onClick={() => copyKey(k.id)}
                    className="p-2 text-mute hover:text-cyan-300 rounded-lg transition-colors">
                    {copiedId === k.id ? <Check className="w-3.5 h-3.5 text-emerald-400" aria-hidden="true" /> : <Copy className="w-3.5 h-3.5" aria-hidden="true" />}
                  </button>
                </div>
                <p className="text-sm text-text mt-2 truncate">{k.name}</p>
                <div className="mt-2 max-w-xs">
                  <div className="flex justify-between text-[11px] text-mute mb-1 tabular-nums">
                    <span>{k.last_used ? t('key.used') : t('key.unused')}</span>
                    <span>{t('key.limit', { n: k.rate_limit })}</span>
                  </div>
                  <div className="credit-track" role="meter" aria-valuenow={k.last_used ? k.rate_limit : 0} aria-valuemax={k.rate_limit} aria-label={t('key.meter')}>
                    <div className="credit-fill" style={{ width: k.last_used ? '100%' : '0%' }} />
                  </div>
                </div>
                <p className="text-xs text-mute mt-2">
                  {t('key.created', { date: tanggalLang(k.created_at, lang) })}
                  {k.last_used && ` · ${t('key.last', { date: tanggalLang(k.last_used, lang, true) })}`}
                  {k.models && t('key.models', { list: k.models })}
                </p>
                {(() => {
                  const u = usage?.keys?.find((x) => x.id === k.id);
                  if (!u) return <p className="text-xs text-mute mt-1">{t('key.noUsage')}</p>;
                  return (
                    <p className="text-xs text-text mt-1 tabular-nums">
                      {t('key.usageLine', { req: fmt(u.requests), in: fmt(u.input_tokens), out: fmt(u.output_tokens) })}
                      <span className="text-mute">{t('key.costLine', { n: fmt(u.est_tokens) })}</span>
                    </p>
                  );
                })()}
              </div>
              <div className="flex shrink-0 items-center gap-2">
                {!k.revoked_at && <Button variant="outline" size="sm" onClick={() => toggleStatus(k.id, !k.is_active)}>{k.is_active ? t('key.disable') : t('key.enable')}</Button>}
                {confirmId === k.id ? (
                  <div className="flex items-center gap-2">
                    <span className="hidden text-xs text-danger sm:block">{k.is_active ? t('key.revokeAsk') : t('key.deleteDisabledAsk')}</span>
                    <Button variant="destructive" size="sm" onClick={() => k.is_active ? revoke(k.id) : deletePermanent(k.id)}>{k.is_active ? t('key.delYes') : t('key.deletePermanent')}</Button>
                    <Button variant="ghost" size="sm" onClick={() => setConfirmId(null)}>{t('mkt.cancel')}</Button>
                  </div>
                ) : <button onClick={() => setConfirmId(k.id)} aria-label={t(k.is_active ? 'key.revoke' : 'key.deletePermanent', { name: k.name })} className="rounded-lg p-2 text-mute transition-colors hover:bg-danger/10 hover:text-danger focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-danger/60"><Trash2 className="h-4 w-4" aria-hidden="true" /></button>}
              </div>
            </li>
          ))}
          </ul>
          {keys.length > 10 && (
            <Pagination page={keysPage.page} hasMore={keysPage.hasMore} onPage={keysPage.setPage} />
          )}
        </>
      )}

      {/* Dialog buat key */}
      <Dialog open={showNew} title={t('key.newTitle')} onClose={() => setShowNew(false)}>
        <div className="space-y-5">
          <div className="space-y-2">
            <label htmlFor="key-name" className="label">{t('key.nameLbl')}</label>
            <Input
              id="key-name"
              placeholder={t('key.namePh')}
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && create()}
              autoFocus
            />
            <p className="text-xs text-mute">{t('key.nameHint')}</p>
          </div>
          <div className="space-y-2">
            <label htmlFor="key-models" className="label">{t('key.modelsLbl')}</label>
            <Input
              id="key-models"
              placeholder={t('key.modelsPh')}
              value={newModels}
              onChange={(e) => setNewModels(e.target.value)}
            />
            <p className="text-xs text-mute">{t('key.modelsHint')}</p>
          </div>
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setShowNew(false)}>{t('mkt.cancel')}</Button>
            <Button onClick={create} disabled={!newName.trim() || creating}>
              {creating ? t('key.creating') : t('key.create')}
            </Button>
          </div>
        </div>
      </Dialog>

      {/* Dialog key mentah: sekali tampil */}
      <Dialog open={!!rawKey} title={t('key.rawTitle')} onClose={() => setRawKey('')}>
        <div className="space-y-4">
          <div className="flex items-start gap-3 bg-gold/5 border border-gold/30 rounded-lg p-3">
            <AlertTriangle className="w-4 h-4 text-gold shrink-0 mt-1" aria-hidden="true" />
            <p className="text-xs text-text leading-relaxed">
              {t('key.rawWarn')}
            </p>
          </div>
          <div className="bg-bg border border-border rounded-lg p-3 font-mono text-sm text-cyan-300 break-all select-all">
            {rawKey}
          </div>
          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={copyRaw}>
              {copied ? <Check className="w-4 h-4" aria-hidden="true" /> : <Copy className="w-4 h-4" aria-hidden="true" />}
              {copied ? t('key.copied') : t('key.copy')}
            </Button>
            <Button onClick={() => setRawKey('')}>{t('key.done')}</Button>
          </div>
        </div>
      </Dialog>
    </div>
  );
}
