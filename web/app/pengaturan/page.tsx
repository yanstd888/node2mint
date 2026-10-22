'use client';
// Pengaturan: profil, keamanan (2FA TOTP + sesi aktif). Tab lewat query ?tab= supaya
// link "Aktifkan 2FA" dari halaman masuk bisa langsung membuka tab keamanan.
import { useEffect, useState, Suspense } from 'react';
import { useSearchParams } from 'next/navigation';
import { api } from '@/lib/api';
import { patch, post, pesanError } from '@/lib/http';
import { useFetch } from '@/lib/useApi';
import { useAuth } from '@/lib/auth';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useConfirm } from '@/components/ui/confirm';
import { rupiah, asList } from '@/lib/utils';
import { tanggalLang } from '@/lib/tanggal';
import { useT, type Key } from '@/lib/i18n';
import { Pagination, useClientPagination } from '@/components/ui/Pagination';
import { ShieldCheck, ShieldOff, Trash2, Copy, Check } from 'lucide-react';
import { qrSvg } from '@/lib/qr';

type Sesi = { id: string; device?: string; user_agent?: string; ip?: string; created_at?: string; last_used?: string; current?: boolean };
type Status2FA = { active?: boolean; label?: string | null; sejak?: string | null };
type KYCData = { status?: 'unverified' | 'pending' | 'verified'; rejection_reason?: string };

const TAB = ['profil', 'keamanan'] as const;

function Pengaturan() {
  const params = useSearchParams();
  const { user } = useAuth();
  const { t } = useT();
  const awal = params.get('tab');
  const [tab, setTab] = useState<'profil' | 'keamanan'>(awal === 'keamanan' ? 'keamanan' : 'profil');

  return (
    <div className="p-6 md:p-10 max-w-3xl mx-auto">
      <header className="mb-6">
        <p className="label mb-1">{t('acc.kicker')}</p>
        <h1 className="text-2xl font-semibold tracking-tight">{t('acc.title')}</h1>
      </header>

      <div className="tab-pills mb-8" role="tablist" aria-label={t('acc.tabsAria')}>
        {TAB.map((x) => (
          <button key={x} role="tab" aria-selected={tab === x} onClick={() => setTab(x)} className="tab-pill">
            {t(x === 'profil' ? 'acc.tabProfil' : 'acc.tabKeamanan')}
          </button>
        ))}
      </div>

      {tab === 'profil' ? <Profil t={t} email={user?.email} saldo={user?.balance} peran={user?.role} /> : <Keamanan t={t} />}
    </div>
  );
}

type PayoutAccount = { id: string; type: 'bank' | 'ewallet' | 'crypto'; provider: string; account_identifier: string; account_name: string };
type ProfileData = { first_name?: string; last_name?: string; address?: string; payout_accounts?: PayoutAccount[] };

