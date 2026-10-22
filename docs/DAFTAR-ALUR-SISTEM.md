# Alur keseluruhan sistem ai-sell-modern

Gateway: 123 rute · Web: 42 halaman · Sumber: gateway/cmd/gateway/main.go + pemindaian panggilan API per halaman.

## Publik/landing

| Halaman | Endpoint dipanggil | Komponen ber-endpoint |
|---|---|---|
| `/` | `/articles`, `/market/featured?per_page=20`, `/models`, `/packages`, `/status`, `/ticker` | — |
| `/pricing` | `/models`, `/packages`, `/status` | — |
| `/docs` | `/models` | — |
| `/bantuan` | `/admin/monitors`, `/articles?page=${page}`, `/models`, `/status` | — |
| `/artikel` | `/articles?page=${page}` | — |
| `/artikel/[slug]` | `/articles/${encodeURIComponent(slug)}` | — |
| `/status` | `/admin/monitors`, `/models`, `/status` | — |
| `/market` | `/market/categories`, `/market/products?${p}`, `/market/reviews` | — |
| `/market/[store]/[slug]` | `/cart`, `/market/products` | — |

## Auth

| Halaman | Endpoint dipanggil | Komponen ber-endpoint |
|---|---|---|
| `/masuk` | `/auth/providers` | — |
| `/lupa-password` | `/auth/forgot` | — |
| `/reset-password` | `/auth/reset` | — |
| `/auth/callback` | — | — |

## Dashboard & akun

| Halaman | Endpoint dipanggil | Komponen ber-endpoint |
|---|---|---|
| `/dashboard` | `/affiliate/links`, `/analytics?range=${range}&scope=${scope}`, `/dashboard`, `/keys/usage`, `/market/orders`, `/store` | — |
| `/api-keys` | `/keys`, `/keys/${id}`, `/keys/${id}/reveal`, `/keys/usage` | — |
| `/chat` | `/api/chat`, `/models` | — |
| `/pengaturan` | `/account/2fa`, `/account/2fa/disable`, `/account/2fa/enable`, `/account/2fa/setup`, `/account/payout-accounts`, `/account/payout-accounts/${id}`, `/account/profile`, `/account/sessions`, `/account/sessions/${id}` | — |
| `/wallet` | `/account/profile`, `/redeem`, `/referral`, `/reward/daily`, `/wallet`, `/wallet/payment-methods`, `/wallet/stripe/checkout`, `/wallet/topup`, `/wallet/transactions?page=${page}`, `/wallet/withdrawals` | — |
| `/reward` | — | — |
| `/redeem` | — | — |
| `/referral` | `/referral` | — |
| `/afiliasi` | `/affiliate/links`, `/market/products?affiliate=true&per_page=100` | — |

## Belanja

| Halaman | Endpoint dipanggil | Komponen ber-endpoint |
|---|---|---|
| `/keranjang` | `/cart`, `/cart/${id}` | — |
| `/payment` | `/market/checkout`, `/market/orders`, `/packages/${encodeURIComponent(id)}/buy`, `/wallet/stripe/checkout` | — |
| `/pesanan` | `/market/orders/${id}/${apa}`, `/market/orders/${id}/items`, `/market/orders?page=${page}` | — |
| `/pesanan/[id]/chat` | `/market/orders/${orderId}/chat`, `/market/orders?page=1` | — |

## Penjual

| Halaman | Endpoint dipanggil | Komponen ber-endpoint |
|---|---|---|
| `/toko` | `/api/media`, `/market/categories`, `/market/reviews?store_id=${informasi?.id ?? `, `/store`, `/store/orders?page=${poPage}`, `/store/products`, `/store/products/${id}`, `/store/products/${id}/boost`, `/store/verify` | — |
| `/tulis-artikel` | `/api/media`, `/articles` | — |

## Admin

| Halaman | Endpoint dipanggil | Komponen ber-endpoint |
|---|---|---|
| `/admin/artikel` | `/admin/articles` | — |
| `/admin/audit` | `/admin/audit` | — |
| `/admin/health` | `/admin/health` | — |
| `/admin/models` | `/admin/models`, `/admin/models/${m.id}`, `/admin/provider/${p.id}/check`, `/admin/providers` | — |
| `/admin/notifications` | `/admin/notifications`, `/admin/notifications/${id}` | — |
| `/admin/orders` | `/admin/orders`, `/admin/orders/${id}/refund` | — |
| `/admin/packages` | `/admin/packages`, `/admin/packages/${p.id}` | — |
| `/admin` | `/admin/summary` | — |
| `/admin/pengaturan` | `/admin/settings`, `/admin/ticker`, `/admin/ticker/${id}`, `/api/media` | — |
| `/admin/products` | `/admin/products` | — |
| `/admin/providers` | `/admin/provider/${p.id}/check`, `/admin/provider/toggle`, `/admin/providers`, `/admin/providers/${editId}/keys`, `/admin/providers/${editId}/keys/${key.id}`, `/admin/providers/${p.id}/keys`, `/admin/providers/check` | — |
| `/admin/uptime` | `/admin/uptime` | — |
| `/admin/users` | `/admin/users/${u.id}/ban`, `/admin/users/${u.id}/unban`, `/admin/users?page=${page}` | — |
| `/admin/wallet` | `/admin/topup/${id}/${approve ? `, `/admin/wallet`, `/admin/withdrawals/${id}/${approve ? ` | — |

## Catatan alur uang vs kuota

- Dompet = uang real (top-up, belanja marketplace, komisi afiliasi, penarikan). Token = kuota API dari paket token.
- Pembelian produk/keranjang/paket selalu melewati `/payment` → endpoint settlement di gateway (Dompet atau Stripe).
- Referral membayar token (pemberi 2,5 juta + 1 hari, invited 2,5 juta + paket awal) dan komisi 10% token, maksimal 2 top-up per invitee.
- Afiliasi membayar komisi uang ke dompet dari pembelian produk.

## Rute gateway tanpa pemanggil di web (dipakai API/SDK pihak ketiga atau admin tooling)

- `/api/admin/redeem`
- `/api/admin/redeem/{id}`
- `/api/admin/stores/{id}`
- `/api/admin/topup/{id}/{action}`
- `/api/admin/withdrawals/{id}/{action}`

- `/api/auth/google`
- `/api/auth/google/callback`
- `/api/auth/login`
- `/api/auth/logout`
- `/api/auth/register`
- `/api/auth/telegram`
- `/api/healthz`
- `/api/notifications`
- `/api/notifications/unread`
- `/api/notifications/{id}/read`
- `/api/playground/execute`
- `/api/v1/chat`
- `/api/webhooks/stripe`
- `/healthz`
- `/v1/chat/completions`
- `/v1/key`
- `/v1/messages`
- `/v1/models`
- `X-Forwarded-Proto`

## Verifikasi lintas rute

- Pemindaian mekanis: 94 endpoint dipanggil web → **0 tanpa rute gateway** (tidak ada tautan mati ke endpoint yang tidak ada).
- Halaman dengan 0 endpoint = halaman yang hanya merender komponen bersama (endpoint tercatat di baris komponen).
