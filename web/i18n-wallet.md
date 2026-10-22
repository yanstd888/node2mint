# i18n Wallet — Implementation Report

## Scope
Translated all visible text in routes `/wallet`, `/payment`, `/keranjang`, `/pesanan`, `/pesanan/[id]/chat`, `/redeem`, `/reward` to `EN ↔ ID` using the existing `useT()` i18n system. No logic or endpoints changed.

## Files Modified
- `web/lib/dict/wallet.ts` — expanded dictionary (EN+ID keys) for `wal.*`, `pay.*`, `ord.*`, `chat.*`.
- `web/app/wallet/page.tsx` — `useT()` introduced; tabs, labels, placeholders, aria-labels, toast messages, status fallback, empty states, history headers.
- `web/app/payment/page.tsx` — `useT()` introduced; labels, aria-labels, summary, receipt, button text, status.
- `web/app/keranjang/page.tsx` — `useT()` introduced; header, empty state, aria-labels, buttons, partial text.
- `web/app/pesanan/page.tsx` — `useT()` introduced; header, empty state, status fallback, product label, quantity, chat button, instant-item controls, confirm toasts.
- `web/app/pesanan/[id]/chat/page.tsx` — `useT()` introduced; back link, error label, heading, empty state, placeholder/aria-label, send button.
- `/redeem` and `/reward` are redirect stubs (`?tab=redeem` / `?tab=reward` → `/wallet`) with no visible text — no changes needed.

Shared component:
- `components/sections/ReferralSection.tsx` — **not modified**. It already uses keys from `seller.ts` (`ref.*`/`sel.*`), which is owned by another agent. Left untouched per the "other dicts off-limits" rule. No `rwd.*` keys added (removed unused ones).

## New Dictionary Keys
### wal.* (wallet page)
Tabs: `wal.tabMain`, `wal.tabRedeem`, `wal.tabReward`, `wal.tabReferral`
Labels: `wal.h`, `wal.tabsAria`, `wal.balanceAria`, `wal.current`, `wal.topupH`, `wal.amount`, `wal.method`, `wal.mTransfer`, `wal.mQris`, `wal.mEwallet`, `wal.methodUnset`, `wal.payTo`, `wal.bank`, `wal.accountNo`, `wal.holder`, `wal.ewalletNo`, `wal.qrisAlt`, `wal.payNote`, `wal.ref`, `wal.refPh`, `wal.manual`, `wal.auto`, `wal.autoTitle`, `wal.withdrawH`, `wal.payout`, `wal.pickAccount`, `wal.noAccounts`, `wal.fee`, `wal.received`, `wal.submitWithdraw`
Toasts: `wal.stripeOk`, `wal.stripeCancel`, `wal.stripeNoUrl`, `wal.stripeDown`, `wal.topupOk`, `wal.topupFail`, `wal.withdrawOk`, `wal.withdrawFail`, `wal.redeemOk`, `wal.redeemFail`, `wal.claimOk`, `wal.claimFail`
Buttons: `wal.sending`, `wal.redirecting`, `wal.redeemCode`, `wal.redeemPh`, `wal.redeemOnce`, `wal.redeemBusy`, `wal.redeemBtn`, `wal.rewardTaken`, `wal.rewardOpen`, `wal.rewardToday`, `wal.streak`, `wal.claiming`, `wal.claimed`, `wal.claim`
History/reward: `wal.historyAria`, `wal.history`, `wal.noTrx`, `wal.noTrxP`, `wal.rewardAria`, `wal.claimHistoryAria`, `wal.claimHistory`

### pay.* (payment page)
`pay.kicker`, `pay.h`, `pay.sub`, `pay.cancel`, `pay.summaryAria`, `pay.order`, `pay.qty`, `pay.failH`, `pay.fail`, `pay.settling`, `pay.settleTimeout`, `pay.mWallet`, `pay.mGateway`, `pay.available`, `pay.payWallet`, `pay.processing`, `pay.card`, `pay.goGateway`, `pay.opening`, `pay.gatewayFail`, `pay.noUrl`, `pay.gatewayMin`, `pay.secure`, `pay.invalid`, `pay.loading`, `pay.receiptKicker`, `pay.receiptH`, `pay.receiptP`, `pay.total`, `pay.method`, `pay.ref`, `pay.time`, `pay.status`, `pay.paid`, `pay.openDash`, `pay.viewOrders`, `pay.back`, `pay.defaultLabel`