function Profil({ t, email, saldo, peran }: { t: (k: Key, v?: Record<string, string | number>) => string; email?: string; saldo?: number; peran?: string }) {
  const profile = useFetch<ProfileData>('/account/profile');
  const kyc = useFetch<KYCData>('/kyc');
  const telegram = useFetch<{ verified?: boolean; telegram_username?: string }>('/telegram/status');
  const confirm = useConfirm();
  const { logout } = useAuth();
  const [kycType, setKycType] = useState<'ktp' | 'passport'>('ktp');
  const [kycFile, setKycFile] = useState<File | null>(null);
  const [kycConsent, setKycConsent] = useState(false);
  const [form, setForm] = useState({ first_name: '', last_name: '', address: '' });
  const [account, setAccount] = useState({ type: 'bank', provider: '', account_identifier: '', account_name: '' });
  const [busy, setBusy] = useState('');
  const [banner, setBanner] = useState<{ ok: boolean; msg: string } | null>(null);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    if (profile.data) setForm({ first_name: profile.data.first_name || '', last_name: profile.data.last_name || '', address: profile.data.address || '' });
  }, [profile.data]);
  const fullName = `${form.first_name} ${form.last_name}`.trim();

  const saveProfile = async (e: React.FormEvent) => {
    e.preventDefault(); setBusy('profile'); setBanner(null);
    try { await patch('/account/profile', form); setBanner({ ok: true, msg: t('acc.saved') }); await profile.reload(); }
    catch (e) { setBanner({ ok: false, msg: pesanError(e, t('acc.saveFail')) }); }
    finally { setBusy(''); }
  };
  const addAccount = async (e: React.FormEvent) => {
    e.preventDefault(); setBusy('account'); setBanner(null);
    try {
      await post('/account/payout-accounts', account);
      setAccount({ type: 'bank', provider: '', account_identifier: '', account_name: fullName });
      setBanner({ ok: true, msg: t('acc.accountAdded') }); await profile.reload();
    } catch (e) { setBanner({ ok: false, msg: pesanError(e, t('acc.accountAddFail')) }); }
    finally { setBusy(''); }
  };
  const removeAccount = async (id: string) => {
    setBusy(id); setBanner(null);
    try { await api.del(`/account/payout-accounts/${id}`); setBanner({ ok: true, msg: t('acc.accountRemoved') }); await profile.reload(); }
    catch (e) { setBanner({ ok: false, msg: pesanError(e, t('acc.accountRemoveFail')) }); }
    finally { setBusy(''); }
  };

  const submitKyc = async (e: React.FormEvent) => {
    e.preventDefault(); if (!kycFile || !kycConsent) return;
    setBusy('kyc'); setBanner(null);
    try { const body = new FormData(); body.set('doc_type', kycType); body.set('document', kycFile); body.set('consent', 'true'); await api.post('/kyc', body); setKycFile(null); setKycConsent(false); setBanner({ ok: true, msg: t('acc.kycSent') }); await kyc.reload(); }
    catch (e) { setBanner({ ok: false, msg: pesanError(e, t('acc.kycSendFail')) }); }
    finally { setBusy(''); }
  };

  if (profile.loading) return <div className="space-y-6"><div className="skel h-28" /><div className="skel h-72" /></div>;
  if (profile.err) return <p role="alert" className="banner banner-bad">{profile.err}</p>;

  return (
    <section className="space-y-6">
      {banner && <p role={banner.ok ? 'status' : 'alert'} className={`banner ${banner.ok ? 'banner-ok' : 'banner-bad'}`}>{banner.msg}</p>}
      <div className="card saldo p-5 grid gap-3 sm:grid-cols-4">
        <div><span className="label">{t('auth.email')}</span><p className="text-sm truncate">{email || '-'}</p></div>
        <div><span className="label">{t('acc.role')}</span><p className="text-sm">{peran || '-'}</p></div>
        <div><span className="label">{t('dash.wallet')}</span><p className="text-lg font-semibold tabular-nums text-cyan-300">{rupiah(saldo)}</p></div>
        <div><span className="label">Telegram</span><p className={`text-sm ${telegram.data?.verified ? 'text-emerald-300' : 'text-mute'}`}>{telegram.data?.verified ? `${t('acc.telegramVerified')}${telegram.data.telegram_username ? ` (@${telegram.data.telegram_username})` : ''}` : t('acc.telegramPending')}</p></div>
      </div>

      <form onSubmit={saveProfile} className="card p-5 space-y-4">
        <div><h2 className="font-semibold">{t('acc.payoutH')}</h2><p className="mt-1 text-sm text-mute">{t('acc.payoutP')}</p></div>
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-2"><label htmlFor="first-name" className="label">{t('acc.firstName')}</label><Input id="first-name" value={form.first_name} onChange={(e) => setForm({ ...form, first_name: e.target.value })} required /></div>
          <div className="space-y-2"><label htmlFor="last-name" className="label">{t('acc.lastName')}</label><Input id="last-name" value={form.last_name} onChange={(e) => setForm({ ...form, last_name: e.target.value })} required /></div>
        </div>
        <div className="space-y-2"><label htmlFor="address" className="label">{t('acc.address')}</label><textarea id="address" className="input min-h-24 resize-y" value={form.address} onChange={(e) => setForm({ ...form, address: e.target.value })} minLength={10} maxLength={500} required /></div>
        <Button type="submit" disabled={busy === 'profile'}>{busy === 'profile' ? t('acc.saving') : t('acc.save')}</Button>
      </form>

      <section className="card p-5 space-y-4">
        <div><h2 className="font-semibold">{t('acc.accountsH')}</h2><p className="mt-1 text-sm text-mute">{t('acc.accountsP')}</p></div>
        {(profile.data?.payout_accounts ?? []).length > 0 && <ul className="divide-y divide-border rounded-lg border border-border">
          {(profile.data?.payout_accounts ?? []).map((a) => <li key={a.id} className="flex items-center justify-between gap-3 p-3"><div className="min-w-0"><p className="text-sm font-medium">{a.provider} · {a.type}</p><p className="truncate text-xs text-mute">{a.account_identifier} · {t('acc.holderShort')} {a.account_name}</p></div><button type="button" onClick={() => removeAccount(a.id)} disabled={busy === a.id} aria-label={t('acc.remove', { name: a.provider })} className="p-2 text-mute hover:text-danger"><Trash2 className="h-4 w-4" /></button></li>)}
        </ul>}
        <form onSubmit={addAccount} className="grid gap-3 border-t border-border pt-4 sm:grid-cols-2">
          <div className="space-y-2"><label htmlFor="payout-type" className="label">{t('acc.type')}</label><select id="payout-type" className="input" value={account.type} onChange={(e) => setAccount({ ...account, type: e.target.value })}><option value="bank">{t('acc.bank')}</option><option value="ewallet">{t('acc.ewallet')}</option><option value="crypto">{t('acc.crypto')}</option></select></div>
          <div className="space-y-2"><label htmlFor="payout-provider" className="label">{t('acc.provider')}</label><Input id="payout-provider" value={account.provider} onChange={(e) => setAccount({ ...account, provider: e.target.value })} placeholder="BCA / DANA / TRC20" required /></div>
          <div className="space-y-2"><label htmlFor="payout-number" className="label">{t('acc.number')}</label><Input id="payout-number" value={account.account_identifier} onChange={(e) => setAccount({ ...account, account_identifier: e.target.value })} required /></div>
          <div className="space-y-2"><label htmlFor="payout-name" className="label">{t('acc.owner')}</label><Input id="payout-name" value={account.account_name} onChange={(e) => setAccount({ ...account, account_name: e.target.value })} placeholder={fullName || t('acc.ownerPh')} required /></div>
          <Button type="submit" disabled={busy === 'account' || !fullName} className="sm:col-span-2">{busy === 'account' ? t('acc.adding') : t('acc.add')}</Button>
        </form>
      </section>

      <section className="card space-y-4 p-5">
        <div><h2 className="font-semibold">{t('acc.kycH')}</h2><p className="mt-1 text-sm text-mute">{t('acc.kycP')}</p></div>
        <p className="text-sm">{t('acc.kycStatus')}: <strong>{kyc.data?.status ? t(`acc.kyc${kyc.data.status[0].toUpperCase()}${kyc.data.status.slice(1)}` as Key) : t('acc.kycUnverified')}</strong>{kyc.data?.rejection_reason ? ` — ${kyc.data.rejection_reason}` : ''}</p>
        {kyc.data?.status !== 'verified' && <form onSubmit={submitKyc} className="grid gap-3 sm:grid-cols-[10rem_1fr_auto] sm:items-end">
          <label className="space-y-2 text-sm"><span className="label">{t('acc.kycType')}</span><select className="input" value={kycType} onChange={(e) => setKycType(e.target.value as 'ktp' | 'passport')}><option value="ktp">{t('acc.kycKtp')}</option><option value="passport">{t('acc.kycPassport')}</option></select></label>
          <label className="space-y-2 text-sm"><span className="label">{t('acc.kycDoc')}</span><input className="input" type="file" accept="application/pdf,image/jpeg,image/png" onChange={(e) => setKycFile(e.target.files?.[0] || null)} required /></label>
          <label className="flex items-start gap-2 text-xs text-mute sm:col-span-2"><input type="checkbox" checked={kycConsent} onChange={(e) => setKycConsent(e.target.checked)} required className="mt-0.5" /> <span>{t('acc.kycConsent')} <a href="/privacy" className="text-cyan-300 hover:text-cyan-200">Privacy Policy</a>.</span></label>
          <Button type="submit" disabled={!kycFile || !kycConsent || busy === 'kyc'}>{busy === 'kyc' ? t('acc.kycSending') : t('acc.kycSubmit')}</Button>
        </form>}
      </section>

      <section className="card space-y-3 p-5">
        <div><h2 className="font-semibold">{t('acc.dataH')}</h2><p className="mt-1 text-sm text-mute">{t('acc.dataP')}</p></div>
        <div className="flex flex-wrap gap-3">
          <Button type="button" variant="outline" onClick={() => { window.location.href = '/api/account/export'; }}>{t('acc.download')}</Button>
          <Button type="button" variant="destructive" disabled={deleting} onClick={async () => { if (deleting) return; if (!(await confirm({ title: t('acc.deleteH'), body: t('acc.deleteBody'), confirmLabel: t('acc.delete') }))) return; setDeleting(true); try { await post('/account/delete', {}); await logout(); window.location.assign('/masuk?next=/pengaturan'); } catch (e) { setBanner({ ok: false, msg: pesanError(e, t('acc.deleteFail')) }); } finally { setDeleting(false); } }}>{deleting ? t('acc.deleting') : t('acc.delete')}</Button>
        </div>
      </section>
    </section>
  );
}

