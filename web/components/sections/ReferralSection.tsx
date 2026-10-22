'use client';
// ReferralSection: program kode undangan pendaftaran — reward token API,
// bukan saldo dompet. Dipakai halaman /referral dan tab Referral di /wallet.
import { useEffect, useState } from 'react';
import { useFetch } from '@/lib/useApi';
import { Pagination, useClientPagination } from '@/components/ui/Pagination';
import { Button } from '@/components/ui/button';
import { tanggal } from '@/lib/utils';
import { useT } from '@/lib/i18n';
import { useToast } from '@/components/ui/toast';
import { Copy, Check, Users } from 'lucide-react';

type SignupRow = {
  id?: string;
  invitee?: string;
  paid?: boolean;
  created_at?: string;
};

type DataRef = {
  code?: string;
  myCode?: string;
  referral_code?: string;
  total_referrals?: number;
  total_tokens?: number;
  reward_per_signup?: number;
  weekly_count?: number;
  weekly_limit?: number;
  weekly_reset_at?: string | null;
  summary?: { totalInvite?: number; totalTokens?: number; signups?: number };
  signups?: SignupRow[];
};

const fmtToken = (n: number) => `${n.toLocaleString('id-ID')} token`;

export default function ReferralSection() {
  const { t } = useT();
  const { data, loading, err } = useFetch<DataRef>('/referral');
  const toast = useToast();
  const [copied, setCopied] = useState(false);
  const [link, setLink] = useState('');
  const [copiedLink, setCopiedLink] = useState(false);

  const daftar = (data?.signups ?? []).slice().reverse();
  const halamanDaftar = useClientPagination(daftar);
  const kodeKu = data?.code || data?.myCode || data?.referral_code || '';
  const s = data?.summary || {};
  const jumlahDaftar = s.signups ?? s.totalInvite ?? data?.total_referrals ?? 0;
  const totalToken = s.totalTokens ?? data?.total_tokens ?? 0;
  const rewardper = data?.reward_per_signup ?? 0;
  const weeklyCount = data?.weekly_count ?? 0;
  const weeklyLimit = data?.weekly_limit ?? 15;
  const weeklyPct = Math.min(100, Math.round((weeklyCount / weeklyLimit) * 100));

  useEffect(() => {
    if (kodeKu) setLink(`${window.location.origin}/masuk?aff=${kodeKu}`);
  }, [kodeKu]);

  const salin = async () => {
    try {
      await navigator.clipboard.writeText(kodeKu);
      setCopied(true);
      toast.ok(t('ref.copiedOk'));
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // clipboard hanya di secure context; kode tetap terlihat di layar.
      toast.err(t('ref.copyFail'));
    }
  };

  return (
    <div className="max-w-3xl">
      {err ? (
        <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{err}</p>
      ) : loading ? (
        <div className="card p-6"><div className="skel h-8 w-48 mb-4" /><div className="skel h-10 w-full" /></div>
      ) : (
        <>
          <section aria-label={t('ref.codeAria')} className="card saldo p-5 mb-6">
            <span className="label">{t('ref.yourCode')}</span>
            <div className="flex flex-wrap items-center gap-3 mt-2">
              <span className="font-mono text-lg text-cyan-300 select-all break-all">{kodeKu || t('ref.unavailable')}</span>
              {kodeKu && (
                <Button size="sm" variant="outline" onClick={salin}>
                  {copied ? <Check className="w-4 h-4" aria-hidden="true" /> : <Copy className="w-4 h-4" aria-hidden="true" />}
                  {copied ? t('sel.copied') : t('sel.copy')}
                </Button>
              )}
            </div>
            {link && (
              <div className="mt-4 space-y-2">
                <span className="label">{t('ref.inviteLink')}</span>
                <div className="flex flex-wrap items-center gap-2">
                  <input readOnly value={link}
                    className="flex-1 min-w-0 h-11 rounded-lg border border-border bg-surface-hi px-3 text-base font-mono text-mute select-all focus:outline-none md:h-9 md:text-xs"
                    onFocus={(e) => e.currentTarget.select()} aria-label={t('ref.inviteLink')} />
                  <Button size="sm" variant="outline"
                    onClick={async () => {
                      try {
                        await navigator.clipboard.writeText(link);
                        setCopiedLink(true);
                        toast.ok(t('ref.copiedOk'));
                        setTimeout(() => setCopiedLink(false), 2000);
                      } catch {
                        // clipboard gagal: link tetap terlihat di kotak.
                        toast.err(t('ref.copyFail'));
                      }
                    }}>
                    {copiedLink ? <Check className="w-4 h-4" aria-hidden="true" /> : <Copy className="w-4 h-4" aria-hidden="true" />}
                    {copiedLink ? t('sel.copied') : t('ref.copyLink')}
                  </Button>
                </div>
                <p className="text-xs text-mute" dangerouslySetInnerHTML={{ __html: t('ref.shareHint') }} />
              </div>
            )}
          </section>

          <section aria-label={t('ref.rulesAria')} className="card p-5 mb-6 space-y-3 text-sm leading-6 text-mute">
            <h2 className="font-semibold text-text">{t('ref.rulesH')}</h2>
            <p>{t('ref.rule1')}</p>
            <p>{t('ref.rule2')}</p>
            <p>{t('ref.rule3')}</p>
          </section>

          <section className="card p-5 mb-6" aria-label={t('ref.weeklyAria')}>
            <div className="flex items-center justify-between gap-4 text-sm">
              <span className="font-medium">{t('ref.weekly')}</span>
              <span className="tabular-nums">{weeklyCount}/{weeklyLimit}</span>
            </div>
            <div className="mt-3 h-2 overflow-hidden rounded-full bg-surface-hi" role="progressbar" aria-valuemin={0} aria-valuemax={weeklyLimit} aria-valuenow={weeklyCount}>
              <div className="h-full rounded-full bg-cyan-400" style={{ width: `${weeklyPct}%` }} />
            </div>
            <p className="mt-2 text-xs text-mute">{weeklyCount >= weeklyLimit ? t('ref.weeklyFull') : t('ref.weeklyHint')}</p>
          </section>

          <section aria-label={t('ref.summaryAria')} className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
            {[
              [t('ref.invites'), String(jumlahDaftar)],
              [`${t('ref.totalTokens')} ${rewardper > 0 ? t('ref.perPerson', { n: rewardper }) : ''}`, fmtToken(totalToken)],
              [t('ref.pendingTopup'), fmtToken(Math.max(0, jumlahDaftar * rewardper - totalToken))],
            ].map(([label, nilai]) => (
              <div key={label} className="card saldo p-5">
                <span className="label">{label}</span>
                <span className="block mt-2 text-xl font-semibold tabular-nums">{nilai}</span>
              </div>
            ))}
          </section>

          <section aria-label={t('ref.historyAria')}>
            <h2 className="text-lg font-semibold tracking-tight mb-4">{t('ref.history')}</h2>
            {daftar.length === 0 ? (
              <div className="card p-10 text-center">
                <Users className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />
                <p className="font-medium">{t('ref.noInvites')}</p>
                <p className="text-sm text-mute mt-1">{t('ref.noInvitesP')}</p>
              </div>
            ) : (
              <>
                <ul className="card zebra">
                  {halamanDaftar.items.map((s, i) => (
                    <li key={s.id ?? i} className="p-4 flex items-center justify-between gap-4">
                      <div>
                        <span className="text-xs text-mute rounded-full border border-border px-2 py-1">
                          {s.paid ? t('ref.tokensReceived') : t('ref.pendingTopup')}
                        </span>
                        <p className="text-xs text-mute mt-1">{tanggal(s.created_at)}</p>
                      </div>
                      <span className="text-sm tabular-nums text-cyan-300">{s.invitee || ' — '}</span>
                    </li>
                  ))}
                </ul>
                {daftar.length > 10 && <Pagination page={halamanDaftar.page} hasMore={halamanDaftar.hasMore} onPage={halamanDaftar.setPage} />}
              </>
            )}
          </section>
        </>
      )}
    </div>
  );
}
