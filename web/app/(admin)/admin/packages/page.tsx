'use client';
// Admin · Paket Token: CRUD paket daily/monthly + katalog harga.
import { useState } from 'react';
import { api } from '@/lib/api';
import { patch, post, pesanError } from '@/lib/http';
import { useFetch } from '@/lib/useApi';
import { useToast } from '@/components/ui/toast';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Badge } from '@/components/ui/badge';
import { Dialog } from '@/components/ui/dialog';
import { useConfirm } from '@/components/ui/confirm';
import { Pagination, useClientPagination } from '@/components/ui/Pagination';
import { rupiah, asList, angka } from '@/lib/utils';
import { useT } from '@/lib/i18n';
import { Box, Pencil, Trash2, Plus } from 'lucide-react';

type Paket = {
  id: string; name: string; token_count: number; price: number;
  duration_days: number; kind: string; list_price?: number | null;
  promo_price?: number | null; effective_days?: number | null;
  is_active?: boolean; sort_order?: number;
};

const BUKA: Paket = {
  id: '', name: '', token_count: 0, price: 0, duration_days: 0, kind: 'monthly',
  is_active: true, sort_order: 0,
};

export default function AdminPackagesPage() {
  const { data, loading, err, reload } = useFetch<unknown>('/admin/packages');
  const { t } = useT();
  const [form, setForm] = useState<Partial<Paket>>({});
  const [buka, setBuka] = useState(false);
  const [busy, setBusy] = useState(false);
  const [hapusId, setHapusId] = useState<string | null>(null);
  const confirm = useConfirm();
  const toast = useToast();

  const daftar = asList<Paket>(data, 'packages', 'items');
  const daily = daftar.filter((p) => p.kind === 'daily');
  const monthly = daftar.filter((p) => p.kind === 'monthly');

  const simpan = async () => {
    if (!form.name || !form.kind) {
      return toast.err(t('ad.pk.nameKindRequired'));
    }
    setBusy(true);
    try {
      if (form.id) {
        await patch(`/admin/packages/${form.id}`, form);
        toast.ok(t('ad.pk.updated'));
      } else {
        await post('/admin/packages', form);
        toast.ok(t('ad.pk.created'));
      }
      setBuka(false);
      setForm({});
      await reload();
    } catch (e) {
      toast.err(pesanError(e, t('ad.pk.saveFail')));
    } finally {
      setBusy(false);
    }
  };

  const hapus = async (p: Paket) => {
    if (!(await confirm({ title: t('ad.common.deleteOf', { name: p.name }), confirmLabel: t('ad.common.delete') }))) return;
    setHapusId(p.id);
    try {
      await api.del(`/admin/packages/${p.id}`);
      toast.ok(t('ad.pk.deleted'));
      await reload();
    } catch (e) {
      toast.err(pesanError(e, t('ad.pk.delFail')));
    } finally {
      setHapusId(null);
    }
  };

  const edit = (p: Paket) => {
    setForm(p);
    setBuka(true);
  };
  const baru = () => {
    setForm({ ...BUKA });
    setBuka(true);
  };

  const aksi = { onEdit: edit, onHapus: hapus, hapusId };

  return (
    <div className="p-6 md:p-10 max-w-6xl mx-auto">
      <header className="mb-8 flex items-start justify-between gap-4">
        <div>
          <p className="label mb-1">{t('ad.common.admin')}</p>
          <h1 className="text-2xl font-semibold tracking-tight">{t('ad.pk.h')}</h1>
          <p className="text-sm text-mute mt-1">{t('ad.pk.sub')}</p>
        </div>
        <Button onClick={baru}><Plus className="w-4 h-4" aria-hidden="true" /> {t('ad.pk.new')}</Button>
      </header>

      {err && <p role="alert" className="mb-6 text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{err}</p>}

      {err ? null : loading ? (
        <div className="space-y-3">{[0, 1, 2].map((i) => <div key={i} className="card p-4 h-16"><div className="skel h-4 w-56" /></div>)}</div>
      ) : daftar.length === 0 ? (
        <div className="card p-10 text-center">
          <Box className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />
          <p className="font-medium">{t('ad.pk.empty')}</p>
        </div>
      ) : (
        <>
          {daily.length > 0 && <Kelompok jenis="daily" label={t('ad.pk.daily')} data={daily} {...aksi} />}
          {monthly.length > 0 && <Kelompok jenis="monthly" label={t('ad.pk.monthly')} data={monthly} {...aksi} />}
        </>
      )}

      <Dialog open={buka} title={form.id ? t('ad.pk.editTitle') : t('ad.pk.newTitle')} onClose={() => setBuka(false)}>
        <div className="grid gap-4">
          <div className="space-y-2">
            <label htmlFor="pk-name" className="label">{t('ad.pk.name')}</label>
            <Input id="pk-name" value={form.name ?? ''}
              onChange={(e) => setForm({ ...form, name: e.target.value })} />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label htmlFor="pk-tokens" className="label">{t('ad.pk.tokens')}</label>
              <Input id="pk-tokens" type="number" value={form.token_count ?? ''}
                onChange={(e) => setForm({ ...form, token_count: Number(e.target.value) })} />
            </div>
            <div className="space-y-2">
              <label htmlFor="pk-price" className="label">{t('ad.pk.price')}</label>
              <Input id="pk-price" type="number" value={form.price ?? ''}
                onChange={(e) => setForm({ ...form, price: Number(e.target.value) })} />
            </div>
            <div className="space-y-2">
              <label htmlFor="pk-days" className="label">{t('ad.pk.days')}</label>
              <Input id="pk-days" type="number" value={form.duration_days ?? ''}
                onChange={(e) => setForm({ ...form, duration_days: Number(e.target.value) })} />
            </div>
            <div className="space-y-2">
              <label htmlFor="pk-kind" className="label">{t('ad.pk.kind')}</label>
              <Select id="pk-kind" value={form.kind ?? 'monthly'}
                onChange={(e) => setForm({ ...form, kind: e.target.value })}>
                <option value="daily">{t('ad.pk.daily')}</option>
                <option value="monthly">{t('ad.pk.monthly')}</option>
              </Select>
            </div>
            <div className="space-y-2">
              <label htmlFor="pk-eff" className="label">{t('ad.pk.effDays')}</label>
              <Input id="pk-eff" type="number" value={form.effective_days ?? ''}
                onChange={(e) => setForm({ ...form, effective_days: e.target.value ? Number(e.target.value) : null })} />
            </div>
            <div className="space-y-2">
              <label htmlFor="pk-list" className="label">{t('ad.pk.listPrice')}</label>
              <Input id="pk-list" type="number" value={form.list_price ?? ''}
                onChange={(e) => setForm({ ...form, list_price: e.target.value ? Number(e.target.value) : null })} />
            </div>
          </div>
          <label className="flex items-center gap-3 text-sm">
            <input type="checkbox" checked={form.is_active ?? true}
              onChange={(e) => setForm({ ...form, is_active: e.target.checked })}
              className="h-4 w-4 rounded accent-cyan-500" />
            {t('ad.common.active')}
          </label>
        </div>
        <div className="flex justify-end gap-3 mt-6">
          <Button variant="ghost" onClick={() => setBuka(false)} disabled={busy}>{t('ad.common.cancel')}</Button>
          <Button onClick={simpan} disabled={busy}>
            {busy ? t('ad.common.saving') : form.id ? t('ad.common.update') : t('ad.common.create')}
          </Button>
        </div>
      </Dialog>
    </div>
  );
}

type Aksi = {
  onEdit: (p: Paket) => void;
  onHapus: (p: Paket) => void;
  hapusId: string | null;
};

// Tiap kelompok dipaginasi sendiri (10/halaman) supaya tiap tabel punya kontrolnya sendiri.
function Kelompok({ jenis, label, data, onEdit, onHapus, hapusId }: Aksi & { jenis: string; label: string; data: Paket[] }) {
  const { t } = useT();
  const page = useClientPagination(data, 10);
  const id = `pkg-${jenis}`;
  return (
    <section className="mb-10" aria-labelledby={id}>
      <h2 id={id} className="text-lg font-semibold mb-3">{label}</h2>
      <div className="tbl-wrap zebra">
        <table className="w-full text-sm">
          <thead><tr>
            <th scope="col" className="text-left py-2">{t('ad.pk.name')}</th>
            <th scope="col" className="text-right py-2">{t('ad.pk.tokens')}</th>
            <th scope="col" className="text-right py-2">{t('ad.pk.priceShort')}</th>
            <th scope="col" className="text-right py-2">{t('ad.pk.daysShort')}</th>
            <th scope="col" className="text-center py-2">{t('ad.common.status')}</th>
            <th scope="col" className="text-center py-2">{t('ad.common.actions')}</th>
          </tr></thead>
          <tbody>
            {page.items.map((p) => (
              <tr key={p.id} className="border-t border-border">
                <td className="py-2">{p.name}</td>
                <td className="text-right tabular-nums py-2">{angka(p.token_count)}</td>
                <td className="text-right tabular-nums py-2">
                  {p.list_price && p.list_price > p.price ? (
                    <>
                      <span className="line-through text-mute">{rupiah(p.list_price)}</span>
                      {' '}{rupiah(p.price)}
                    </>
                  ) : rupiah(p.price)}
                </td>
                <td className="text-right tabular-nums py-2">{p.effective_days ?? p.duration_days}</td>
                <td className="text-center py-2">
                  <Badge tone={p.is_active ? 'ok' : 'danger'}>
                    {p.is_active ? t('ad.common.active') : t('ad.common.inactive')}
                  </Badge>
                </td>
                <td className="py-2">
                  <div className="flex items-center justify-center gap-1">
                    <button onClick={() => onEdit(p)} aria-label={t('ad.common.editOf', { name: p.name })}
                      className="p-2 text-mute hover:text-cyan-300 hover:bg-surface-hi rounded-lg transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60">
                      <Pencil className="w-4 h-4" aria-hidden="true" />
                    </button>
                    <button onClick={() => onHapus(p)} disabled={hapusId === p.id}
                      aria-label={t('ad.common.deleteOf', { name: p.name })}
                      className="p-2 text-mute hover:text-danger hover:bg-danger/10 rounded-lg transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-danger/60">
                      {hapusId === p.id ? '…' : <Trash2 className="w-4 h-4" aria-hidden="true" />}
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {data.length > 10 && <Pagination page={page.page} hasMore={page.hasMore} onPage={page.setPage} />}
    </section>
  );
}