function Keamanan({ t }: { t: (k: Key, v?: Record<string, string | number>) => string }) {
  const { lang } = useT();
  const { data, loading, err, reload } = useFetch<Status2FA>('/account/2fa');
  const sesi = useFetch<unknown>('/account/sessions');
  const { refresh } = useAuth();
  const [uri, setUri] = useState('');
  const [kode, setKode] = useState('');
  const [matiKode, setMatiKode] = useState('');
  const [busy, setBusy] = useState<string | null>(null);
  const [banner, setBanner] = useState<{ ok: boolean; msg: string } | null>(null);
  // Cabut sesi itu destruktif (perangkat lain langsung logout) — konfirmasi dulu.
  const [konfirmasi, setKonfirmasi] = useState<string | null>(null);

  // URI otpauth memuat rahasianya di parameter ?secret= — QR digambar lokal di
  // browser lewat lib/qr.ts, jadi rahasia tidak pernah dikirim ke layanan online.
  const rahasia = (() => {
    try { return new URL(uri).searchParams.get('secret') || ''; } catch { return ''; }
  })();

  // QR dari URI otpauth (bukan dari rahasia saja) supaya issuer/label ikut terbaca.
  const qrSvgMarkup = (() => {
    if (!uri) return '';
    try { return qrSvg(uri, { size: 200, margin: 4, ec: 'M' }); } catch { return ''; }
  })();

  // status salin per-tombol supaya tidak perlu state terpisah per field
  const [disalin, setDisalin] = useState<string | null>(null);
  const salin = async (nilai: string, kunci: string) => {
    try {
      await navigator.clipboard.writeText(nilai);
      setDisalin(kunci);
      window.setTimeout(() => setDisalin((x) => (x === kunci ? null : x)), 2000);
    } catch {
      setDisalin(null);
    }
  };

  const daftar = asList<Sesi>(sesi.data, 'sessions');
  const sesiPage = useClientPagination(daftar, 10);

  const mulai = async () => {
    setBusy('setup');
    try {
      const { data: res } = await post<{ uri?: string; secret?: string; otpauth_url?: string }>('/account/2fa/setup', {});
      setUri(res?.uri || res?.otpauth_url || '');
      setBanner({ ok: true, msg: t('acc.setupMsg') });
    } catch (e) {
      setBanner({ ok: false, msg: pesanError(e, t('acc.setupFail')) });
    } finally {
      setBusy(null);
    }
  };

  const aktifkan = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy('enable');
    try {
      await post('/account/2fa/enable', { code: kode.trim() });
      setBanner({ ok: true, msg: t('acc.enableMsg') });
      setUri('');
      setKode('');
      reload();
      refresh();
    } catch (e) {
      setBanner({ ok: false, msg: pesanError(e, t('acc.enableFail')) });
    } finally {
      setBusy(null);
    }
  };

  // Matikan 2FA: konfirmasi dengan kode TOTP berjalan supaya bukan siapa pun
  // yang pegang sesi bisa mematikannya.
  const matikan = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy('disable');
    try {
      await post('/account/2fa/disable', { code: matiKode.trim() });
      setBanner({ ok: true, msg: t('acc.disableMsg') });
      setMatiKode('');
      reload();
      refresh();
    } catch (e) {
      setBanner({ ok: false, msg: pesanError(e, t('acc.disableFail')) });
    } finally {
      setBusy(null);
    }
  };

  const hapusSesi = async (id: string) => {
    setBusy(`sesi:${id}`);
    try {
      await api.del(`/account/sessions/${id}`);
      setBanner({ ok: true, msg: t('acc.ended') });
      sesi.reload();
    } catch (e) {
      setBanner({ ok: false, msg: pesanError(e, t('acc.endFail')) });
    } finally {
      setBusy(null);
      setKonfirmasi(null);
    }
  };

  return (
    <section className="space-y-8">
      {banner && (
        <p role={banner.ok ? 'status' : 'alert'}
          className={`text-sm rounded-lg px-4 py-3 border ${banner.ok ? 'bg-cyan-500/10 border-cyan-400/30 text-cyan-200' : 'bg-danger/10 border-danger/30 text-danger'}`}>
          {banner.msg}
        </p>
      )}

      <div className="card p-5">
        <div className="flex items-center gap-3 mb-2">
          {data?.active
            ? <ShieldCheck className="w-5 h-5 text-cyan-300" aria-hidden="true" />
            : <ShieldOff className="w-5 h-5 text-mute" aria-hidden="true" />}
          <h2 className="font-semibold">{t('acc.twoFaH')}</h2>
          <span className={`badge ${data?.active ? 'badge-on' : 'badge-off'}`}>{data?.active ? t('acc.on') : t('acc.off')}</span>
        </div>

        {err && <p role="alert" className="text-sm text-danger mt-2">{err}</p>}
        {loading && <div className="skel h-4 w-48 mt-2" />}
        {data?.sejak && <p className="text-xs text-mute">{t('acc.since', { date: tanggalLang(data.sejak, lang, true) })}</p>}

        {!loading && !data?.active && !uri && (
          <Button className="mt-4" disabled={busy === 'setup'} onClick={mulai}>
            {busy === 'setup' ? t('acc.setupBusy') : t('acc.setup')}
          </Button>
        )}

        {uri && (
          <form onSubmit={aktifkan} className="mt-4 space-y-4">
            <div className="grid gap-5 sm:grid-cols-[200px_1fr] sm:items-start">
              {qrSvgMarkup && (
                <div className="space-y-2">
                  <span className="label">{t('acc.scanQr')}</span>
                  <div
                    className="rounded-xl bg-white p-3 w-[200px] h-[200px] [&>svg]:w-full [&>svg]:h-full"
                    // SVG dibuat lokal dari URI otpauth; tidak ada request keluar.
                    dangerouslySetInnerHTML={{ __html: qrSvgMarkup }}
                  />
                </div>
              )}
              <div className="space-y-2 min-w-0">
                <span className="label">{t('acc.secret')}</span>
                <div className="flex items-start gap-2">
                  <p className="font-mono text-sm bg-bg border border-border rounded-lg p-3 break-all select-all text-cyan-300 flex-1">
                    {rahasia || uri}
                  </p>
                  <button
                    type="button"
                    onClick={() => salin(rahasia || uri, 'secret')}
                    aria-label={t('acc.copySecret')}
                    className="p-2 mt-1 text-mute hover:text-cyan-300 hover:bg-cyan-400/10 rounded-lg transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60 shrink-0"
                  >
                    {disalin === 'secret' ? <Check className="w-4 h-4 text-cyan-300" aria-hidden="true" /> : <Copy className="w-4 h-4" aria-hidden="true" />}
                  </button>
                </div>
                {disalin === 'secret' && <p className="text-xs text-cyan-300">{t('acc.copied')}</p>}
                <p className="text-xs text-mute">
                  {t('acc.secretHint')}
                </p>
              </div>
            </div>
            <div className="space-y-2">
              <label htmlFor="kode-2fa" className="label">{t('acc.code6')}</label>
              <Input id="kode-2fa" value={kode} onChange={(e) => setKode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                inputMode="numeric" autoComplete="one-time-code" placeholder="123456" required />
            </div>
            <div className="act-group">
              <Button type="submit" disabled={busy === 'enable' || kode.length !== 6}>
                {busy === 'enable' ? t('acc.enabling') : t('acc.enable')}
              </Button>
              <Button type="button" variant="ghost" onClick={() => { setUri(''); setKode(''); }}>{t('mkt.cancel')}</Button>
            </div>
          </form>
        )}
        {data?.active && !uri && (
          <form onSubmit={matikan} className="mt-4 space-y-3 border-t border-border pt-4">
            <span className="label">{t('acc.disableH')}</span>
            <p className="text-xs text-mute">
              {t('acc.disableP')}
            </p>
            <div className="flex flex-wrap items-center gap-2">
              <Input id="kode-matikan-2fa" value={matiKode}
                onChange={(e) => setMatiKode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                inputMode="numeric" autoComplete="one-time-code" placeholder="123456" className="w-32" />
              <Button type="submit" variant="ghost" disabled={busy === 'disable' || matiKode.length !== 6}>
                {busy === 'disable' ? t('acc.disabling') : t('acc.disable')}
              </Button>
            </div>
          </form>
        )}
      </div>

      <div>
        <h2 className="text-lg font-semibold tracking-tight mb-4">{t('acc.sessionsH')}</h2>
        {sesi.err ? (
          <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{sesi.err}</p>
        ) : sesi.loading ? (
          <div className="space-y-3">{[0, 1].map((i) => <div key={i} className="card p-4 h-16"><div className="skel h-4 w-56" /></div>)}</div>
        ) : daftar.length === 0 ? (
          <p className="card p-6 text-sm text-mute text-center">{t('acc.noSessions')}</p>
        ) : (
          <>
            <ul className="tbl-wrap zebra">
            {sesiPage.items.map((s) => (
              <li key={s.id} className="p-4 flex items-center justify-between gap-4">
                <div className="min-w-0">
                  <p className="text-sm truncate">
                    {s.device || s.user_agent || t('acc.unknownDevice')}
                    {s.current && <span className="text-cyan-300 text-xs"> &middot; {t('acc.thisSession')}</span>}
                  </p>
                  <p className="text-xs text-mute mt-1 font-mono">
                    {s.ip || '-'} &middot; {tanggalLang(s.last_used ?? s.created_at, lang, true)}
                  </p>
                </div>
                {konfirmasi === s.id ? (
                  <div className="act-group">
                    <span className="text-xs text-danger hidden sm:block">{t('acc.endAsk')}</span>
                    <Button variant="destructive" size="sm" disabled={busy === `sesi:${s.id}`} onClick={() => hapusSesi(s.id)}>
                      {busy === `sesi:${s.id}` ? t('acc.ending') : t('acc.endYes')}
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => setKonfirmasi(null)}>{t('mkt.cancel')}</Button>
                  </div>
                ) : (
                  <button onClick={() => setKonfirmasi(s.id)} aria-label={t('acc.endAria', { name: s.device || s.user_agent || t('acc.thisSession') })}
                    className="p-2 text-mute hover:text-danger hover:bg-danger/10 rounded-lg transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-danger/60 shrink-0">
                    <Trash2 className="w-4 h-4" aria-hidden="true" />
                  </button>
                )}
              </li>
            ))}
            </ul>
            {daftar.length > 10 && <Pagination page={sesiPage.page} hasMore={sesiPage.hasMore} onPage={sesiPage.setPage} />}
          </>
        )}
      </div>
    </section>
  );
}

export default function PengaturanPage() {
  // useSearchParams butuh Suspense boundary di App Router.
  return (
    <Suspense fallback={<div className="p-6 md:p-10 max-w-3xl mx-auto"><div className="skel h-8 w-52" /></div>}>
      <Pengaturan />
    </Suspense>
  );
}
