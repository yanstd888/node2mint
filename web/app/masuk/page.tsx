'use client';
// Masuk + Daftar: split layout, panel brand kiri (desktop), form kanan.
// Bahasa visual mengikuti landing (card kaca, kicker mono, grid + spotlight).
// Satu slot provider untuk kedua mode, supaya masuk & daftar persis sama.
import { useEffect, useRef, useState, Suspense } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { AlertCircle, ArrowLeft, CheckCircle2, Eye, EyeOff, Gift, KeyRound, Mail, ShieldCheck, UserRound } from 'lucide-react';
import { api } from '@/lib/api';
import { apiErrorCode, pesanError } from '@/lib/http';
import { useAuth } from '@/lib/auth';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useT, type Key } from '@/lib/i18n';
import { useToast } from '@/components/ui/toast';
import LanguageMenu from '@/components/LanguageMenu';

declare global {
  interface Window {
    turnstile?: { render: (element: HTMLElement, options: Record<string, unknown>) => string; remove: (id: string) => void };
  }
}

const NAMA: Record<string, string> = { google: 'Google', telegram: 'Telegram' };
// Mark resmi provider dari public/provider-logos (lokal, tanpa aset runtime luar).
const MARK: Record<string, string> = {
  google: '/provider-logos/google.svg',
  telegram: '/provider-logos/telegram.svg',
};

type Provider = { id: string; enabled: boolean; url?: string };

// Mark provider + label. Ukuran/opasitas ikut status aktif, jadi baris disabled
// tetap terbaca sebagai provider resmi namun jelas tidak bisa diklik.
function ProviderRow({ id, label, note }: { id: string; label: string; note?: string }) {
  return (
    <>
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src={MARK[id]} alt="" aria-hidden="true" className={`h-[18px] w-[18px] shrink-0 object-contain ${note ? 'opacity-45 grayscale' : 'opacity-95'}`} />
      <span className="truncate">{label}</span>
      {note && <span className="ml-auto shrink-0 pl-2 text-xs text-mute">{note}</span>}
    </>
  );
}

function OAuthButtons({
  aff,
  turnstileSiteKey,
  turnstileToken,
  onTurnstileSiteKey,
  onError,
  onConsumed,
}: {
  aff: string;
  turnstileSiteKey: string;
  turnstileToken: string;
  onTurnstileSiteKey: (siteKey: string) => void;
  onError: (message: string) => void;
  onConsumed: () => void;
}) {
  const { t } = useT();
  const [providers, setProviders] = useState<Provider[]>([]);
  const [busy, setBusy] = useState('');

  useEffect(() => {
    let alive = true;
    api.get<{ providers: Provider[]; turnstile_site_key?: string }>('/auth/providers')
      .then(({ data }) => { if (alive) { setProviders(data.providers || []); onTurnstileSiteKey(data.turnstile_site_key || ''); } })
      .catch(() => {});
    return () => { alive = false; };
  }, [onTurnstileSiteKey]);

  const start = async (provider: string) => {
    if (turnstileSiteKey && !turnstileToken) {
      onError(t('auth.turnstileRequired'));
      return;
    }
    setBusy(provider);
    onError('');
    try {
      const { data } = await api.post<{ url: string }>('/auth/oauth/start', {
        provider,
        aff_code: aff || undefined,
        turnstile_token: turnstileToken || undefined,
      });
      if (!data.url) throw new Error('OAuth URL kosong');
      window.location.assign(data.url);
    } catch (e: any) {
      onError(pesanError(e, t('auth.fail')));
      onConsumed();
      setBusy('');
    }
  };

  const byID = new Map(providers.map((p) => [p.id, p]));
  const row = 'flex h-11 items-center gap-3 rounded-2xl border px-4 text-sm';
  return (
    <div className="mt-6">
      <div className="flex items-center gap-3" role="separator">
        <span className="h-px flex-1 bg-white/10" />
        <span className="font-mono text-[10px] uppercase tracking-[.16em] text-mute">{t('auth.or')}</span>
        <span className="h-px flex-1 bg-white/10" />
      </div>
      <div className="mt-4 space-y-2">
        {['google', 'telegram'].map((id) => {
          const provider = byID.get(id);
          const label = t('auth.with', { name: NAMA[id] });
          return provider?.enabled ? (
            <button key={id} type="button" disabled={!!busy} onClick={() => void start(id)} className={`${row} w-full border-border bg-surface/60 text-left hover:border-white/20 hover:bg-surface-hi focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60 disabled:cursor-wait disabled:opacity-60`}>
              <ProviderRow id={id} label={label} />
              {busy === id && <span className="ml-auto h-4 w-4 animate-spin rounded-full border-2 border-white/20 border-t-cyan-300" aria-hidden="true" />}
            </button>
          ) : (
            <span key={id} aria-disabled="true" title={t('auth.unavailable')} className={`${row} border-border/60 bg-surface/30 text-mute cursor-not-allowed`}>
              <ProviderRow id={id} label={label} note={t('auth.unavailable')} />
            </span>
          );
        })}
      </div>
    </div>
  );
}

