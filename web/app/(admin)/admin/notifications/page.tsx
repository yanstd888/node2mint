'use client';

// Admin: kelola notifikasi sistem. Buat notifikasi ke semua user atau user tunggal.

import { useEffect, useState } from 'react';

import { api } from '@/lib/api';

import { useAuth } from '@/lib/auth';

import { useToast } from '@/components/ui/toast';

import { useT } from '@/lib/i18n';

import { Bell, Send, Trash2, Users, User } from 'lucide-react';
import { useConfirm } from '@/components/ui/confirm';



type AdminNotif = {

  id: string;

  user_id: string;

  type: string;

  title: string;

  message: string;

  is_read: boolean;

  created_at: string;

};



type NewNotif = {

  user_id: string;

  title: string;

  message: string;

};



export default function AdminNotificationsPage() {

  const { user } = useAuth();

  const toast = useToast();

  const { t } = useT();

  const [notifs, setNotifs] = useState<AdminNotif[]>([]);

  const [loading, setLoading] = useState(true);

  const [form, setForm] = useState<NewNotif>({ user_id: '', title: '', message: '' });

  const [target, setTarget] = useState<'all' | 'user'>('all');

  const [submitting, setSubmitting] = useState(false);
  const confirm = useConfirm();



  const isAdmin = user?.role === 'ADMIN' || user?.role === 'SUPER_ADMIN';



  useEffect(() => {

    if (!isAdmin) return;

    void api.get<AdminNotif[]>('/admin/notifications').then(({ data }) => {

      setNotifs(data);

      setLoading(false);

    }).catch(() => {

      toast.err(t('ad.nt.loadFail'));

      setLoading(false);

    });

  }, [isAdmin]); // eslint-disable-line react-hooks/exhaustive-deps



  const submit = async (e: React.FormEvent) => {

    e.preventDefault();

    if (!form.title.trim() || !form.message.trim()) {

      toast.err(t('ad.nt.required'));

      return;

    }

    const n = target === 'all' ? 0 : 1;
    if (!(await confirm({ title: t('ad.nt.confirmSend'), body: t('ad.nt.sendBody', { n }), confirmLabel: t('ad.nt.send') }))) return;

    setSubmitting(true);

    try {

      await api.post('/admin/notifications', {

        user_id: target === 'user' ? form.user_id : undefined,

        title: form.title,

        message: form.message,

        type: 'SYSTEM',

      });

      toast.ok(t('ad.nt.sent'));

      setForm({ user_id: '', title: '', message: '' });

      const { data } = await api.get<AdminNotif[]>('/admin/notifications');

      setNotifs(data);

    } catch {

      toast.err(t('ad.nt.sendFail'));

    } finally {

      setSubmitting(false);

    }

  };



  const remove = async (id: string) => {

    const item = notifs.find((n) => n.id === id);
    if (!(await confirm({ title: t('ad.nt.deleteOf'), body: item?.title || '-', confirmLabel: t('ad.common.delete') }))) return;

    try {

      await api.del(`/admin/notifications/${id}`);

      setNotifs(notifs.filter((n) => n.id !== id));

      toast.ok(t('ad.nt.deleted'));

    } catch {

      toast.err(t('ad.nt.delFail'));

    }

  };



  if (!user || !isAdmin) return null;



  return (

    <div className="p-6 md:p-10 max-w-6xl mx-auto">

      <header className="mb-8">

        <p className="label mb-1">{t('ad.common.admin')}</p>

        <h1 className="text-2xl md:text-3xl font-semibold tracking-tight">{t('ad.nt.h')}</h1>

        <p className="mt-2 text-sm text-mute">

          {t('ad.nt.sub')}

        </p>

      </header>



      <section className="card p-6 mb-6">

        <h2 className="font-semibold mb-4 flex items-center gap-2">

          <Send className="h-4 w-4" />

          {t('ad.nt.sendH')}

        </h2>

        <form onSubmit={submit} className="space-y-4">

          <div>

            <label className="block text-sm font-medium mb-1">{t('ad.nt.target')}</label>

            <div className="flex gap-4 text-sm">

              <label className="flex items-center gap-2">

                <input

                  type="radio"

                  name="target"

                  checked={target === 'all'}

                  onChange={() => { setTarget('all'); setForm({ ...form, user_id: '' }); }}

                />

                <Users className="h-4 w-4" /> {t('ad.nt.allUsers')}

              </label>

              <label className="flex items-center gap-2">

                <input

                  type="radio"

                  name="target"

                  checked={target === 'user'}

                  onChange={() => setTarget('user')}

                />

                <User className="h-4 w-4" /> {t('ad.nt.singleUser')}

              </label>

            </div>

          </div>



          {target === 'user' && (

            <div>

              <label className="block text-sm font-medium mb-1">{t('ad.nt.userId')}</label>

              <input

                type="text"

                value={form.user_id}

                onChange={(e) => setForm({ ...form, user_id: e.target.value })}

                placeholder={t('ad.nt.userIdPh')}

                className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60"

                required

              />

            </div>

          )}



          <div>

            <label className="block text-sm font-medium mb-1">{t('ad.nt.title')}</label>

            <input

              type="text"

              value={form.title}

              onChange={(e) => setForm({ ...form, title: e.target.value })}

              placeholder={t('ad.nt.titlePh')}

              className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60"

              required

            />

          </div>



          <div>

            <label className="block text-sm font-medium mb-1">{t('ad.nt.message')}</label>

            <textarea

              value={form.message}

              onChange={(e) => setForm({ ...form, message: e.target.value })}

              placeholder={t('ad.nt.messagePh')}

              rows={3}

              className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60"

              required

            />

          </div>



          <button

            type="submit"

            disabled={submitting || !form.title.trim() || !form.message.trim()}

            className="flex items-center gap-2 rounded-lg border border-cyan-400/30 bg-cyan-500 px-4 py-2 text-sm font-medium text-bg hover:bg-cyan-400 disabled:opacity-50"

          >

            <Send className="h-4 w-4" />

            {submitting ? t('ad.nt.sending') : t('ad.nt.send')}

          </button>

        </form>

      </section>



      <section>

        <h2 className="font-semibold mb-4 flex items-center gap-2">

          <Bell className="h-4 w-4" />

          {t('ad.nt.sentH')}

        </h2>

        {loading ? (

          <div className="space-y-2">

            {[0, 1, 2].map((i) => <div key={i} className="skel h-16 rounded-lg" />)}

          </div>

        ) : notifs.length === 0 ? (

          <p className="text-sm text-mute">{t('ad.nt.empty')}</p>

        ) : (

          <div className="overflow-x-auto">

            <table className="w-full text-sm">

              <thead>

                <tr className="border-b border-border">

                  <th className="text-left py-2">{t('ad.nt.title')}</th>

                  <th className="text-left py-2">{t('ad.nt.message')}</th>

                  <th className="text-left py-2">{t('ad.nt.target')}</th>

                  <th className="text-left py-2">{t('ad.nt.read')}</th>

                  <th className="text-left py-2">{t('ad.nt.date')}</th>

                  <th className="text-center py-2">{t('ad.common.actions')}</th>

                </tr>

              </thead>

              <tbody>

                {notifs.map((n) => (

                  <tr key={n.id} className="border-b border-border/30">

                    <td className="py-2 font-medium">{n.title}</td>

                    <td className="py-2 text-mute max-w-xs truncate">{n.message}</td>

                    <td className="py-2">

                      {n.user_id ? n.user_id.slice(0, 8) + '...' : t('ad.nt.allUsers')}

                    </td>

                    <td className="py-2">{n.is_read ? t('ad.common.yes') : t('ad.common.no')}</td>

                    <td className="py-2 text-xs text-mute">

                      {new Date(n.created_at).toLocaleDateString('id-ID')}

                    </td>

                    <td className="py-2 text-center">

                      <button

                        onClick={() => void remove(n.id)}

                        aria-label={t('ad.common.delete')}

                        className="p-1 text-mute hover:text-danger rounded focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-cyan-400/60"

                      >

                        <Trash2 className="h-4 w-4" />

                      </button>

                    </td>

                  </tr>

                ))}

              </tbody>

            </table>

          </div>

        )}

      </section>

    </div>

  );

}

