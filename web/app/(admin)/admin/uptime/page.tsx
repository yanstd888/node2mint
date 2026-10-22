'use client';

// Admin · Uptime: daftar monitor + form tambah monitor.

import { useState } from 'react';

import { post, pesanError } from '@/lib/http';

import { useFetch } from '@/lib/useApi';

import { useToast } from '@/components/ui/toast';

import { useT } from '@/lib/i18n';

import { Button } from '@/components/ui/button';

import { Badge } from '@/components/ui/badge';

import { Input } from '@/components/ui/input';

import { tanggal, asList } from '@/lib/utils';

import { Activity } from 'lucide-react';



type Monitor = { id: string; name?: string; url?: string; interval_sec?: number; is_active?: boolean; checked_at?: string; last_checked?: string; uptime_pct?: number; status?: string };

const statusLabel = (status: string | undefined, active: boolean | undefined, t: ReturnType<typeof useT>['t']) => {
  switch ((status ?? '').toUpperCase()) {
    case 'UP':
    case 'OK':
    case 'HEALTHY':
    case 'OPERATIONAL': return t('st.ok');
    case 'DEGRADED':
    case 'LIMITED': return t('st.slow');
    case 'DOWN':
    case 'OUTAGE': return t('st.down');
    case 'ERROR': return t('st.error');
    default: return active === false ? t('ad.common.inactive') : t('ad.common.active');
  }
};



export default function AdminUptimePage() {

  const { data, loading, err, reload } = useFetch<unknown>('/admin/uptime');

  const toast = useToast();

  const { t } = useT();

  const [form, setForm] = useState({ name: '', url: '', interval_sec: '300' });

  const [busy, setBusy] = useState(false);



  const monitors = asList<Monitor>(data, 'monitors');



  const tambah = async (e: React.FormEvent) => {

    e.preventDefault();

    setBusy(true);

    try {

      await post('/admin/uptime', {

        name: form.name.trim(),

        url: form.url.trim(),

        interval_sec: Number(form.interval_sec) || 300,

      });

      toast.ok(t('ad.up.added'));

      setForm({ name: '', url: '', interval_sec: '300' });

      await reload();

    } catch (e) {

      toast.err(pesanError(e, t('ad.up.addFail')));

    } finally {

      setBusy(false);

    }

  };



  return (

    <div className="p-6 md:p-10 max-w-4xl mx-auto">

      <header className="mb-8">

        <p className="label mb-1">{t('ad.common.admin')}</p>

        <h1 className="text-2xl font-semibold tracking-tight">{t('ad.up.h')}</h1>

      </header>



      {err && <p role="alert" className="mb-6 text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{err}</p>}



      <form onSubmit={tambah} className="card p-5 space-y-4 mb-10">

        <h2 className="font-semibold">{t('ad.up.addH')}</h2>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">

          <div className="space-y-2">

            <label htmlFor="m-nama" className="label">{t('ad.up.name')}</label>

            <Input id="m-nama" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder={t('ad.up.namePh')} required />

          </div>

          <div className="space-y-2">

            <label htmlFor="m-url" className="label">{t('ad.up.url')}</label>

            <Input id="m-url" type="url" value={form.url} onChange={(e) => setForm({ ...form, url: e.target.value })} placeholder="https://…/healthz" required />

          </div>

        </div>

        <div className="space-y-2 sm:max-w-[200px]">

          <label htmlFor="m-interval" className="label">{t('ad.up.interval')}</label>

          <Input id="m-interval" type="number" min={30} inputMode="numeric" value={form.interval_sec}

            onChange={(e) => setForm({ ...form, interval_sec: e.target.value })} />

        </div>

        <Button type="submit" disabled={busy || !form.name.trim() || !form.url.trim()}>

          {busy ? t('ad.common.saving') : t('ad.up.add')}

        </Button>

      </form>



      <section aria-label={t('ad.up.listAria')}>
        <h2 className="text-lg font-semibold tracking-tight mb-4">{t('ad.up.listH')}</h2>
        {err ? null : loading ? (

          <div className="space-y-3">{[0, 1].map((i) => <div key={i} className="card p-4 h-16"><div className="skel h-4 w-56" /></div>)}</div>

        ) : monitors.length === 0 ? (

          <div className="card p-8 text-center">

            <Activity className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />

            <p className="font-medium">{t('ad.up.empty')}</p>
            <p className="text-sm text-mute mt-1">{t('ad.up.emptyP')}</p>
          </div>

        ) : (

          <ul className="card divide-y divide-border">

            {monitors.map((m) => (

              <li key={m.id} className="p-4 flex items-center justify-between gap-4">

                <div className="min-w-0">

                  <p className="text-sm truncate">{m.name || t('ad.up.listH')}</p>

                  <p className="text-xs text-mute font-mono mt-1 truncate">{m.url || '-'}</p>

                  <p className="text-xs text-mute mt-1">

                    {t('ad.up.each', { n: m.interval_sec ?? '-', date: tanggal(m.checked_at ?? m.last_checked, true) })}

                  </p>

                </div>

                <div className="shrink-0 text-right">

                  <Badge>{statusLabel(m.status, m.is_active, t)}</Badge>

                  {m.uptime_pct !== undefined && (

                    <p className="text-xs text-mute mt-1 tabular-nums">{Number(m.uptime_pct).toFixed(2)}%</p>

                  )}

                </div>

              </li>

            ))}

          </ul>

        )}

      </section>

    </div>

  );

}

