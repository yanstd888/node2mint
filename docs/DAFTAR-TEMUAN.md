# DAFTAR-TEMUAN — AI SELL

Tanggal verifikasi: 2026-09-26  
Status: source, DB-backed tests, active gateway, and public HTTP paths reviewed. Tidak ada commit/push.

## Fixed

| Severity | Temuan | Perbaikan dan bukti |
|---|---|---|
| Critical | Image gateway stale membuat `/api/store/wallet/transfer` 404 walau source sudah punya route. | Gateway dibuild/recreate dari source terbaru. Health `200`; route transfer tanpa sesi mencapai middleware dan mengembalikan `401`, bukan `404`. |
| Critical | Escrow dapat berhadapan dengan dispute/confirm, payout storeless dapat menutup order tanpa penerima, dan dispute seller-wins dapat menggantung. | Lock order+dispute, hold check di transaksi, storeless reject, seller-wins atomic settle, batch cursor drain. DB test escrow lulus. |
| Critical | Buyer-wins refund berisiko melepas item instan/restock pada order yang sudah `COMPLETED`; buyer kosong dapat kehilangan dana. | `completed_at` menjadi guard settlement; refund wajib memiliki store dan buyer; DB escrow test lulus dan fixture bersih. |
| High | OAuth/social signup tidak menyimpan IP signup dan tidak memberikan entitlement signup yang konsisten; referral IP kosong fail-open. | Hash IP pada OAuth, grant hanya akun baru/IP belum dipakai, referral fail-closed. OAuth/referral DB tests lulus. |
| High | Checkout/order belum retry-safe dan tabel idempotency menolak scope marketplace. | Product/cart order memakai `Idempotency-Key`, replay/conflict intent; scope `market_order` ditambahkan ke schema + DB aktif. Replay/conflict DB test lulus. |
| High | JWT yang diterbitkan pada detik sama dapat memiliki session token identik. | Random `jti` 128-bit ditambahkan; auth regression test lulus. |
| High | Admin model UI mengirim 1–3 type/`REASONING`, tetapi backend dan CHECK hanya menerima satu subset. | Normalisasi 1–3 type unik, opsi UI disamakan, CHECK dan migration diperbarui; unit/DB validation lulus. |
| Medium | Admin tidak dapat menghapus redeem code dari UI. | Tombol hapus + konfirmasi + feedback EN/ID ditambahkan memakai endpoint yang sudah ada. TypeScript lulus. |
| Medium | Admin dispute tidak melihat timestamp/respons seller dan label `Store` hardcoded. | Field API dan UI ditambahkan; label masuk dictionary EN/ID. TypeScript lulus. |

## Residual / accepted risk

| Severity | Temuan | Batasan / tindak lanjut |
|---|---|---|
| Medium | Audit log ditulis setelah transaksi uang/admin commit. | Kegagalan audit tidak membatalkan mutasi. Atomic audit memerlukan memindahkan 29 call site ke transaksi pemiliknya. |
| Medium | `fraud_event`/`fraud_flag` ditulis dan notifikasi admin dikirim, tetapi tidak ada event reader khusus; `fraud_flag` belum menjadi hard block. | Review masih melalui notifikasi admin. Tambahkan endpoint/UI dan keputusan policy sebelum memblokir akun. |
| Medium | `ADMIN_IP_ALLOWLIST` dan `v1_origin_denylist` tersedia tetapi unset di deployment aktif. | Jangan menebak CIDR/origin. Isi melalui deployment policy ketika alamat yang diizinkan sudah diputuskan. |
| Medium | Full authenticated browser E2E belum dijalankan. | Tidak ada login secret yang diminta/disimpan. Protected behavior hanya diklaim berdasarkan source review dan DB-backed tests; public HTTP probes lulus. |
| Low | Bad article/store slug masih soft-404; client image compression belum ada. | Tidak memengaruhi money-path. Tambahkan saat route not-found dan upload UX diprioritaskan. |
| Low | Hardcoded-label i18n sweep dan legal review formal belum selesai. | Terms/privacy/refund memakai yurisdiksi Indonesia sebagai draft produk, bukan nasihat hukum. |

## Verification gates

- `go test -count=1 ./...` — pass.
- `go vet ./...` — pass.
- DB-backed escrow, OAuth/referral, transfer, dan idempotency replay/conflict tests — pass; fixture cleanup diverifikasi (`users=2`, `stores=2`, `orders=7`, `products=16`, `ledger=52`, seed balances tetap `admin=50000`, `seller=10000`).
- `node node_modules/typescript/bin/tsc --noEmit -p .` — pass.
- `node scripts/audit/i18n-shadow-check.js` — pass.
- Active gateway `/healthz` — `200 ok`; public models — `200`, 5 models; community — `200`.
- Public routes `/guides`, `/faq`, `/changelog`, `/terms`, `/privacy`, `/refund`, `/toko`, `/payment` — `200`.
- Tidak menjalankan `npm run build`; tidak ada commit atau push.