function TurnstileBox({ siteKey, onToken }: { siteKey: string; onToken: (token: string) => void }) {
  const slot = useRef<HTMLDivElement>(null);
  useEffect(() => {
    onToken('');
    if (!siteKey || !slot.current) return;
    let active = true;
    let widget = '';
    const render = () => {
      if (!active || !slot.current || !window.turnstile) return;
      slot.current.replaceChildren();
      widget = window.turnstile.render(slot.current, {
        sitekey: siteKey, theme: 'dark', action: 'auth',
        callback: (token: string) => onToken(token),
        'expired-callback': () => onToken(''),
        'error-callback': () => onToken(''),
      });
    };
    let script = document.querySelector<HTMLScriptElement>('script[data-node2mint-turnstile]');
    if (window.turnstile) render();
    else if (script) script.addEventListener('load', render, { once: true });
    else {
      script = document.createElement('script');
      script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit';
      script.async = true;
      script.dataset.node2mintTurnstile = 'true';
      script.addEventListener('load', render, { once: true });
      document.head.append(script);
    }
    return () => {
      active = false;
      script?.removeEventListener('load', render);
      if (widget) window.turnstile?.remove(widget);
    };
  }, [siteKey, onToken]);
  return siteKey ? <div className="flex justify-center pt-1" aria-label="Verifikasi keamanan"><div ref={slot} /></div> : null;
}

export default function LoginPage() {
  return <Suspense fallback={<main className="min-h-dvh" />}><LoginBody /></Suspense>;
}

// Wrapper kaca yang sama untuk panel brand dan kartu form, sesuai DESIGN-SYSTEM.
function Panel({ className, children }: { className: string; children: React.ReactNode }) {
  return <div className={`card bg-white/[.025] ${className}`}>{children}</div>;
}

