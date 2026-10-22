'use client';

// Admin: daftar provider AI — tambah (parity new-api "add channel") + toggle

// aktif/nonaktif. Kunci upstream hanya dikirim saat create; API hanya pernah

// membalikkan masker 4 karakter terakhir.

import { useEffect, useState } from 'react';

import { api } from '@/lib/api';

import { post, patch, pesanError } from '@/lib/http';

import { useToast } from '@/components/ui/toast';
import { useConfirm } from '@/components/ui/confirm';

import { useT } from '@/lib/i18n';

import { Button } from '@/components/ui/button';

import { Input } from '@/components/ui/input';

import { Pencil, Trash2 } from 'lucide-react';

import { Pagination, useClientPagination } from '@/components/ui/Pagination';



type Provider = {

  id: string;

  name: string;

  label: string;

  type: string;

  base_url: string;

  is_active: boolean;

  priority: number;

  has_key: boolean;

  key_mask?: string;

  models?: string;

  last_check_at?: string;

  last_check_status?: string;

  last_latency_ms?: number;

  key_count?: number;

};

type ProviderKey = { id: string; label: string; key_mask: string; is_primary: boolean };



const KOSONG = { name: '', label: '', type: 'openai', base_url: '', api_key: '', priority: 0, models: '' };



// Galat upstream umumnya JSON {"error":{"message":…}}: tampilkan pesannya, bukan

// blob mentah. Kalau /models base_url itu bisa dibaca, sebutkan nama model yang

// sah — base_url + key sudah benar, yang salah cuma nama model.

const tolak = (error?: string, code?: number, model?: string, models?: string[]) => {

  let msg = (error || '').trim();

  try {

    const j = JSON.parse(msg) as { error?: { message?: string }; message?: string };

    msg = j.error?.message || j.message || msg;

  } catch {

    // bukan JSON: pakai apa adanya (mis. "dial tcp … connection refused").

  }

  const sebab = msg || 'Tidak dapat terhubung.';

  const kepala = `${code ? `HTTP ${code} · ` : ''}${sebab}${model ? ` · model ${model}` : ''}`;

  if (models?.length) {

    const contoh = models.slice(0, 8).join(', ');

    return `${kepala} — base_url + kunci bisa dibaca, tapi model itu tidak ada di sana. Model yang tersedia (${models.length}): ${contoh}${models.length > 8 ? ', …' : ''}`;

  }

  return kepala;

};



