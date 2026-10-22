'use client';

// Admin models: katalog publik, routing provider/upstream, multiplier token, dan status.

import { useState } from 'react';

import { post, patch, pesanError } from '@/lib/http';

import { useFetch } from '@/lib/useApi';

import { api } from '@/lib/api';

import { Button } from '@/components/ui/button';

import { Input } from '@/components/ui/input';

import { Select } from '@/components/ui/select';

import { Badge } from '@/components/ui/badge';

import { Dialog } from '@/components/ui/dialog';

import { useToast } from '@/components/ui/toast';
import { useConfirm } from '@/components/ui/confirm';

import { useT } from '@/lib/i18n';

import { angka, asList } from '@/lib/utils';

import { Plus, Pencil, Trash2, PlugZap } from 'lucide-react';

import { Pagination, useClientPagination } from '@/components/ui/Pagination';

type Mdl = {

  id: string;

  name: string;

  type?: string;

  context_length?: number;

  max_output_tokens?: number;

  is_active?: boolean;

  tier?: 'FREE' | 'PAID';

  token_multiplier?: number;

  family?: string;

  provider_id?: string;

  provider_name?: string;

  upstream_model?: string;

};

type Provider = { id: string; name: string; label?: string; is_active?: boolean; models?: string };

const TYPE_OPTS = ['TEXT', 'IMAGE', 'VIDEO', 'AUDIO', 'EMBEDDING', 'VISION', 'RERANK', 'REASONING'];

const KOSONG = {
  name: '', type: 'TEXT', tier: 'PAID', token_multiplier: '1',
  context_length: '', max_output_tokens: '', provider_id: '', upstream_model: '', family: '',
};