function LoginBody() {
  const params = useSearchParams();
  const [mode, setMode] = useState<'masuk' | 'daftar'>(params.get('mode') === 'daftar' || params.has('aff') ? 'daftar' : 'masuk');
  const [email, setEmail] = useState('');
  const [fullName, setFullName] = useState('');
  const [pass, setPass] = useState('');
  const [passConfirm, setPassConfirm] = useState('');
  const [showPass, setShowPass] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [emailTouched, setEmailTouched] = useState(false);
  const [passTouched, setPassTouched] = useState(false);
  const [confirmTouched, setConfirmTouched] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState('');
  const [ok, setOk] = useState(false);
  // 2FA: input kode muncul hanya setelah backend menolak dengan "kode 2FA diperlukan".
  // Sebelum ini tidak ada jalan sama sekali bagi akun ber-2FA untuk login.
  const [totp, setTotp] = useState('');
  const [perlu2fa, setPerlu2fa] = useState(false);
  const [turnstileSiteKey, setTurnstileSiteKey] = useState('');
  const [turnstileToken, setTurnstileToken] = useState('');
  const [turnstileGeneration, setTurnstileGeneration] = useState(0);
  const { login } = useAuth();
  const toast = useToast();
  const { t } = useT();
  const next = params.get('next');
  const tujuan = next && next.startsWith('/') && !next.startsWith('//') ? next : '/dashboard';
  // Link ?aff= langsung membuka form daftar dan mengisi kode undangan.
  const [aff, setAff] = useState(() => (params.get('aff') || '').trim().toUpperCase());
  const emailInvalid = !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim());
  const emailError = emailTouched && emailInvalid ? t('auth.emailInvalid') : '';
  const passwordError = passTouched && (pass.length < 8 || pass.length > 72) ? t('auth.passReq') : '';
  const confirmError = confirmTouched && passConfirm && pass !== passConfirm ? t('auth.passMismatch') : '';

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr('');
    setOk(false);
    const names = fullName.trim().split(/\s+/).filter(Boolean);
    setEmailTouched(true);
    if (emailInvalid) {
      setErr(t('auth.emailInvalid'));
      setBusy(false);
      return;
    }
    if (turnstileSiteKey && !turnstileToken) {
      setErr(t('auth.turnstileRequired'));
      setBusy(false);
      return;
    }
    if (mode === 'daftar' && names.length < 1) {
      setErr(t('auth.fullNameParts'));
      setBusy(false);
      return;
    }
    if (mode === 'daftar' && passwordError) {
      setErr(passwordError);
      setBusy(false);
      return;
    }
    if (mode === 'daftar' && pass !== passConfirm) {
      setErr(t('auth.passMismatch'));
      setBusy(false);
      return;
    }
    try {
      if (mode === 'daftar') {
        await api.post('/auth/register', {
          first_name: names[0],
          last_name: names.slice(1).join(' '),
          email: email.trim().toLowerCase(),
          password: pass,
          password_confirm: passConfirm,
          aff_code: aff || undefined,
          turnstile_token: turnstileToken || undefined,
        });
        // Token Turnstile sekali pakai; setelah daftar, minta challenge baru
        // untuk login alih-alih mencoba memakai token pendaftaran ulang.
        setOk(true);
        toast.ok(t('auth.accountCreated'));
        setMode('masuk');
        setPass('');
        setPassConfirm('');
        if (turnstileSiteKey) setTurnstileGeneration((current) => current + 1);
        setBusy(false);
        return;
      }
      await login(email, pass, turnstileToken || undefined, perlu2fa ? totp.trim() : undefined);
      toast.ok(t('auth.loginSuccess'));
      window.location.href = tujuan;
    } catch (e: any) {
      const code = apiErrorCode(e);
      // Kode 2FA diperlukan/salah: tampilkan kolom kode, jangan lempar ke pesan umum.
      if (code === 'two_factor_required' || code === 'two_factor_invalid') {
        setPerlu2fa(true);
        setTotp('');
        setErr(code === 'two_factor_invalid' ? t('auth.totpWrong') : t('auth.totpNeeded'));
        setBusy(false);
        if (turnstileSiteKey) setTurnstileGeneration((current) => current + 1);
        return;
      }
      const feedback = code === 'email_invalid'
        ? t('auth.emailInvalid')
        : code === 'invalid_credentials'
          ? t('auth.badCreds')
          : code === 'email_registered'
            ? t('auth.emailRegistered')
            : pesanError(e, t('auth.fail'));
      setErr(feedback);
      if (turnstileSiteKey) setTurnstileGeneration((current) => current + 1);
      setBusy(false);
    }
  };

  const judul = mode === 'masuk' ? t('auth.hIn') : t('auth.hUp');
  const kicker = mode === 'masuk' ? t('auth.kickerIn') : t('auth.kickerUp');
  return (
    <div className="min-h-dvh bg-grid pb-10 pt-16 sm:pb-16 sm:pt-20">
      <div className="absolute right-4 top-4 z-10 sm:right-6 sm:top-6"><LanguageMenu /></div>
      <main className="mx-auto grid max-w-6xl items-start gap-6 px-4 sm:px-6 lg:grid-cols-[.9fr_1.1fr] lg:items-center">
        {/* Panel brand */}
        <aside className="spotlight card relative overflow-hidden p-6 sm:p-8 lg:min-h-[34rem] lg:flex lg:flex-col lg:justify-between lg:p-10">
          <div>
            <Link href="/" className="mb-5 inline-flex items-center gap-2 rounded text-sm text-mute hover:text-cyan-300 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60">
              <ArrowLeft className="h-4 w-4" aria-hidden="true" /> {t('auth.backHome')}
            </Link>
            <div className="flex items-center gap-3 text-lg font-semibold tracking-tight">
            <img src="/node2mint-icon.ico" alt="" aria-hidden="true" className="h-8 w-8 rounded-lg object-contain" />
            Node2Mint
          </div>
          </div>
          <div className="mt-8 lg:mt-0">
            <h2 className="metal-text max-w-[22rem] text-3xl font-medium leading-[1.05] tracking-[-.03em] sm:text-4xl">
              {t('auth.brandTitle')}
            </h2>
            <p className="mt-4 max-w-[24rem] text-sm leading-relaxed text-mute sm:text-base">
              {t('auth.brandSub')}
            </p>
            <p className="mt-6 inline-flex items-center gap-2 font-mono text-xs uppercase tracking-[.16em] text-cyan-200">
              <ShieldCheck className="h-4 w-4" aria-hidden="true" />
              {t('auth.brandFoot')}
            </p>
          </div>
        </aside>

        {/* Form */}
        <Panel className="p-6 sm:p-8">
          <p className="label">{kicker}</p>
          <h1 className="mt-2 text-2xl font-medium tracking-tight sm:text-3xl">{judul}</h1>
          <p className="mt-2 text-sm text-mute">
            {mode === 'masuk' ? t('auth.noAcc') : t('auth.hasAcc')}
            <button
              type="button"
              onClick={() => { setMode(mode === 'masuk' ? 'daftar' : 'masuk'); setErr(''); }}
              className="rounded font-medium text-cyan-300 hover:text-cyan-400 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60"
            >
              {mode === 'masuk' ? t('auth.toUp') : t('auth.toIn')}
            </button>
          </p>

          <form onSubmit={handleSubmit} className="mt-6 space-y-5" aria-busy={busy} noValidate={false}>
            {mode === 'daftar' && (
              <div className="space-y-2">
                <label htmlFor="full-name" className="label">{t('auth.fullName')}</label>
                <div className="relative">
                  <UserRound className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-mute" aria-hidden="true" />
                  <Input id="full-name" className="pl-10" autoComplete="name" placeholder={t('auth.fullNamePh')} value={fullName} onChange={(e) => setFullName(e.target.value)} required minLength={2} maxLength={121} />
                </div>
              </div>
            )}
            <div className="space-y-2">
              <label htmlFor="email" className="label">{t('auth.email')}</label>
              <div className="relative">
                <Mail className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-mute" aria-hidden="true" />
                <Input id="email" className="pl-10" type="email" autoComplete="email" placeholder="example@email.com" value={email} onChange={(e) => { setEmail(e.target.value); setEmailTouched(true); }} onBlur={() => setEmailTouched(true)} required aria-invalid={!!emailError} aria-describedby={emailError ? 'email-error' : 'auth-status'} />
              </div>
              {emailError && <p id="email-error" role="alert" className="text-xs text-danger">{emailError}</p>}
            </div>
            <div className="space-y-2">
              <div className="flex items-center justify-between gap-3">
                <label htmlFor="password" className="label">{t('auth.pass')}</label>
                {mode === 'masuk' && (
                  <Link href="/lupa-password" className="rounded text-xs text-cyan-300 hover:text-cyan-400 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60">
                    {t('auth.forgot')}
                  </Link>
                )}
              </div>
              <div className="relative">
                <KeyRound className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-mute" aria-hidden="true" />
                <Input id="password" className="pl-10 pr-11" type={showPass ? 'text' : 'password'} autoComplete={mode === 'masuk' ? 'current-password' : 'new-password'} placeholder={mode === 'masuk' ? t('auth.passIn') : t('auth.passUp')} value={pass} onChange={(e) => { setPass(e.target.value); setPassTouched(true); }} onBlur={() => setPassTouched(true)} required minLength={8} maxLength={72} aria-invalid={!!passwordError} aria-describedby={passwordError ? 'password-error' : 'auth-status'} />
                <button type="button" onClick={() => setShowPass((show) => !show)} aria-label={showPass ? t('auth.hidePass') : t('auth.showPass')} className="absolute right-1 top-1/2 grid h-9 w-9 -translate-y-1/2 place-items-center rounded-lg text-mute hover:text-cyan-300">
                  {showPass ? <EyeOff className="h-4 w-4" aria-hidden="true" /> : <Eye className="h-4 w-4" aria-hidden="true" />}
                </button>
              </div>
              {passwordError && <p id="password-error" role="alert" className="text-xs text-danger">{passwordError}</p>}
            </div>

            {/* 2FA: hanya muncul setelah backend meminta kode. autofocus supaya user
                bisa langsung mengetik kode dari aplikasi authenticator. */}
            {mode === 'masuk' && perlu2fa && (
              <div className="space-y-2">
                <label htmlFor="totp" className="label">{t('auth.totpLabel')}</label>
                <div className="relative">
                  <ShieldCheck className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-mute" aria-hidden="true" />
                  <Input
                    id="totp"
                    className="pl-10 font-mono tracking-[0.3em]"
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    placeholder="123456"
                    maxLength={6}
                    value={totp}
                    autoFocus
                    onChange={(e) => setTotp(e.target.value.replace(/\D/g, '').slice(0, 6))}
                    aria-describedby="auth-status"
                  />
                </div>
                <p className="text-xs text-mute">{t('auth.totpHint')}</p>
              </div>
            )}

            {mode === 'daftar' && (
              <>
                <div className="space-y-2">
                  <label htmlFor="password-confirm" className="label">{t('auth.passConfirm')}</label>
                  <div className="relative">
                    <KeyRound className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-mute" aria-hidden="true" />
                    <Input id="password-confirm" className="pl-10 pr-11" type={showConfirm ? 'text' : 'password'} autoComplete="new-password" placeholder={t('auth.passUp')} value={passConfirm} onChange={(e) => { setPassConfirm(e.target.value); setConfirmTouched(true); }} onBlur={() => setConfirmTouched(true)} required minLength={8} maxLength={72} aria-invalid={!!confirmError} aria-describedby={confirmError ? 'password-confirm-error' : 'auth-status'} />
                    <button type="button" onClick={() => setShowConfirm((show) => !show)} aria-label={showConfirm ? t('auth.hidePass') : t('auth.showPass')} className="absolute right-1 top-1/2 grid h-9 w-9 -translate-y-1/2 place-items-center rounded-lg text-mute hover:text-cyan-300">
                      {showConfirm ? <EyeOff className="h-4 w-4" aria-hidden="true" /> : <Eye className="h-4 w-4" aria-hidden="true" />}
                    </button>
                  </div>
                  {confirmError && <p id="password-confirm-error" role="alert" className="text-xs text-danger">{confirmError}</p>}
                </div>
                <div className="space-y-2">
                  <label htmlFor="referral-code" className="label">{t('auth.refOptional')}</label>
                  <div className="relative">
                    <Gift className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-mute" aria-hidden="true" />
                    <Input id="referral-code" className="pl-10" autoComplete="off" value={aff} onChange={(e) => setAff(e.target.value.toUpperCase())} placeholder="REFCODE" maxLength={64} />
                  </div>
                </div>
              </>
            )}

            <TurnstileBox key={turnstileGeneration} siteKey={turnstileSiteKey} onToken={setTurnstileToken} />

            {/* Satu live region untuk error dan status: pembaca layar tidak
                pernah melewatkan kegagalan submit maupun progres pendaftaran. */}
            <div id="auth-status" aria-live="polite">
              {err && (
                <p role="alert" className="flex items-start gap-2 rounded-lg border border-danger/30 bg-danger/10 px-3 py-2 text-sm text-danger">
                  <AlertCircle className="mt-1 h-4 w-4 shrink-0" aria-hidden="true" />
                  <span>{err}</span>
                </p>
              )}
              {ok && (
                <p className="flex items-start gap-2 rounded-lg border border-emerald-300/30 bg-emerald-400/10 px-3 py-2 text-sm text-emerald-200">
                  <CheckCircle2 className="mt-1 h-4 w-4 shrink-0" aria-hidden="true" />
                  <span>{mode === 'masuk' ? t('auth.registered') : t('auth.busy')}</span>
                </p>
              )}
            </div>

            <Button type="submit" disabled={busy} className="h-11 w-full bg-cyan-300 text-[#071015] hover:bg-cyan-200">
              {busy && <span aria-hidden="true" className="h-4 w-4 shrink-0 animate-spin rounded-full border-2 border-[#071015]/30 border-t-[#071015]" />}
              {busy ? t('auth.busy') : mode === 'masuk' ? t('auth.submitIn') : t('auth.submitUp')}
            </Button>
          </form>
          <OAuthButtons
            aff={aff}
            turnstileSiteKey={turnstileSiteKey}
            turnstileToken={turnstileToken}
            onTurnstileSiteKey={setTurnstileSiteKey}
            onError={setErr}
            onConsumed={() => { setTurnstileToken(''); if (turnstileSiteKey) setTurnstileGeneration((current) => current + 1); }}
          />
        </Panel>
      </main>
    </div>
  );
}