export default function AdminProvidersPage() {

  const toast = useToast();
  const confirm = useConfirm();

  const { t } = useT();

  const [providers, setProviders] = useState<Provider[]>([]);

  const [loading, setLoading] = useState(true);

  const [err, setErr] = useState('');

  const [toggling, setToggling] = useState<string | null>(null);

  const [form, setForm] = useState({ ...KOSONG });

  const [editId, setEditId] = useState<string | null>(null);

  const [keys, setKeys] = useState<ProviderKey[]>([]);

  const [newKey, setNewKey] = useState({ label: '', api_key: '' });

  const [keyBusy, setKeyBusy] = useState(false);

  const [busy, setBusy] = useState(false);

  const [checking, setChecking] = useState(false);

  const [check, setCheck] = useState<{ ok: boolean; msg: string } | null>(null);

  // Hasil cek per baris: provider tersimpan diuji lewat endpoint health yang

  // sama dengan tombol Check draf, jadi statusnya nyata dari base_url tersimpan.

  const [rowCheck, setRowCheck] = useState<{ id: string; ok: boolean; msg: string } | null>(null);



  const load = async () => {

    setLoading(true);

    try {

      const { data } = await api.get<{ providers: Provider[] }>('/admin/providers');

      setProviders(data.providers || []);

      setErr('');

    } catch {

      setErr(t('ad.pv.loadErr'));
    } finally {

      setLoading(false);

    }

  };



  useEffect(() => { load(); }, []);

  const providersPage = useClientPagination(providers, 10);



  const toggle = async (id: string) => {

    setToggling(id);

    try {

      await api.post('/admin/provider/toggle', { id });

      setProviders((ps) => ps.map((p) => (p.id === id ? { ...p, is_active: !p.is_active } : p)));

    } catch {

      toast.err(t('ad.pv.toggleFail'));

    } finally {

      setToggling(null);

    }

  };



  const cek = async () => {

    setChecking(true); setCheck(null);

    try {

      const { data } = await api.post<{ ok: boolean; status_code?: number; model?: string; model_replaced?: boolean; models?: string[]; error?: string }>('/admin/providers/check', {

        base_url: form.base_url.trim(), api_key: form.api_key.trim(), model: form.models.split(',')[0].trim(),

        // Provider mode responses diuji ke /responses, bukan /chat/completions.

        api_type: form.type === 'responses' ? 'responses' : 'chat_completions',

      });

      if (data.ok && data.model_replaced && data.models?.length) {

        setForm((current) => ({ ...current, models: data.models!.join(', ') }));

      }

      const corrected = data.model_replaced ? t('ad.pv.modelsUpdated') : '';

      setCheck({ ok: data.ok, msg: data.ok ? `${t('ad.pv.checkActive')}${data.status_code ? ` · HTTP ${data.status_code}` : ''}${data.model ? ` · model ${data.model}` : ''}${corrected}` : tolak(data.error, data.status_code, data.model, data.models) });

    } catch (e) { setCheck({ ok: false, msg: pesanError(e, t('ad.pv.checkFail')) }); }

    finally { setChecking(false); }

  };



  const cekBaris = async (p: Provider) => {

    setToggling(`check:${p.id}`);

    setRowCheck(null);

    try {

      const { data } = await api.post<{ ok: boolean; status?: string; latency_ms?: number; status_code?: number; model?: string; error?: string }>(

        `/admin/provider/${p.id}/check`, {});

      const rincian = [data.status, data.latency_ms !== undefined ? `${data.latency_ms} ms` : '', data.status_code ? `HTTP ${data.status_code}` : '']

        .filter(Boolean).join(' · ');

      setRowCheck({ id: p.id, ok: data.ok, msg: data.ok ? `${t('ad.pv.checkActive')}${rincian ? ` · ${rincian}` : ''}` : tolak(data.error, data.status_code, data.model) });

    } catch (e) {

      setRowCheck({ id: p.id, ok: false, msg: pesanError(e, t('ad.pv.checkFail')) });

    } finally {

      setToggling(null);

    }

  };



  const importModels = async (p: Provider) => {

    setToggling(`import:${p.id}`);

    setRowCheck(null);

    try {

      const { data } = await api.post<{ discovered: number; imported: number }>(`/admin/providers/${p.id}/models/import`, {});

      setRowCheck({ id: p.id, ok: true, msg: `Imported ${data.imported} of ${data.discovered} upstream models as inactive.` });

      load();

    } catch (e) {

      setRowCheck({ id: p.id, ok: false, msg: pesanError(e, 'Failed to import upstream models.') });

    } finally {

      setToggling(null);

    }

  };



  const edit = async (p: Provider) => {

    setEditId(p.id);

    setForm({ name: p.name, label: p.label, type: p.type, base_url: p.base_url, api_key: '', priority: p.priority, models: p.models || '' });

    setCheck(null);

    try {

      const { data } = await api.get<{ keys: ProviderKey[] }>(`/admin/providers/${p.id}/keys`);

      setKeys(data.keys || []);

    } catch { setKeys([]); }

    window.scrollTo({ top: 0, behavior: 'smooth' });

  };



  const simpan = async (e: React.FormEvent) => {

    e.preventDefault();

    setBusy(true);

    try {

      const body = {

        name: form.name.trim(),

        label: form.label.trim() || form.name.trim(),

        type: form.type,

        base_url: form.base_url.trim(),

        api_key: form.api_key.trim(),

        priority: Number(form.priority) || 0,

        models: form.models.trim(),

      };

      if (editId) await patch(`/admin/providers/${editId}`, body);

      else await post('/admin/providers', body);

      toast.ok(editId ? t('ad.pv.updated') : t('ad.pv.created'));

      setForm({ ...KOSONG });

      setEditId(null);

      setKeys([]);

      load();

    } catch (e) {

      toast.err(pesanError(e, t('ad.pv.saveFail')));

    } finally {

      setBusy(false);

    }

  };



  return (

    <div className="p-6 md:p-10 max-w-4xl mx-auto">

      <header className="mb-8">

        <p className="label mb-1">{t('ad.common.admin')}</p>

        <h1 className="text-2xl font-semibold tracking-tight">{t('ad.pv.h')}</h1>

      </header>



      {err && <p role="alert" className="mb-6 text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{err}</p>}



      <form onSubmit={simpan} className="card p-5 space-y-4 mb-8">

        <div className="flex items-center justify-between gap-3">

          <h2 className="font-semibold">{editId ? t('ad.pv.editTitle') : t('ad.pv.addTitle')}</h2>
          {editId && <Button type="button" variant="ghost" size="sm" onClick={() => { setEditId(null); setForm({ ...KOSONG }); setKeys([]); }}>{t('ad.pv.cancelEdit')}</Button>}
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">

          <div className="space-y-2">

            <label htmlFor="pv-name" className="label">{t('ad.pv.name')}</label>

            <Input id="pv-name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })}

              placeholder="openai" required maxLength={64} />

          </div>

          <div className="space-y-2">

            <label htmlFor="pv-label" className="label">{t('ad.pv.label')}</label>

            <Input id="pv-label" value={form.label} onChange={(e) => setForm({ ...form, label: e.target.value })}

              placeholder="OpenAI GPT" maxLength={80} />

          </div>

          <div className="space-y-2">

            <label htmlFor="pv-type" className="label">{t('ad.pv.type')}</label>

            <select id="pv-type" value={form.type} onChange={(e) => { setForm({ ...form, type: e.target.value }); setCheck(null); }}

              className="h-10 w-full rounded-xl border border-slate-500 bg-surface-hi px-3 text-sm text-white focus:border-cyan-400 focus:outline-none">

              <option value="openai">openai (OpenAI-compatible)</option>

              <option value="deepseek">deepseek</option>

              <option value="openrouter">openrouter</option>

              <option value="responses">responses (OpenAI Responses API)</option>

            </select>

          </div>

          <div className="space-y-2">

            <label htmlFor="pv-prio" className="label">{t('ad.pv.priority')}</label>

            <Input id="pv-prio" type="number" value={form.priority}

              onChange={(e) => setForm({ ...form, priority: Number(e.target.value) })} />

          </div>

          <div className="space-y-2 sm:col-span-2">

            <label htmlFor="pv-url" className="label">{t('ad.pv.baseUrl')}</label>

            <Input id="pv-url" value={form.base_url} onChange={(e) => { setForm({ ...form, base_url: e.target.value }); setCheck(null); }}

              placeholder="https://api.openai.com/v1" required />

          </div>

          <div className="space-y-2 sm:col-span-2">

            <label htmlFor="pv-key" className="label">{t('ad.pv.apiKey')}</label>

            <div className="flex gap-2">

              <Input id="pv-key" type="password" autoComplete="off" value={form.api_key}

                onChange={(e) => { setForm({ ...form, api_key: e.target.value }); setCheck(null); }} placeholder={editId ? t('ad.pv.keyKeep') : 'sk-…'} required={!editId} />

              <Button type="button" variant="outline" disabled={checking || !form.base_url.trim() || !form.api_key.trim()} onClick={cek}>

                {checking ? t('ad.common.checking') : t('ad.pv.check')}

              </Button>

            </div>

            {check && <p role={check.ok ? 'status' : 'alert'} className={`text-xs ${check.ok ? 'text-cyan-200' : 'text-danger'}`}>{check.msg}</p>}

            <p className="text-xs text-mute">{t('ad.pv.keyNote')}</p>

          </div>

          <div className="space-y-2 sm:col-span-2">

            <label htmlFor="pv-models" className="label">{t('ad.pv.models')}</label>

            <Input id="pv-models" value={form.models} onChange={(e) => { setForm({ ...form, models: e.target.value }); setCheck(null); }}

              placeholder="gpt-4o, gpt-4o-mini" />

          </div>

        </div>

        {editId && (

          <div className="rounded-xl border border-border bg-surface-hi/40 p-4 space-y-3">

            <div>

              <p className="font-medium text-sm">{t('ad.pv.poolH')}</p>

              <p className="text-xs text-mute mt-1">{t('ad.pv.poolP')}</p>

            </div>

            <ul className="space-y-2">

              {keys.map((key) => <li key={key.id} className="flex items-center justify-between gap-3 rounded-lg border border-border px-3 py-2 text-xs">

                <span>{key.label || t('ad.pv.noLabel')} · <span className="key-mask">{key.key_mask}</span>{key.is_primary ? ` · ${t('ad.pv.primary')}` : ''}</span>

                {!key.is_primary && <button type="button" aria-label={t('ad.pv.delKeyAria', { name: key.label || t('ad.pv.noLabel') })} className="p-2 text-mute hover:text-danger" onClick={async () => {
                  if (!(await confirm({ title: t('ad.common.deleteOf', { name: key.label || t('ad.pv.noLabel') }), body: 'This provider key will be permanently deleted.', confirmLabel: t('ad.common.delete') }))) return;
                  try { await api.del(`/admin/providers/${editId}/keys/${key.id}`); setKeys((items) => items.filter((item) => item.id !== key.id)); toast.ok(t('ad.pv.keyDeleted')); }

                  catch (e) { toast.err(pesanError(e, t('ad.pv.keyDelFail'))); }

                }}><Trash2 className="w-4 h-4" aria-hidden="true" /></button>}

              </li>)}

            </ul>

            <div className="grid gap-2 sm:grid-cols-[.7fr_1.3fr_auto]">

              <Input value={newKey.label} onChange={(e) => setNewKey({ ...newKey, label: e.target.value })} placeholder={t('ad.pv.keyLabelPh')} />

              <Input type="password" autoComplete="off" value={newKey.api_key} onChange={(e) => setNewKey({ ...newKey, api_key: e.target.value })} placeholder={t('ad.pv.keyAddPh')} />

              <Button type="button" variant="outline" disabled={keyBusy || !newKey.api_key.trim()} onClick={async () => {

                setKeyBusy(true);

                try {

                  const { data } = await post<{ id: string; key_mask: string; model: string }>(`/admin/providers/${editId}/keys`, newKey);

                  setKeys((items) => [...items, { id: data.id, label: newKey.label, key_mask: data.key_mask, is_primary: false }]);

                  setNewKey({ label: '', api_key: '' });

                  toast.ok(t('ad.pv.keyValid', { model: data.model }));

                  load();

                } catch (e) { toast.err(pesanError(e, t('ad.pv.keyRejected'))); }

                finally { setKeyBusy(false); }

              }}>{keyBusy ? t('ad.common.checking') : t('ad.pv.keyAdd')}</Button>

            </div>

          </div>

        )}

        <Button type="submit" disabled={busy}>{busy ? t('ad.common.saving') : editId ? t('ad.pv.saveEdit') : t('ad.pv.add')}</Button>

      </form>



      {loading ? (

        <div className="space-y-3">

          {[0, 1, 2].map((i) => <div key={i} className="card p-4 h-20"><div className="skel h-4 w-48 mb-2" /><div className="skel h-3 w-72" /></div>)}

        </div>

      ) : (

        <ul className="tbl-wrap zebra">

          {providersPage.items.map((p) => (

            <li key={p.id} className="p-4 flex items-center justify-between gap-4">

              <div className="min-w-0">

                <div className="flex items-center gap-3 flex-wrap">

                  <span className="text-sm font-medium text-text">{p.label || p.name}</span>

                  <span className={`badge ${p.is_active ? 'badge-on' : 'badge-off'}`}>{p.is_active ? t('ad.common.active') : t('ad.common.inactive')}</span>

                  <span className="badge badge-off">{t('ad.pv.priorityBadge', { n: p.priority })}</span>

                  {p.key_mask && <span className="key-mask">{p.key_mask}</span>}

                  <span className="badge badge-off">{t('ad.pv.keyCount', { n: p.key_count || 1 })}</span>

                </div>

                <p className="text-xs text-mute mt-1 truncate font-mono">{p.type} &middot; {p.base_url}</p>

                {p.last_check_status && p.last_latency_ms !== undefined && <p className="text-xs text-mute mt-1">{t('ad.pv.lastCheck', { status: p.last_check_status, ms: p.last_latency_ms })}</p>}

              </div>

              <div className="act-group">

                <Button variant="ghost" size="sm" onClick={() => edit(p)} aria-label={t('ad.common.editOf', { name: p.label || p.name })}>

                  <Pencil className="w-4 h-4" aria-hidden="true" /> {t('ad.common.edit')}

                </Button>

                <Button

                  variant="outline"

                  size="sm"

                  disabled={toggling === `import:${p.id}`}

                  onClick={() => importModels(p)}

                >

                  {toggling === `import:${p.id}` ? 'Importing…' : 'Import models'}

                </Button>

                <Button

                  variant="outline"

                  size="sm"

                  disabled={toggling === `check:${p.id}`}

                  onClick={() => cekBaris(p)}

                >

                  {toggling === `check:${p.id}` ? t('ad.common.checking') : t('ad.pv.check')}

                </Button>

                <Button

                  variant={p.is_active ? 'ghost' : 'default'}

                  size="sm"

                  disabled={toggling === p.id}

                  onClick={() => toggle(p.id)}

                >

                  {toggling === p.id ? t('ad.common.processing') : p.is_active ? t('ad.common.disable') : t('ad.common.enable')}

                </Button>

              </div>

              {rowCheck?.id === p.id && (

                <p role={rowCheck.ok ? 'status' : 'alert'}

                  className={`basis-full text-xs ${rowCheck.ok ? 'text-cyan-200' : 'text-danger'}`}>

                  {rowCheck.msg}

                </p>

              )}

            </li>

          ))}

          {providers.length === 0 && !err && (

            <li className="card p-8 text-center text-sm text-mute">{t('ad.pv.empty')}</li>

          )}

        </ul>

      )}

      {providers.length > 10 && <Pagination page={providersPage.page} hasMore={providersPage.hasMore} onPage={providersPage.setPage} />}

    </div>

  );

}

