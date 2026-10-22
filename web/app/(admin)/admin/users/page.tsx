'use client';
// Admin: daftar user (email, role, saldo).
import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import { Pagination } from '@/components/ui/Pagination';
import { rupiah } from '@/lib/utils';
import { useT } from '@/lib/i18n';
import { useConfirm } from '@/components/ui/confirm';

type User = { id: string; email: string; role: string; wallet_balance: number; active_api_keys: number; tokens_remaining: number; tokens_used: number; is_banned: boolean; banned_reason?: string };

export default function AdminUsersPage() {
  const { t } = useT();
  const [users, setUsers] = useState<User[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState('');
  const confirm = useConfirm();

  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const { data } = await api.get<{ users: User[]; has_more?: boolean }>(`/admin/users?page=${page}`);
        if (alive) {
          setUsers(data.users || []);
          setHasMore(!!data.has_more);
        }
      } catch {
        if (alive) setErr(t('ad.us.loadErr'));
      } finally {
        if (alive) setLoading(false);
      }
    })();
    return () => { alive = false; };
  }, [page, t]);

  return (
    <div className="p-6 md:p-10 max-w-7xl mx-auto">
      <header className="mb-8">
        <p className="label mb-1">{t('ad.common.admin')}</p>
        <h1 className="text-2xl font-semibold tracking-tight">{t('adm.users')}</h1>
      </header>

      {err && <p role="alert" className="mb-6 text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{err}</p>}

      {loading ? (
        <div className="space-y-3">
          {[0, 1, 2, 3].map((i) => <div key={i} className="card p-4 h-16"><div className="skel h-4 w-64" /></div>)}
        </div>
      ) : (
        <div className="tbl-wrap">
          <table className="w-full text-sm">
            <thead>
              <tr className="tbl-head">
                <th>{t('ad.us.email')}</th>
                <th>{t('ad.us.role')}</th>
                <th className="text-right">{t('ad.us.balance')}</th>
				<th className="text-right">{t('ad.us.apiKeys')}</th>
				<th className="text-right">{t('ad.us.tokensLeft')}</th>
				<th className="text-right">{t('ad.us.tokensUsed')}</th>
                <th>{t('ad.common.actions')}</th>
              </tr>
            </thead>
            <tbody className="zebra">
              {users.map((u) => (
                <tr key={u.id}>
                  <td className="px-4 py-3 break-all">{u.email}</td>
                  <td className="px-4 py-3"><span className={`badge ${u.role === 'admin' ? 'badge-off' : 'badge-on'}`}>{u.role}</span></td>
                  <td className="px-4 py-3 text-right tabular-nums text-cyan-300">{rupiah(u.wallet_balance)}</td>
				  <td className="px-4 py-3 text-right tabular-nums">{u.active_api_keys.toLocaleString()}</td>
				  <td className="px-4 py-3 text-right tabular-nums">{u.tokens_remaining.toLocaleString()}</td>
				  <td className="px-4 py-3 text-right tabular-nums">{u.tokens_used.toLocaleString()}</td>
                  <td className="px-4 py-3">
                    {u.is_banned ? (
                      <button disabled={busy === u.id} className="btn btn-sm" onClick={async () => {
                        if (!(await confirm({ title: t('ad.us.unban'), body: `${u.email} — access will be restored.`, confirmLabel: t('ad.us.unban'), variant: 'default' }))) return;
                        setBusy(u.id); setErr('');
                        try { await api.post(`/admin/users/${u.id}/unban`, {}); setUsers((xs) => xs.map((x) => x.id === u.id ? { ...x, is_banned: false, banned_reason: '' } : x)); }
                        catch { setErr(t('ad.us.unbanFail')); } finally { setBusy(''); }
                      }}>{t('ad.us.unban')}</button>
                    ) : u.role === 'admin' ? <span className="text-mute">—</span> : (
                      <button disabled={busy === u.id} className="btn btn-sm btn-danger" onClick={async () => {
                        const reason = window.prompt(t('ad.us.banReason'));
                        if (!reason?.trim()) return;
                        if (!(await confirm({ title: t('ad.us.ban'), body: `${u.email} — login will be blocked.`, confirmLabel: t('ad.us.ban') }))) return;
                        setBusy(u.id); setErr('');
                        try { await api.post(`/admin/users/${u.id}/ban`, { reason: reason.trim() }); setUsers((xs) => xs.map((x) => x.id === u.id ? { ...x, is_banned: true, banned_reason: reason.trim() } : x)); }
                        catch { setErr(t('ad.us.banFail')); } finally { setBusy(''); }
                      }}>{t('ad.us.ban')}</button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {users.length === 0 && !err && <p className="p-8 text-center text-sm text-mute">{t('ad.us.empty')}</p>}
        </div>
      )}

      {!err && !loading && users.length > 0 && (
        <Pagination page={page} hasMore={hasMore} onPage={setPage} />
      )}
    </div>
  );
}