export default function AdminModelsPage() {

  const { data, loading, err, reload } = useFetch<unknown>('/admin/models');

  const { data: dProv } = useFetch<unknown>('/admin/providers');

  const models = asList<Mdl>(data, 'models', 'items');

  const providers = asList<Provider>(dProv, 'providers', 'items');

  const modelsPage = useClientPagination(models, 10);

  const [form, setForm] = useState({ ...KOSONG });

  const [editId, setEditId] = useState<string | null>(null);

  const [open, setOpen] = useState(false);

  const [busy, setBusy] = useState(false);

  const [checking, setChecking] = useState<string | null>(null);

  const [hasil, setHasil] = useState<Record<string, { ok: boolean; latency_ms?: number }>>({});

  const toast = useToast();
  const confirm = useConfirm();

  const { t } = useT();

  const buka = (m?: Mdl) => {

    setEditId(m?.id ?? null);

    setForm(m ? {

      name: m.name,

      type: m.type ? m.type.split(',').filter(Boolean).join(',') : 'TEXT',

      tier: m.tier || 'PAID',

      token_multiplier: m.token_multiplier != null ? String(m.token_multiplier) : '1',

      context_length: m.context_length != null ? String(m.context_length) : '',

      max_output_tokens: m.max_output_tokens != null ? String(m.max_output_tokens) : '',

      provider_id: m.provider_id || '',

      upstream_model: m.upstream_model || '',

      family: m.family || '',

    } : { ...KOSONG });

    setOpen(true);

  };

  const simpan = async () => {

    if (!form.name.trim()) return;

    const multiplier = Number(form.token_multiplier);

    if (!Number.isFinite(multiplier) || multiplier < 0 || multiplier > 1000) {

      toast.err(t('ad.md.multiplierErr'));

      return;

    }

    const typeCount = form.type.split(',').filter(Boolean).length;
    if (typeCount < 1 || typeCount > 3) {
      toast.err(t('ad.md.typeCountErr'));
      return;
    }
    if (!!form.provider_id !== !!form.upstream_model.trim()) {

      toast.err(t('ad.md.pairErr'));

      return;

    }

    setBusy(true);

    try {

      const body = {

        name: form.name.trim(),

        type: form.type,

        tier: form.tier,

        token_multiplier: multiplier,

        context_length: form.context_length ? Number(form.context_length) : undefined,

        max_output_tokens: form.max_output_tokens ? Number(form.max_output_tokens) : undefined,

        provider_id: form.provider_id,

        upstream_model: form.upstream_model.trim(),

        family: form.family || '',

      };

      if (editId) await patch(`/admin/models/${editId}`, body);

      else await post('/admin/models', body);

      toast.ok(editId ? t('ad.md.updated') : t('ad.md.created'));

      setOpen(false);

      reload();

    } catch (e) {

      toast.err(pesanError(e, t('ad.md.saveFail')));

    } finally {

      setBusy(false);

    }

  };

  const toggle = async (m: Mdl) => {

    try {

      await patch(`/admin/models/${m.id}`, { is_active: m.is_active === false });

      reload();

    } catch (e) {

      toast.err(pesanError(e, t('ad.md.toggleFail')));

    }

  };

  const hapus = async (m: Mdl) => {
    if (!(await confirm({ title: t('ad.common.deleteOf', { name: m.name }), body: `${m.name} will be permanently deleted.`, confirmLabel: t('ad.common.delete') }))) return;
    try {

      await api.del(`/admin/models/${m.id}`);

      toast.ok(t('ad.md.deleted', { name: m.name }));

      reload();

    } catch (e) {

      toast.err(pesanError(e, t('ad.md.delFail')));

    }

  };

  const check = async (p: Provider) => {

    setChecking(p.id);

    try {

      const { data: res } = await post<{ ok: boolean; latency_ms?: number }>(`/admin/provider/${p.id}/check`, {});

      setHasil((h) => ({ ...h, [p.id]: { ok: !!res?.ok, latency_ms: res?.latency_ms } }));

    } catch (e) {

      setHasil((h) => ({ ...h, [p.id]: { ok: false } }));

      toast.err(pesanError(e, t('ad.md.checkFail')));

    } finally {

      setChecking(null);

    }

  };

  return (

    <div className="p-6 md:p-10 max-w-6xl mx-auto">

      <div className="flex items-start justify-between gap-4 mb-8">

        <div>

          <p className="label mb-1">{t('ad.common.admin')}</p>

          <h1 className="text-2xl font-semibold tracking-tight">{t('ad.md.h')}</h1>

        </div>

        <Button onClick={() => buka()}>

          <Plus className="w-4 h-4" aria-hidden="true" /> {t('ad.md.add')}

        </Button>

      </div>

      {err && <p role="alert" className="mb-6 text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{err}</p>}

      {/* Check koneksi per provider */}

      {providers.length > 0 && (

        <section aria-label={t('ad.md.checkAria')} className="card p-4 mb-6">

          <p className="label mb-3">{t('ad.md.checkH')}</p>

          <ul className="flex flex-wrap gap-2">

            {providers.map((p) => {

              const h = hasil[p.id];

              return (

                <li key={p.id} className="flex items-center gap-2 bg-surface-hi border border-border rounded-lg px-3 py-2">

                  <span className="text-sm">{p.label || p.name}</span>

                  {h && <Badge tone={h.ok ? 'ok' : 'danger'}>{h.ok ? `OK ${h.latency_ms ?? '-'}ms` : t('ad.md.fail')}</Badge>}

                  <Button variant="ghost" size="sm" disabled={checking === p.id} onClick={() => check(p)}>

                    <PlugZap className="w-3.5 h-3.5" aria-hidden="true" /> {checking === p.id ? '…' : t('ad.md.check')}

                  </Button>

                </li>

              );

            })}

          </ul>

        </section>

      )}

      {loading ? (

        <div className="skel h-56" />

      ) : models.length === 0 ? (

        <p className="card p-8 text-center text-sm text-mute">{t('ad.md.empty')}</p>

      ) : (

        <div className="card overflow-x-auto">

          <table className="w-full text-sm">

            <thead>

              <tr className="text-left text-mute border-b border-border">

                <th scope="col" className="px-4 py-3 font-medium">{t('ad.md.thModel')}</th>

                <th scope="col" className="px-4 py-3 font-medium">{t('ad.md.thTier')}</th>

                <th scope="col" className="px-4 py-3 font-medium">{t('ad.md.thType')}</th>

                <th scope="col" className="px-4 py-3 font-medium">{t('ad.md.thRouting')}</th>

                <th scope="col" className="px-4 py-3 font-medium text-right">{t('ad.md.thCtx')}</th>

                <th scope="col" className="px-4 py-3 font-medium text-right hidden sm:table-cell">{t('ad.md.thMaxOut')}</th>

                <th scope="col" className="px-4 py-3 font-medium">{t('ad.md.thStatus')}</th>

                <th scope="col" className="px-4 py-3 font-medium text-right">{t('ad.md.thActions')}</th>

              </tr>

            </thead>

            <tbody>

              {modelsPage.items.map((m) => {

                const aktif = m.is_active !== false;

                return (

                  <tr key={m.id} className="border-b border-border/60 last:border-0">

                    <td className="px-4 py-3 font-mono text-cyan-300 whitespace-nowrap">{m.name}</td>

                    <td className="px-4 py-3 whitespace-nowrap">

                      {m.tier

                        ? <Badge tone={m.tier.toUpperCase() === 'FREE' ? 'netral' : 'ok'}>{m.tier.toUpperCase() === 'FREE' ? t('ad.md.optFree') : t('ad.md.optPaid')}</Badge>

                        : <span className="text-mute">-</span>}

                      {Number(m.token_multiplier) > 0 && (

                        <span className="ml-2 font-mono text-xs text-mute">×{m.token_multiplier}</span>

                      )}

                    </td>

                    <td className="px-4 py-3 text-mute">{m.type || 'TEXT'}</td>

                    <td className="px-4 py-3 min-w-48">

                      {m.provider_name && m.upstream_model ? (

                        <div className="text-xs"><span>{m.provider_name}</span><span className="text-mute"> → </span><code className="text-cyan-300">{m.upstream_model}</code></div>

                      ) : <span className="text-xs text-mute">{t('ad.md.legacy')}</span>}

                    </td>

                    <td className="px-4 py-3 text-right tabular-nums">{m.context_length ? angka(m.context_length) : '-'}</td>

                    <td className="px-4 py-3 text-right tabular-nums hidden sm:table-cell">{m.max_output_tokens ? angka(m.max_output_tokens) : '-'}</td>

                    <td className="px-4 py-3"><Badge>{aktif ? t('ad.common.active') : t('ad.common.inactive')}</Badge></td>

                    <td className="px-4 py-3">

                      <div className="flex items-center justify-end gap-1">

                        <Button variant="ghost" size="sm" onClick={() => toggle(m)}>{aktif ? t('ad.md.btnDisable') : t('ad.md.btnEnable')}</Button>

                        <button onClick={() => buka(m)} aria-label={t('ad.common.editOf', { name: m.name })}

                          className="p-2 text-mute hover:text-cyan-300 hover:bg-surface-hi rounded-lg transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60">

                          <Pencil className="w-4 h-4" aria-hidden="true" />

                        </button>

                        <button onClick={() => hapus(m)} aria-label={t('ad.common.deleteOf', { name: m.name })}

                          className="p-2 text-mute hover:text-danger hover:bg-danger/10 rounded-lg transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-danger/60">

                          <Trash2 className="w-4 h-4" aria-hidden="true" />

                        </button>

                      </div>

                    </td>

                  </tr>

                );

              })}

            </tbody>

          </table>

        </div>

      )}

      {models.length > 10 && <Pagination page={modelsPage.page} hasMore={modelsPage.hasMore} onPage={modelsPage.setPage} />}

      <Dialog open={open} title={editId ? t('ad.md.editTitle') : t('ad.md.addTitle')} onClose={() => setOpen(false)}>

        <div className="space-y-4">

          <div className="space-y-2">

            <label htmlFor="m-name" className="label">{t('ad.md.lblName')}</label>

            <Input id="m-name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder={t('ad.md.namePh')} className="font-mono" />

          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">

            <div className="space-y-2">
              <label className="label">{t('ad.md.lblType')} (pilih 1–3)</label>
              <div className="flex flex-wrap gap-2">
                {TYPE_OPTS.map((t) => {
                  const sel = form.type.split(',').filter(Boolean);
                  const cek = sel.includes(t);
                  return (
                    <label key={t} className="inline-flex items-center gap-2 text-sm">
                      <input
                        type="checkbox"
                        className="rounded border-border text-cyan-500 focus:ring-cyan-400/60"
                        checked={cek}
                        disabled={sel.length >= 3 && !cek}
                        onChange={(e) => {
                          const next = e.target.checked ? [...sel, t] : sel.filter((x) => x !== t);
                          setForm({ ...form, type: next.join(',') || 'TEXT' });
                        }}
                      />
                      {t}
                    </label>
                  );
                })}
              </div>
            </div>

            <div className="space-y-2">

              <label htmlFor="m-tier" className="label">{t('ad.md.lblTier')}</label>

              <Select id="m-tier" value={form.tier} onChange={(e) => setForm({ ...form, tier: e.target.value })}>

                <option value="PAID">{t('ad.md.optPaid')}</option>

                <option value="FREE">{t('ad.md.optFree')}</option>

              </Select>

            </div>

            <div className="space-y-2">

              <label htmlFor="m-multiplier" className="label">{t('ad.md.lblMultiplier')}</label>

              <Input id="m-multiplier" inputMode="decimal" value={form.token_multiplier}

                onChange={(e) => setForm({ ...form, token_multiplier: e.target.value })} placeholder="1" />

            </div>

          </div>

          <div className="space-y-2">

            <label htmlFor="m-family" className="label">{t('ad.md.family')}</label>

            <Select id="m-family" value={form.family} onChange={(e) => setForm({ ...form, family: e.target.value })}>

              <option value="">{t('ad.md.autoDetect')}</option>

              <option value="anthropic">{t('ad.md.optAnthropic')}</option>

              <option value="openai">{t('ad.md.optOpenai')}</option>

              <option value="google">{t('ad.md.optGoogle')}</option>

              <option value="deepseek">{t('ad.md.optDeepseek')}</option>

              <option value="xai">{t('ad.md.optXai')}</option>

              <option value="meta">{t('ad.md.optMeta')}</option>

              <option value="mistralai">{t('ad.md.optMistral')}</option>

              <option value="moonshot">{t('ad.md.optMoonshot')}</option>

              <option value="alibaba">{t('ad.md.optAlibaba')}</option>

              <option value="other">{t('ad.md.optOther')}</option>

            </Select>

            <p className="text-xs text-mute">{t('ad.md.familyHint')}</p>

          </div>

          <div className="grid grid-cols-2 gap-3">

            <div className="space-y-2">

              <label htmlFor="m-ctx" className="label">{t('ad.md.ctxLen')}</label>

              <Input id="m-ctx" inputMode="numeric" value={form.context_length} onChange={(e) => setForm({ ...form, context_length: e.target.value })} placeholder="128000" />

            </div>

            <div className="space-y-2">

              <label htmlFor="m-max" className="label">{t('ad.md.lblMaxOut')}</label>

              <Input id="m-max" inputMode="numeric" value={form.max_output_tokens} onChange={(e) => setForm({ ...form, max_output_tokens: e.target.value })} placeholder="4096" />

            </div>

          </div>

          <div className="rounded-xl border border-border bg-surface-hi/50 p-4 space-y-3">

            <div>

              <p className="label">{t('ad.md.lblRouting')}</p>

              <p className="mt-1 text-xs text-mute">{t('ad.md.routingP')}</p>

            </div>

            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">

              <div className="space-y-2">

                <label htmlFor="m-provider" className="label">{t('ad.md.lblProvider')}</label>

                <Select id="m-provider" value={form.provider_id} onChange={(e) => setForm({ ...form, provider_id: e.target.value, upstream_model: '' })}>

                  <option value="">{t('ad.md.legacy')}</option>

                  {providers.filter((p) => p.is_active !== false).map((p) => <option key={p.id} value={p.id}>{p.label || p.name}</option>)}

                </Select>

              </div>

              <div className="space-y-2">

                <label htmlFor="m-upstream" className="label">{t('ad.md.lblUpstream')}</label>

                <Input id="m-upstream" list="upstream-model-options" value={form.upstream_model}

                  disabled={!form.provider_id} onChange={(e) => setForm({ ...form, upstream_model: e.target.value })}

                  placeholder={form.provider_id ? t('ad.md.upstreamPh') : t('ad.md.pickProvider')} className="font-mono" />

                <datalist id="upstream-model-options">

                  {(providers.find((p) => p.id === form.provider_id)?.models || '').split(',').map((m) => m.trim()).filter(Boolean).map((m) => <option key={m} value={m} />)}

                </datalist>

              </div>

            </div>

          </div>

          <div className="flex justify-end gap-2 pt-1">

            <Button variant="ghost" onClick={() => setOpen(false)}>{t('ad.common.cancel')}</Button>

            <Button onClick={simpan} disabled={!form.name.trim() || busy}>{busy ? t('ad.common.saving') : t('ad.common.save')}</Button>

          </div>

        </div>

      </Dialog>

    </div>

  );

}