### ord.* (orders page)
`ord.h`, `ord.empty`, `ord.emptyP`, `ord.confirmOk`, `ord.confirmFail`, `ord.itemsFail`, `ord.statusUnknown`, `ord.product`, `ord.unit`, `ord.chatAria`, `ord.chat`, `ord.loading`, `ord.hideItems`, `ord.showItems`, `ord.confirm`, `ord.noInstant`, `ord.yourItems`

### cart.* additions
`cart.kicker`, `cart.h`, `cart.browse`, `cart.partial` (the rest already existed)

### chat.* (order chat)
`chat.order`, `chat.h`, `chat.noId`, `chat.back`, `chat.sendFail`, `chat.historyAria`, `chat.empty`, `chat.ph`, `chat.send`

## Verification — `tsc --noEmit`
```
cd '.../web'
node node_modules/typescript/bin/tsc --noEmit
```
Raw output:
```
app/admin/packages/page.tsx(170,17): error TS17002: Expected corresponding JSX closing tag for 'div'.
app/admin/packages/page.tsx(177,9): error TS1005: ')' expected.
app/admin/packages/page.tsx(179,5): error TS1128: Declaration or statement expected.
app/admin/packages/page.tsx(180,3): error TS1109: Expression expected.
app/admin/packages/page.tsx(181,1): error TS1128: Declaration or statement expected.
EXIT:0
```
All 5 errors are pre-existing and located in `app/admin/packages/page.tsx` — **outside the files I own**. Zero errors in any file I modified.

## Runtime render probe
Skipped — `scripts/audit/render.py` (Selenium/Chrome) is not available in this headless Windows environment. The parent verifcation agent is expected to run browser checks.

## Intentionally NOT Translated (backend / dynamic data)
Per the task rules, string values that come from the backend are left as-is so they stay stable across language:
- **Order statuses** (`PENDING`, `PAID`, `PROCESSING`, `CANCELLED`, `REFUNDED`, `LUNAS`-like) — displayed raw from `o.status` / `receipt.method`. Only the *fallback* for an empty status is localized (`ord.statusUnknown`).
- **Product titles** (`o.product?.title`, `it.title`) — backend data, shown verbatim.
- **Store names** (`o.store?.name`) — backend data, shown verbatim.
- **Category names** — backend data, shown verbatim.
- **Bank names / account numbers / holder names / e-wallet providers/numbers** (`tujuan.topup_bank_*`, `tujuan.topup_ewallet_*`, `a.provider`, `a.account_identifier`, `a.account_name`) — financial identifiers from the backend.
- **Payment method labels on receipt** (`receipt.method`: `Dompet`/`Payment Gateway`) — these are the localized key names passed into `finishPurchase(t('pay.mWallet'))` now, but the receipt stores the already-localized string, so it renders consistently with the UI language at purchase time. The raw `label` (order title) and `reference` (order ID) are backend strings, left verbatim.
- **Reward string content** (`2.500.000 token API`, `+1 hari masa aktif`) in `ReferralSection` — lives in `seller.ts` (not mine) and is already i18n'd via `ref.*`.

## Notes / deviations
- `tanggal()` in `lib/utils.ts` is hardcoded to `id-ID`. Left unchanged per "format helpers stay as-is"; only `payment.ts` locale was switched to use `lang` for the receipt timestamp since it's newly formatted there. Other `tanggal()` callers unchanged (out of scope).
- `Redeem`/`Reward` top-level pages are redirect stubs — all their real text lives in `TabRedeem`/`TabReward`, which are now translated.
- The `ReferralSection` component (shared by `/referral` and `/wallet` tab) already uses the i18n system via the `ref.*`/`sel.*` keys owned by another agent; I did not touch it to avoid colliding with that work.

→ skipped: `render.py` runtime probe (no Chrome/Selenium here), add when an agent with the browser audit toolchain runs it.
→ unchanged: `tanggal()` helper's hardcoded `id-ID` locale, add when the utils refactor is in scope.
