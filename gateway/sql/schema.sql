-- Schema: aisell (PostgreSQL)
-- Partisi usage_log per bulan via declarative partitioning (PostgreSQL 10+).
-- Saldo atomik: semua perubahan lewat transaksi, constraint balance >= 0.
-- API key: hanya hash, tidak pernah mentah.
--
-- File ini IDEMPOTEN: tabel dasar pakai IF NOT EXISTS, kolom baru dan tabel baru
-- pakai ALTER ... ADD COLUMN IF NOT EXISTS / CREATE TABLE IF NOT EXISTS supaya
-- database yang sudah jalan ikut naik tanpa reset volume.
-- Jalankan ulang: docker compose exec -T db psql -U aiuser -d aisell -f /docker-entrypoint-initdb.d/01-schema.sql

CREATE TABLE IF NOT EXISTS "user" (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email      TEXT NOT NULL UNIQUE,
  "name"     TEXT,
  role       TEXT NOT NULL DEFAULT 'USER' CHECK (role IN ('USER','SELLER','ADMIN','SUPER_ADMIN')),
  wallet_balance NUMERIC(16,6) NOT NULL DEFAULT 0 CHECK (wallet_balance >= 0),
  password_hash TEXT NOT NULL, -- bcrypt
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS api_key (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  name       TEXT NOT NULL,
  prefix     TEXT NOT NULL,          -- "sk-abcd1234"
  key_hash   TEXT NOT NULL,          -- SHA256, unik
  scopes     TEXT,                   -- CSV: chat,embedding,rerank
  rate_limit INTEGER DEFAULT 60,     -- /menit
  is_active  BOOLEAN NOT NULL DEFAULT true,
  last_used  TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_api_key_hash ON api_key(key_hash); -- cepat patokan lookup

-- Metadata-only request ledger for /v1. Never stores prompt, response, or raw API keys.
CREATE TABLE IF NOT EXISTS api_request_log (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID, key_id UUID,
  model TEXT NOT NULL DEFAULT '', provider TEXT NOT NULL DEFAULT '',
  input_tokens INTEGER NOT NULL DEFAULT 0, output_tokens INTEGER NOT NULL DEFAULT 0,
  cached_tokens INTEGER NOT NULL DEFAULT 0, cost_tokens BIGINT NOT NULL DEFAULT 0,
  latency_ms INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL CHECK (status IN ('success','fail','fallback')),
  error_code TEXT NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_api_request_log_created ON api_request_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_api_request_log_status ON api_request_log(status, created_at DESC);

CREATE TABLE IF NOT EXISTS chat_history (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  title TEXT NOT NULL DEFAULT '', model TEXT NOT NULL DEFAULT '', messages JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_chat_history_user_updated ON chat_history(user_id, updated_at DESC);

ALTER TABLE "user" ADD COLUMN IF NOT EXISTS deletion_requested_at TIMESTAMPTZ;
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS credit_transaction (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  amount        NUMERIC(16,6) NOT NULL,
  type          TEXT NOT NULL,
  status        TEXT NOT NULL DEFAULT 'COMPLETED' CHECK (status IN ('PENDING','COMPLETED','FAILED')),
  description   TEXT NOT NULL,
  reference_id  TEXT,
  environment   TEXT NOT NULL DEFAULT 'production' CHECK (environment IN ('sandbox','production')),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_credit_user ON credit_transaction(user_id);

-- Partisi usage_log per bulan (performance + retention).
CREATE TABLE IF NOT EXISTS usage_log (
  id            UUID NOT NULL DEFAULT gen_random_uuid(),
  user_id       UUID NOT NULL,
  api_key_id    UUID,
  model         TEXT NOT NULL,
  input_tokens  INTEGER NOT NULL,
  output_tokens INTEGER NOT NULL,
  cost          NUMERIC(16,6) NOT NULL,
  status        TEXT NOT NULL CHECK (status IN ('OK','RATE_LIMITED','INSUFFICIENT_CREDIT','ERROR')),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
) PARTITION BY RANGE (created_at);
ALTER TABLE usage_log ADD COLUMN IF NOT EXISTS cache_tokens INTEGER NOT NULL DEFAULT 0;

-- Partisi default + sebulan ke depan (otomatis dibuat di init).
CREATE TABLE IF NOT EXISTS usage_log_default PARTITION OF usage_log DEFAULT;

CREATE TABLE IF NOT EXISTS provider (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name       TEXT NOT NULL,          -- "openai", "anthropic"
  label      TEXT NOT NULL,          -- "OpenAI GPT-4"
  "type"     TEXT NOT NULL,          -- "openai" | "anthropic"
  base_url   TEXT NOT NULL,
  api_key   TEXT NOT NULL,           -- ENCRYPTED (AES-256-GCM, lihat internal/secret)
  is_active  BOOLEAN NOT NULL DEFAULT true,
  priority   INTEGER NOT NULL DEFAULT 0,   -- failover urut priority
  timeout_ms INTEGER NOT NULL DEFAULT 30000,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS provider_api_key (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider_id UUID NOT NULL REFERENCES provider(id) ON DELETE CASCADE,
  label       TEXT NOT NULL DEFAULT '',
  api_key     TEXT NOT NULL, -- ENCRYPTED (AES-256-GCM)
  is_active   BOOLEAN NOT NULL DEFAULT true,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_provider_api_key_active ON provider_api_key(provider_id) WHERE is_active;

COMMENT ON TABLE api_key IS 'hanya hash disimpan; mentah keluar sekali saat create';
COMMENT ON TABLE usage_log IS 'partisi RANGE per bulan; gunakan pg_partman atau CREATE untuk next month';

-- ---------------------------------------------------------------------------
-- ALTER: kolom baru pada tabel yang sudah ada (port dari Prisma lama)
-- ---------------------------------------------------------------------------

-- user: OAuth-only user tidak punya password; tambah email_verified/image/referral_code.
ALTER TABLE "user" ALTER COLUMN password_hash DROP NOT NULL;
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS email_verified TIMESTAMPTZ;
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS image TEXT;
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS referral_code TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_referral_code ON "user"(referral_code) WHERE referral_code IS NOT NULL;
-- Anti-farming: IP pendaftaran di-hash (SHA256+secret), tidak pernah mentah.
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS signup_ip_hash TEXT;
CREATE INDEX IF NOT EXISTS idx_user_signup_ip ON "user"(signup_ip_hash);
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS bio TEXT;
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS preferred_locale TEXT NOT NULL DEFAULT 'id';
ALTER TABLE "user" DROP CONSTRAINT IF EXISTS user_preferred_locale_check;
ALTER TABLE "user" ADD CONSTRAINT user_preferred_locale_check CHECK (preferred_locale IN ('id','en'));
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS kyc_status TEXT NOT NULL DEFAULT 'unverified';
ALTER TABLE "user" DROP CONSTRAINT IF EXISTS user_kyc_status_check;
ALTER TABLE "user" ADD CONSTRAINT user_kyc_status_check CHECK (kyc_status IN ('unverified','pending','verified'));
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS kyc_rejection_reason TEXT;
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS kyc_reviewed_at TIMESTAMPTZ;
CREATE TABLE IF NOT EXISTS kyc_verification (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  doc_type TEXT NOT NULL CHECK (doc_type IN ('ktp','passport')), original_name TEXT NOT NULL,
  mime_type TEXT NOT NULL, encrypted_data TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
  rejection_reason TEXT, reviewed_by UUID, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), reviewed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_kyc_status_created ON kyc_verification(status, created_at DESC);
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS first_name TEXT;
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS last_name TEXT;
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS address TEXT;
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS is_banned BOOLEAN NOT NULL DEFAULT false;
-- Banned: reason + timestamp for audit + login-time notification.
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS banned_reason TEXT;
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS banned_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_user_banned ON "user"(is_banned) WHERE is_banned;

-- Fraud: per-user flag set by velocity / pattern detection. Flagged users are
-- reviewed in the admin panel; is_banned still gates all auth (unchanged).
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS fraud_flag BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS fraud_reason TEXT;
CREATE INDEX IF NOT EXISTS idx_user_fraud ON "user"(fraud_flag) WHERE fraud_flag;

-- credit_transaction: invoiceId/metadata + status REFUNDED/REJECTED.
ALTER TABLE credit_transaction ADD COLUMN IF NOT EXISTS invoice_id TEXT;
ALTER TABLE credit_transaction ADD COLUMN IF NOT EXISTS metadata TEXT;
ALTER TABLE credit_transaction ADD COLUMN IF NOT EXISTS proof_url TEXT;
ALTER TABLE credit_transaction ADD COLUMN IF NOT EXISTS environment TEXT NOT NULL DEFAULT 'production';
ALTER TABLE credit_transaction DROP CONSTRAINT IF EXISTS credit_transaction_environment_check;
ALTER TABLE credit_transaction ADD CONSTRAINT credit_transaction_environment_check CHECK (environment IN ('sandbox','production'));
CREATE INDEX IF NOT EXISTS idx_credit_invoice ON credit_transaction(invoice_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_topup_user_invoice
  ON credit_transaction(user_id, invoice_id) WHERE type='TOPUP';
ALTER TABLE credit_transaction DROP CONSTRAINT IF EXISTS credit_transaction_status_check;
ALTER TABLE credit_transaction ADD CONSTRAINT credit_transaction_status_check
  CHECK (status IN ('PENDING','COMPLETED','FAILED','REFUNDED','REJECTED'));

-- usage_log: model_id + cache_tokens + request_type.
ALTER TABLE usage_log ADD COLUMN IF NOT EXISTS model_id UUID;
ALTER TABLE usage_log ADD COLUMN IF NOT EXISTS cache_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_log ADD COLUMN IF NOT EXISTS request_type TEXT DEFAULT 'CHAT';

-- Migrasi nama kolom lama. Penulis billing dan pembaca statistik sekarang
-- sama-sama memakai cache_tokens; pertahankan data deployment yang sempat
-- menulis cached_tokens sebelum kolom lama dihapus pada migrasi terpisah.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = current_schema()
      AND table_name = 'usage_log'
      AND column_name = 'cached_tokens'
  ) THEN
    EXECUTE 'UPDATE usage_log
             SET cache_tokens = GREATEST(cache_tokens, COALESCE(cached_tokens, 0))
             WHERE cached_tokens > cache_tokens';
  END IF;
END $$;

-- provider: parity dengan UpstreamProvider lama (apiType/modelId/health).
ALTER TABLE provider ADD COLUMN IF NOT EXISTS api_type TEXT NOT NULL DEFAULT 'chat_completions';
ALTER TABLE provider ADD COLUMN IF NOT EXISTS model_id TEXT;
ALTER TABLE provider ADD COLUMN IF NOT EXISTS is_healthy BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE provider ADD COLUMN IF NOT EXISTS last_checked TIMESTAMPTZ;
ALTER TABLE provider ADD COLUMN IF NOT EXISTS last_check_status TEXT;
ALTER TABLE provider ADD COLUMN IF NOT EXISTS last_latency_ms INTEGER;
ALTER TABLE provider ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- ---------------------------------------------------------------------------
-- Marketplace: katalog + toko + kategori
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS market_category (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name       TEXT NOT NULL,
  slug       TEXT NOT NULL UNIQUE,
  icon       TEXT,
  is_active  BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS market_store (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID NOT NULL UNIQUE REFERENCES "user"(id) ON DELETE CASCADE,
  name       TEXT NOT NULL,
  banner     TEXT,
  logo       TEXT,
  description TEXT,
  is_active  BOOLEAN NOT NULL DEFAULT true,    -- toko self-service; admin tetap dapat membekukan
  is_verified BOOLEAN NOT NULL DEFAULT false,  -- badge "Verified Store"; mengatur batas boost harian
  verified_until TIMESTAMPTZ,
  wallet_balance NUMERIC(16,6) NOT NULL DEFAULT 0 CHECK (wallet_balance >= 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE market_store ADD COLUMN IF NOT EXISTS is_verified BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE market_store ADD COLUMN IF NOT EXISTS verified_until TIMESTAMPTZ;
ALTER TABLE market_store ADD COLUMN IF NOT EXISTS boost_credits INTEGER NOT NULL DEFAULT 0 CHECK (boost_credits >= 0);

CREATE TABLE IF NOT EXISTS media_asset (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  file_name  TEXT NOT NULL,
  mime_type  TEXT NOT NULL,
  bytes      INTEGER NOT NULL CHECK (bytes >= 0),
  data       BYTEA,                      -- isi file (dev: bytea, produksi: object storage)
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_media_user ON media_asset(user_id);

-- ---------------------------------------------------------------------------
-- Marketplace: produk, keranjang, boost, ulasan
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS market_product (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  seller_id   UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  title       TEXT NOT NULL,
  description TEXT,
  price       NUMERIC(12,2) NOT NULL CHECK (price >= 0),
  stock       INTEGER NOT NULL DEFAULT 0 CHECK (stock >= 0),
  image_url   TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_product_seller ON market_product(seller_id);

ALTER TABLE market_product ADD COLUMN IF NOT EXISTS store_id UUID REFERENCES market_store(id) ON DELETE CASCADE;
ALTER TABLE market_product ADD COLUMN IF NOT EXISTS category_id UUID REFERENCES market_category(id);
ALTER TABLE market_product ADD COLUMN IF NOT EXISTS media TEXT;               -- JSON array URL
ALTER TABLE market_product ADD COLUMN IF NOT EXISTS is_digital BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE market_product ADD COLUMN IF NOT EXISTS is_instant BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE market_product ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT false; -- approve admin
ALTER TABLE market_product ADD COLUMN IF NOT EXISTS is_demo BOOLEAN NOT NULL DEFAULT false; -- tampilkan tanpa bisa dibeli
ALTER TABLE market_product ADD COLUMN IF NOT EXISTS is_boosted BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE market_product ADD COLUMN IF NOT EXISTS view_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE market_product ADD COLUMN IF NOT EXISTS sales_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE market_product ADD COLUMN IF NOT EXISTS allow_affiliate BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE market_product ADD COLUMN IF NOT EXISTS affiliate_percent NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK (affiliate_percent >= 0 AND affiliate_percent <= 100);
ALTER TABLE market_product ADD COLUMN IF NOT EXISTS pin_order INTEGER;
ALTER TABLE market_product ADD COLUMN IF NOT EXISTS boosted_until TIMESTAMPTZ;
ALTER TABLE market_product ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
-- Hapus lunak seller: baris tetap ada (pesanan/keranjang/link afiliasi/boost/isian
-- tidak ikut terhapus), produk disembunyikan dari publik, POST …/{id}/restore membalikkannya.
ALTER TABLE market_product ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_product_cat ON market_product(category_id);
CREATE INDEX IF NOT EXISTS idx_product_store ON market_product(store_id);
CREATE INDEX IF NOT EXISTS idx_product_active ON market_product(is_active, is_boosted, sales_count DESC);

ALTER TABLE market_product ADD COLUMN IF NOT EXISTS low_stock_threshold INTEGER NOT NULL DEFAULT 3;
ALTER TABLE market_product DROP CONSTRAINT IF EXISTS market_product_low_stock_threshold_check;
ALTER TABLE market_product ADD CONSTRAINT market_product_low_stock_threshold_check CHECK (low_stock_threshold >= 0);
ALTER TABLE market_product ADD COLUMN IF NOT EXISTS low_stock_notified BOOLEAN NOT NULL DEFAULT false;
CREATE INDEX IF NOT EXISTS idx_product_low_stock ON market_product(stock, low_stock_threshold) WHERE is_active;

CREATE TABLE IF NOT EXISTS cart_item (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  product_id UUID NOT NULL REFERENCES market_product(id) ON DELETE CASCADE,
  quantity   INTEGER NOT NULL DEFAULT 1 CHECK (quantity > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, product_id)
);
CREATE INDEX IF NOT EXISTS idx_cart_user ON cart_item(user_id);

CREATE TABLE IF NOT EXISTS market_boost (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id   UUID NOT NULL REFERENCES market_store(id) ON DELETE CASCADE,
  user_id    UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  product_id UUID REFERENCES market_product(id) ON DELETE SET NULL,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  days       INTEGER NOT NULL DEFAULT 7,                    -- durasi boost yang dibayar
  cost       NUMERIC(12,2) NOT NULL DEFAULT 5000,           -- biaya yang dibayar (Rp)
  price      NUMERIC(12,2) NOT NULL DEFAULT 5000   -- Rp5.000/hari
);
ALTER TABLE market_boost ADD COLUMN IF NOT EXISTS days INTEGER NOT NULL DEFAULT 7,
                              ADD COLUMN IF NOT EXISTS cost NUMERIC(12,2) NOT NULL DEFAULT 5000,
                              ADD COLUMN IF NOT EXISTS product_id UUID REFERENCES market_product(id) ON DELETE SET NULL;
ALTER TABLE market_boost ADD COLUMN IF NOT EXISTS paid_at TIMESTAMPTZ;
ALTER TABLE market_boost ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'PURCHASE';
CREATE INDEX IF NOT EXISTS idx_boost_store ON market_boost(store_id);
CREATE INDEX IF NOT EXISTS idx_boost_expires ON market_boost(expires_at);

CREATE TABLE IF NOT EXISTS market_review (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id   UUID NOT NULL REFERENCES market_store(id) ON DELETE CASCADE,
  product_id UUID REFERENCES market_product(id) ON DELETE SET NULL,
  buyer_id   UUID REFERENCES "user"(id) ON DELETE SET NULL,
  buyer_name TEXT,
  rating     INTEGER NOT NULL DEFAULT 5 CHECK (rating BETWEEN 1 AND 5),
  comment    TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_review_store ON market_review(store_id);

CREATE TABLE IF NOT EXISTS market_wishlist (
  user_id    UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  product_id UUID NOT NULL REFERENCES market_product(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, product_id)
);
CREATE INDEX IF NOT EXISTS idx_wishlist_user_created ON market_wishlist(user_id, created_at DESC);

-- ---------------------------------------------------------------------------
-- Marketplace: order (escrow) — kolom lama + kolom port
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS market_order (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  buyer_id     UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  product_id   UUID NOT NULL REFERENCES market_product(id),
  qty          INTEGER NOT NULL CHECK (qty > 0),
  total_price  NUMERIC(12,2) NOT NULL,
  status       TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','COMPLETED','CANCELLED','REFUNDED')),
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_order_buyer ON market_order(buyer_id);

ALTER TABLE market_order ALTER COLUMN buyer_id DROP NOT NULL;      -- guest checkout
ALTER TABLE market_order ADD COLUMN IF NOT EXISTS store_id UUID REFERENCES market_store(id);
ALTER TABLE market_order ADD COLUMN IF NOT EXISTS buyer_email TEXT;
ALTER TABLE market_order ADD COLUMN IF NOT EXISTS buyer_telegram TEXT;
ALTER TABLE market_order ADD COLUMN IF NOT EXISTS buyer_name TEXT;
ALTER TABLE market_order ADD COLUMN IF NOT EXISTS price_at_time NUMERIC(12,2);
ALTER TABLE market_order ADD COLUMN IF NOT EXISTS subtotal NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE market_order ADD COLUMN IF NOT EXISTS fee_amount NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE market_order ADD COLUMN IF NOT EXISTS seller_payout NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE market_order ADD COLUMN IF NOT EXISTS affiliate_code TEXT;
ALTER TABLE market_order ADD COLUMN IF NOT EXISTS affiliate_commission_amount NUMERIC(16,6) NOT NULL DEFAULT 0 CHECK (affiliate_commission_amount >= 0);
ALTER TABLE market_order ADD COLUMN IF NOT EXISTS process_deadline TIMESTAMPTZ;
ALTER TABLE market_order ADD COLUMN IF NOT EXISTS processed_at TIMESTAMPTZ;
ALTER TABLE market_order ADD COLUMN IF NOT EXISTS received_at TIMESTAMPTZ;
ALTER TABLE market_order ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;
ALTER TABLE market_order ADD COLUMN IF NOT EXISTS refunded_at TIMESTAMPTZ;
ALTER TABLE market_order ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
-- status lengkap sesuai alur escrow: buyer bayar -> seller proses -> selesai / refund.
ALTER TABLE market_order DROP CONSTRAINT IF EXISTS market_order_status_check;
ALTER TABLE market_order ADD CONSTRAINT market_order_status_check
  CHECK (status IN ('PENDING','PAID','PROCESSING','COMPLETED','CANCELLED','REFUNDED','DISPUTED'));
CREATE INDEX IF NOT EXISTS idx_order_store ON market_order(store_id);
CREATE INDEX IF NOT EXISTS idx_order_status ON market_order(status, process_deadline);
CREATE INDEX IF NOT EXISTS idx_order_received ON market_order(received_at) WHERE status IN ('PAID','PROCESSING');
CREATE INDEX IF NOT EXISTS idx_order_completed ON market_order(completed_at) WHERE status='COMPLETED';

-- ---------------------------------------------------------------------------
-- Marketplace: dispute / refund
-- ---------------------------------------------------------------------------
-- Buyer mengangkat sengketa (dispute) pada order yang statusnya PAID/PROCESSING,
-- atau hingga 7 hari setelah COMPLETED. Sengketa memindahkan order ke DISPUTED
-- sehingga escrow tidak bisa dilepas ke seller. Admin menyelesaikan:
--   - buyer_wins: kembalikan dana ke pembeli dari saldo toko seller (cek
--     cukup/tidak). Jika saldo toko < subtotal → manual_review (tidak
--     memotong sebagian — kebijakan: tidak ada refund parsial).
CREATE TABLE IF NOT EXISTS market_dispute (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id       UUID NOT NULL REFERENCES market_order(id) ON DELETE CASCADE,
  buyer_id       UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  store_id       UUID REFERENCES market_store(id) ON DELETE SET NULL,
  reason         TEXT NOT NULL CHECK (length(reason) BETWEEN 10 AND 1000),
  status         TEXT NOT NULL DEFAULT 'OPEN'
                 CHECK (status IN ('OPEN','BUYER_WINS','MANUAL_REVIEW','RESOLVED')),
  evidence_url   TEXT,                        -- buyer proof (link to uploaded media)
  resolution     TEXT,                        -- admin note / manual review reason
  seller_response TEXT,                      -- seller's written response to the dispute
  seller_response_at TIMESTAMPTZ,             -- when seller responded
  response_deadline TIMESTAMPTZ,               -- 24h informational seller-response target
  resolved_by    UUID REFERENCES "user"(id) ON DELETE SET NULL,
  resolved_at    TIMESTAMPTZ,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dispute_order ON market_dispute(order_id);
CREATE INDEX IF NOT EXISTS idx_dispute_status ON market_dispute(status);
CREATE INDEX IF NOT EXISTS idx_dispute_buyer ON market_dispute(buyer_id);
CREATE INDEX IF NOT EXISTS idx_dispute_created ON market_dispute(created_at DESC);
ALTER TABLE market_dispute ADD COLUMN IF NOT EXISTS evidence_url TEXT;
ALTER TABLE market_dispute ADD COLUMN IF NOT EXISTS seller_response TEXT;
ALTER TABLE market_dispute ADD COLUMN IF NOT EXISTS seller_response_at TIMESTAMPTZ;
ALTER TABLE market_dispute ADD COLUMN IF NOT EXISTS response_deadline TIMESTAMPTZ;

-- Satu baris = satu barang digital yang hanya boleh dikirim sekali.
-- ditetapkan di transaksi checkout yang sama dengan saldo dan pengurangan stok.
CREATE TABLE IF NOT EXISTS market_product_item (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id UUID NOT NULL REFERENCES market_product(id) ON DELETE CASCADE,
  order_id   UUID REFERENCES market_order(id) ON DELETE SET NULL,
  content    TEXT NOT NULL CHECK (length(content) > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_product_item_available ON market_product_item(product_id) WHERE order_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_product_item_order ON market_product_item(order_id);

-- Isian instan ikut terhapus bersama produknya (CASCADE di CREATE TABLE di
-- atas). Aman karena DeleteStoreProduct menolak menghapus produk yang sudah
-- punya pesanan, jadi barang yang sudah dibayar tidak pernah ikut terhapus.

-- Chat pesanan: terbatas satu order. Hanya buyer + seller toko itu yang bisa
-- baca/kirim; otorisasi di handler (buyer_id/seller_id di-SELECT dan dicek).
CREATE TABLE IF NOT EXISTS market_chat (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id   UUID NOT NULL REFERENCES market_order(id) ON DELETE CASCADE,
  sender_id  UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  message    TEXT NOT NULL CHECK (length(message) > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_chat_order ON market_chat(order_id);
-- Versi awal keliru membatasi satu pesan per order.
ALTER TABLE market_chat DROP CONSTRAINT IF EXISTS market_chat_order_id_key;

CREATE TABLE IF NOT EXISTS notifications (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  type       TEXT NOT NULL CHECK (type IN ('PURCHASE','SALE','ORDER_CHAT','SYSTEM','AFFILIATE_COMMISSION')),
  title      TEXT NOT NULL CHECK (length(title) BETWEEN 1 AND 120),
  message    TEXT NOT NULL CHECK (length(message) BETWEEN 1 AND 2000),
  order_id   UUID REFERENCES market_order(id) ON DELETE CASCADE,
  is_read    BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON notifications(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread ON notifications(user_id) WHERE NOT is_read;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS dispatch_id TEXT;
CREATE INDEX IF NOT EXISTS idx_notifications_dispatch ON notifications(dispatch_id) WHERE dispatch_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_notifications_dispatch_user ON notifications(dispatch_id, user_id) WHERE dispatch_id IS NOT NULL;
-- Tipe AFFILIATE_COMMISSION ditambahkan untuk notif kredit komisi afiliasi ke
-- dompet. CREATE TABLE IF NOT EXISTS tidak mengubah DB lama, jadi CHECK-nya
-- di-drop & dibuat ulang (idempoten) supaya DB lama ikut naik.
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_type_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_type_check
  CHECK (type IN ('PURCHASE','SALE','ORDER_CHAT','SYSTEM','AFFILIATE_COMMISSION'));

-- ---------------------------------------------------------------------------
-- Dompet: topup, penarikan, paket token, langganan, reward
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS token_package (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name          TEXT NOT NULL,
  token_count   INTEGER NOT NULL CHECK (token_count > 0),
  price         NUMERIC(12,2) NOT NULL CHECK (price >= 0),
  duration_days INTEGER NOT NULL DEFAULT 30,
  is_active     BOOLEAN NOT NULL DEFAULT true,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE token_package ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'daily';
ALTER TABLE token_package ADD COLUMN IF NOT EXISTS list_price NUMERIC(12,2);
ALTER TABLE token_package ADD COLUMN IF NOT EXISTS promo_price NUMERIC(12,2);
ALTER TABLE token_package ADD COLUMN IF NOT EXISTS effective_days INTEGER;
ALTER TABLE token_package ADD COLUMN IF NOT EXISTS sort_order INTEGER NOT NULL DEFAULT 0;
ALTER TABLE token_package ADD COLUMN IF NOT EXISTS bonus_tokens INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS user_token_package (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  token_package_id UUID NOT NULL REFERENCES token_package(id),
  expires_at      TIMESTAMPTZ NOT NULL,
  remaining       INTEGER NOT NULL CHECK (remaining >= 0),
  used_amount     INTEGER NOT NULL DEFAULT 0,
  purchased_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE user_token_package ADD COLUMN IF NOT EXISTS purchased_price NUMERIC(12,2);
ALTER TABLE user_token_package ADD COLUMN IF NOT EXISTS effective_expires_at TIMESTAMPTZ;
ALTER TABLE user_token_package ADD COLUMN IF NOT EXISTS parent_package_id UUID;
ALTER TABLE user_token_package ADD COLUMN IF NOT EXISTS grant_source TEXT NOT NULL DEFAULT 'paid';
ALTER TABLE user_token_package ADD COLUMN IF NOT EXISTS requires_telegram BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE user_token_package DROP CONSTRAINT IF EXISTS user_token_package_grant_source_check;
ALTER TABLE user_token_package ADD CONSTRAINT user_token_package_grant_source_check
  CHECK (grant_source IN ('paid','signup','referral','referral_commission','redeem'));
-- Historical grants are classified only by dedicated reward package names.
UPDATE user_token_package u SET grant_source='signup', requires_telegram=true
  FROM token_package p WHERE p.id=u.token_package_id AND lower(p.name)='free - just signup';
UPDATE user_token_package u SET grant_source='referral', requires_telegram=true
  FROM token_package p WHERE p.id=u.token_package_id AND lower(p.name)='reward referral';
UPDATE user_token_package u SET grant_source='redeem', requires_telegram=true
  FROM token_package p WHERE p.id=u.token_package_id AND lower(p.name)='reward redeem';
CREATE INDEX IF NOT EXISTS idx_utp_user ON user_token_package(user_id);
CREATE INDEX IF NOT EXISTS idx_utp_expires ON user_token_package(expires_at);
-- Cegah seed ganda (name+count unik; 'Reward Referral' sah bervariasi token_count).
CREATE UNIQUE INDEX IF NOT EXISTS uq_token_package_name_count ON token_package (lower(name), token_count);

CREATE TABLE IF NOT EXISTS api_token_minute (
  user_id      UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  window_start TIMESTAMPTZ NOT NULL,
  reserved     BIGINT NOT NULL DEFAULT 0 CHECK (reserved >= 0),
  PRIMARY KEY (user_id, window_start)
);
CREATE INDEX IF NOT EXISTS idx_api_token_minute_window ON api_token_minute(window_start);

CREATE TABLE IF NOT EXISTS subscription (
  id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id                UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  tier_id                UUID,
  tier_name              TEXT,
  price                  NUMERIC(12,2) NOT NULL DEFAULT 0,
  status                 TEXT NOT NULL DEFAULT 'INCOMPLETE'
                         CHECK (status IN ('INCOMPLETE','ACTIVE','CANCELED','INCOMPLETE_EXPIRED','PAST_DUE','UNPAID')),
  current_period_start   TIMESTAMPTZ NOT NULL DEFAULT now(),
  current_period_end     TIMESTAMPTZ NOT NULL DEFAULT now(),
  cancel_at_period_end   BOOLEAN NOT NULL DEFAULT false,
  stripe_customer_id     TEXT,
  stripe_subscription_id TEXT,
  created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_sub_user ON subscription(user_id);
-- Auto-renew via wallet: kolom tambahan supaya tabel ini dipakai sungguhan
-- (sebelumnya hanya sisa scaffolding Stripe yang tak pernah dibaca kode).
--   auto_renew   : user memilih diperpanjang otomatis saat habis
--   renew_days   : durasi tiap periode (30 hari untuk Verified Store)
--   renew_price  : harga per periode, dikunci saat user setuju (harga bisa
--                  berubah di platform_setting; yang sudah deal tidak ikut naik)
--   last_renew_at: penanda idempoten — satu periode hanya ditagih sekali
ALTER TABLE subscription ADD COLUMN IF NOT EXISTS auto_renew BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE subscription ADD COLUMN IF NOT EXISTS renew_days INTEGER NOT NULL DEFAULT 30;
ALTER TABLE subscription ADD COLUMN IF NOT EXISTS renew_price NUMERIC(12,2);
ALTER TABLE subscription ADD COLUMN IF NOT EXISTS last_renew_at TIMESTAMPTZ;
ALTER TABLE subscription ADD COLUMN IF NOT EXISTS last_error TEXT;
-- Satu langganan aktif per user per target (mis. verified store). Partial index
-- supaya baris CANCELED/EXPIRED lama tidak memblokir langganan baru.
CREATE UNIQUE INDEX IF NOT EXISTS uq_sub_active_user
  ON subscription(user_id, tier_name) WHERE status = 'ACTIVE';
-- Reconciler memindai langganan yang jatuh tempo + auto-renew aktif.
CREATE INDEX IF NOT EXISTS idx_sub_due
  ON subscription(current_period_end) WHERE auto_renew AND status = 'ACTIVE';

CREATE TABLE IF NOT EXISTS free_tier_quota (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id        UUID NOT NULL UNIQUE REFERENCES "user"(id) ON DELETE CASCADE,
  lifetime_limit INTEGER NOT NULL DEFAULT 50,
  used_amount    INTEGER NOT NULL DEFAULT 0 CHECK (used_amount >= 0)
);

CREATE TABLE IF NOT EXISTS user_topup_accumulation (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         UUID NOT NULL UNIQUE REFERENCES "user"(id) ON DELETE CASCADE,
  total_topup     NUMERIC(16,6) NOT NULL DEFAULT 0,
  milestone_level INTEGER NOT NULL DEFAULT 0,
  claimed_at      TIMESTAMPTZ,
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS daily_reward_claim (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  reward_date TEXT NOT NULL,        -- YYYY-MM-DD (UTC)
  amount      NUMERIC(16,6) NOT NULL,
  claimed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, reward_date)
);

CREATE TABLE IF NOT EXISTS withdrawal (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  amount       NUMERIC(16,6) NOT NULL CHECK (amount > 0),
  status       TEXT NOT NULL DEFAULT 'PENDING'
               CHECK (status IN ('PENDING','APPROVED','REJECTED','FLAGGED')),
  payment_type TEXT,               -- bank | ewallet | crypto
  payout_ref   TEXT,
  flag_reason  TEXT,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Tujuan pencairan (bank/nomor rekening/e-wallet) HARUS ikut tersimpan: admin
-- menyetujui penarikan tanpa informasi ini membayar ke mana pun tanpa bukti.
ALTER TABLE withdrawal ADD COLUMN IF NOT EXISTS destination TEXT;
ALTER TABLE withdrawal ADD COLUMN IF NOT EXISTS fee_amount NUMERIC(16,6) NOT NULL DEFAULT 0 CHECK (fee_amount >= 0);
ALTER TABLE withdrawal ADD COLUMN IF NOT EXISTS net_amount NUMERIC(16,6) NOT NULL DEFAULT 0 CHECK (net_amount >= 0);
CREATE INDEX IF NOT EXISTS idx_wd_user ON withdrawal(user_id);
CREATE INDEX IF NOT EXISTS idx_wd_status ON withdrawal(status);

CREATE TABLE IF NOT EXISTS telegram_verifications (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL UNIQUE REFERENCES "user"(id) ON DELETE CASCADE,
  telegram_user_id BIGINT NOT NULL UNIQUE,
  telegram_username TEXT,
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','revoked')),
  verified_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_at TIMESTAMPTZ,
  last_checked_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_telegram_verifications_active ON telegram_verifications(status, last_checked_at);

CREATE TABLE IF NOT EXISTS telegram_link_token (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  purpose TEXT NOT NULL CHECK (purpose IN ('verify','seller_connect')),
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  result_code TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_telegram_link_pending ON telegram_link_token(user_id, purpose, expires_at) WHERE consumed_at IS NULL;

CREATE TABLE IF NOT EXISTS seller_telegram (
  seller_id              UUID PRIMARY KEY REFERENCES "user"(id) ON DELETE CASCADE,
  telegram_chat_id       BIGINT NOT NULL UNIQUE,
  telegram_username      TEXT,
  notifications_enabled  BOOLEAN NOT NULL DEFAULT true,
  connected_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS telegram_notify_log (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  event_key       TEXT NOT NULL UNIQUE,
  seller_id       UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  type            TEXT NOT NULL CHECK (type IN ('NEW_ORDER','DISPUTE','OUT_OF_STOCK','LOW_STOCK')),
  order_id        UUID REFERENCES market_order(id) ON DELETE SET NULL,
  product_id      UUID REFERENCES market_product(id) ON DELETE SET NULL,
  payload         JSONB NOT NULL DEFAULT '{}'::jsonb,
  status          TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','SENDING','SENT','FAILED')),
  attempts        INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  sent_at         TIMESTAMPTZ,
  last_error      TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_telegram_notify_ready ON telegram_notify_log(status, next_attempt_at, created_at);

CREATE TABLE IF NOT EXISTS payout_account (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id            UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  type               TEXT NOT NULL CHECK (type IN ('bank','ewallet','crypto')),
  provider           TEXT NOT NULL CHECK (length(provider) BETWEEN 2 AND 50),
  account_identifier TEXT NOT NULL CHECK (length(account_identifier) BETWEEN 3 AND 150),
  account_name       TEXT NOT NULL CHECK (length(account_name) BETWEEN 3 AND 100),
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, type, provider, account_identifier)
);
CREATE INDEX IF NOT EXISTS idx_payout_account_user ON payout_account(user_id);

-- ---------------------------------------------------------------------------
-- Crypto payments (Binance Pay + USDT TRC-20)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS crypto_payment (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id               UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  credit_transaction_id UUID NOT NULL UNIQUE REFERENCES credit_transaction(id) ON DELETE CASCADE,
  provider              TEXT NOT NULL CHECK (provider IN ('BINANCE_PAY','TRON_TRC20')),
  status                TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','PAID','EXPIRED','FAILED')),
  asset                 TEXT NOT NULL,
  network               TEXT,
  expected_amount      NUMERIC(24,8) NOT NULL CHECK (expected_amount > 0),
  destination          TEXT,
  provider_order_id     TEXT,
  tx_hash               TEXT,
  expires_at            TIMESTAMPTZ NOT NULL,
  paid_at               TIMESTAMPTZ,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_crypto_provider_order ON crypto_payment(provider, provider_order_id) WHERE provider_order_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_crypto_tx_hash ON crypto_payment(tx_hash) WHERE tx_hash IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_crypto_pending ON crypto_payment(status, expires_at);

-- Midtrans dihapus total saat cutover DOKU (9 Oktober 2026). Baris `midtrans_payment`
-- lama sudah dibuang dan CHECK `MIDTRANS_TOPUP` dikeluarkan dari email_outbox di
-- blok migrasi bawah; tidak ada sisa schema Midtrans di sini.

-- DOKU Checkout: provider invoice and internal top-up remain separate so a
-- browser redirect can never credit the wallet. request_id binds DOKU's status
-- response/notification to the exact create-payment request.
CREATE TABLE IF NOT EXISTS doku_payment (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id               UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  credit_transaction_id UUID NOT NULL UNIQUE REFERENCES credit_transaction(id) ON DELETE CASCADE,
  invoice_number        TEXT NOT NULL UNIQUE CHECK (length(invoice_number) BETWEEN 4 AND 30),
  request_key           TEXT,
  request_id            TEXT NOT NULL UNIQUE,
  checkout_url          TEXT,
  amount                NUMERIC(16,0) NOT NULL CHECK (amount >= 10000 AND amount <= 50000000),
  currency              TEXT NOT NULL DEFAULT 'IDR' CHECK (currency = 'IDR'),
  status                TEXT NOT NULL DEFAULT 'CREATING' CHECK (status IN ('CREATING','PENDING','PAID','FAILED','EXPIRED')),
  environment           TEXT NOT NULL DEFAULT 'sandbox' CHECK (environment IN ('sandbox','production')),
  expires_at            TIMESTAMPTZ NOT NULL,
  paid_at               TIMESTAMPTZ,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_doku_request ON doku_payment(user_id, request_key) WHERE request_key IS NOT NULL;
-- `CREATE TABLE IF NOT EXISTS` tidak memperbarui CHECK/default DB lama.
ALTER TABLE doku_payment DROP CONSTRAINT IF EXISTS doku_payment_status_check;
ALTER TABLE doku_payment ADD CONSTRAINT doku_payment_status_check
  CHECK (status IN ('CREATING','PENDING','PAID','FAILED','EXPIRED'));
ALTER TABLE doku_payment ALTER COLUMN status SET DEFAULT 'CREATING';
DROP INDEX IF EXISTS uq_doku_pending_user;
CREATE UNIQUE INDEX IF NOT EXISTS uq_doku_pending_user ON doku_payment(user_id)
  WHERE status IN ('CREATING','PENDING');
CREATE INDEX IF NOT EXISTS idx_doku_pending ON doku_payment(status, expires_at);

-- Body-only audit trail: no request headers or secret are retained. The unique
-- hash makes webhook retries replay-safe while preserving later state changes.
CREATE TABLE IF NOT EXISTS doku_payment_event (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  payment_id   UUID NOT NULL REFERENCES doku_payment(id) ON DELETE CASCADE,
  source       TEXT NOT NULL CHECK (source IN ('WEBHOOK','RECONCILE')),
  event_status TEXT NOT NULL CHECK (event_status IN ('PENDING','PAID','FAILED','EXPIRED')),
  payload_hash TEXT NOT NULL,
  payload      JSONB NOT NULL,
  received_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (payment_id, source, payload_hash)
);
CREATE INDEX IF NOT EXISTS idx_doku_event_payment ON doku_payment_event(payment_id, received_at);

CREATE TABLE IF NOT EXISTS email_outbox (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  event_key      TEXT NOT NULL UNIQUE,
  event_type     TEXT NOT NULL CHECK (event_type IN ('DOKU_TOPUP','MANUAL_TOPUP','USDT_TRC20_TOPUP','MARKET_ORDER')),
  user_id        UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  reference_id   TEXT NOT NULL,
  payload        JSONB NOT NULL DEFAULT '{}'::jsonb,
  status         TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','SENDING','SENT','FAILED')),
  attempts       INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_error     TEXT,
  provider_message_id TEXT,
  sent_at        TIMESTAMPTZ,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE email_outbox DROP CONSTRAINT IF EXISTS email_outbox_event_type_check;
ALTER TABLE email_outbox ADD CONSTRAINT email_outbox_event_type_check
  CHECK (event_type IN ('DOKU_TOPUP','MANUAL_TOPUP','USDT_TRC20_TOPUP','MARKET_ORDER'));
ALTER TABLE email_outbox ADD COLUMN IF NOT EXISTS provider_message_id TEXT;
ALTER TABLE email_outbox ADD COLUMN IF NOT EXISTS payload JSONB NOT NULL DEFAULT '{}'::jsonb;
CREATE INDEX IF NOT EXISTS idx_email_outbox_ready ON email_outbox(status, next_attempt_at, created_at);


CREATE TABLE IF NOT EXISTS referral_code (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code       TEXT NOT NULL UNIQUE,
  created_by UUID REFERENCES "user"(id) ON DELETE SET NULL,
  "type"     TEXT NOT NULL DEFAULT 'SIGNUP' CHECK ("type" IN ('SIGNUP','TOPUP')),
  max_uses   INTEGER,
  used_count INTEGER NOT NULL DEFAULT 0,
  reward     NUMERIC(16,6) NOT NULL DEFAULT 5000,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS referral_signup (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  inviter_id  UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  invitee_id  UUID NOT NULL UNIQUE REFERENCES "user"(id) ON DELETE CASCADE,
  code        TEXT NOT NULL,
  signup_ip_hash TEXT,                       -- SHA256(ip+JWT_SECRET) — anti-farming, never raw IP
  bonus_given BOOLEAN NOT NULL DEFAULT false,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_refsignup_inviter ON referral_signup(inviter_id);
CREATE INDEX IF NOT EXISTS idx_refsignup_ip ON referral_signup(signup_ip_hash);
-- Max 2 qualifying topups per inviter-invitee pair: counter increments only on
-- paid topups that pass anti-abuse (IP/reusedIP) checks.
ALTER TABLE referral_signup ADD COLUMN IF NOT EXISTS qualifying_topups INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS referral_commission (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  inviter_id     UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  invitee_id     UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  amount         NUMERIC(16,6) NOT NULL,
  source_txn_id  UUID UNIQUE,       -- satu topup = satu komisi
  status         TEXT NOT NULL DEFAULT 'PENDING'
                 CHECK (status IN ('PENDING','FLAGGED','PAID','REJECTED')),
  flag_reason    TEXT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_refcom_inviter ON referral_commission(inviter_id);

-- Idempotency: kolom baru untuk anti-farming referral (harusnya ada di CREATE TABLE di atas,
-- ditambahkan di sini supaya DB lama ikut naik).
ALTER TABLE referral_signup ADD COLUMN IF NOT EXISTS signup_ip_hash TEXT;
CREATE INDEX IF NOT EXISTS idx_refsignup_ip ON referral_signup(signup_ip_hash);

CREATE TABLE IF NOT EXISTS affiliate_link (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id UUID NOT NULL REFERENCES market_product(id) ON DELETE CASCADE,
  user_id    UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  code       TEXT NOT NULL UNIQUE,
  is_active  BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (product_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_afflink_user ON affiliate_link(user_id);

CREATE TABLE IF NOT EXISTS affiliate_click (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  link_id    UUID NOT NULL REFERENCES affiliate_link(id) ON DELETE CASCADE,
  ip_hash    TEXT,
  clicked_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_affclick_link ON affiliate_click(link_id);

CREATE TABLE IF NOT EXISTS affiliate_conversion (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  link_id           UUID NOT NULL REFERENCES affiliate_link(id) ON DELETE CASCADE,
  order_id          UUID NOT NULL UNIQUE REFERENCES market_order(id) ON DELETE CASCADE,
  commission_amount NUMERIC(16,6) NOT NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_affconv_link ON affiliate_conversion(link_id);

-- ---------------------------------------------------------------------------
-- Katalog model + harga (menggantikan harga hardcode di kode)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS ai_model (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name       TEXT NOT NULL UNIQUE,          -- "gpt-4o"
  "type"     TEXT NOT NULL DEFAULT 'TEXT'
             CHECK ("type" ~ '^(TEXT|IMAGE|VIDEO|AUDIO|EMBEDDING|VISION|RERANK|REASONING)(,(TEXT|IMAGE|VIDEO|AUDIO|EMBEDDING|VISION|RERANK|REASONING)){0,2}$'),
  is_active  BOOLEAN NOT NULL DEFAULT true,
  description TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Type dapat menyimpan 1–3 kemampuan dalam urutan kanonik comma-separated.
-- Migrasi ini juga memperbarui database lama yang masih memakai CHECK single-value.
ALTER TABLE ai_model DROP CONSTRAINT IF EXISTS ai_model_type_check;
ALTER TABLE ai_model ADD CONSTRAINT ai_model_type_check
  CHECK ("type" ~ '^(TEXT|IMAGE|VIDEO|AUDIO|EMBEDDING|VISION|RERANK|REASONING)(,(TEXT|IMAGE|VIDEO|AUDIO|EMBEDDING|VISION|RERANK|REASONING)){0,2}$');
ALTER TABLE ai_model ADD COLUMN IF NOT EXISTS context_length INTEGER;        -- mis. 128000
ALTER TABLE ai_model ADD COLUMN IF NOT EXISTS max_output_tokens INTEGER;     -- mis. 4096
ALTER TABLE ai_model ADD COLUMN IF NOT EXISTS sort_weight INTEGER NOT NULL DEFAULT 0;
-- Tagihan token murni: tier FREE/PAID + multiplier per model (mis. 3 = 1 token
-- upstream → 3 token dari saldo user). FREE = kuota harian dulu, habis → saldo.
ALTER TABLE ai_model ADD COLUMN IF NOT EXISTS tier TEXT NOT NULL DEFAULT 'PAID'
  CHECK (tier IN ('FREE','PAID'));
ALTER TABLE ai_model ADD COLUMN IF NOT EXISTS family TEXT;
ALTER TABLE ai_model ADD COLUMN IF NOT EXISTS token_multiplier NUMERIC(8,2) NOT NULL DEFAULT 1.0;

-- Routing eksplisit: satu model publik dapat punya beberapa provider kandidat.
-- Urutan failover mengikuti provider.priority; tiap provider hanya satu upstream per alias.
CREATE TABLE IF NOT EXISTS model_provider_route (
  model_id      UUID NOT NULL REFERENCES ai_model(id) ON DELETE CASCADE,
  provider_id   UUID NOT NULL REFERENCES provider(id) ON DELETE RESTRICT,
  upstream_model TEXT NOT NULL CHECK (length(btrim(upstream_model)) BETWEEN 1 AND 255),
  is_active     BOOLEAN NOT NULL DEFAULT true,
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (model_id, provider_id)
);
-- Upgrade deployment lama yang masih membatasi satu provider per model publik.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
      FROM pg_constraint c
      JOIN pg_class t ON t.oid = c.conrelid
     WHERE t.relname = 'model_provider_route'
       AND c.conname = 'model_provider_route_pkey'
       AND pg_get_constraintdef(c.oid) = 'PRIMARY KEY (model_id)'
  ) THEN
    ALTER TABLE model_provider_route DROP CONSTRAINT model_provider_route_pkey;
    ALTER TABLE model_provider_route ALTER COLUMN model_id SET NOT NULL;
    ALTER TABLE model_provider_route ADD CONSTRAINT model_provider_route_pkey PRIMARY KEY (model_id, provider_id);
  END IF;
END $$;
CREATE INDEX IF NOT EXISTS idx_model_route_provider ON model_provider_route(provider_id);

-- Kuota gratis harian khusus model FREE (satu baris per user per hari).
CREATE TABLE IF NOT EXISTS free_quota (
  user_id UUID NOT NULL,
  day     DATE NOT NULL DEFAULT CURRENT_DATE,
  used    BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (user_id, day)
);

-- Setelan global satu-baris-per-kunci (mis. free_daily_tokens).
CREATE TABLE IF NOT EXISTS app_setting (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
INSERT INTO app_setting (key, value) VALUES ('free_daily_tokens', '20000')
  ON CONFLICT (key) DO NOTHING;

CREATE TABLE IF NOT EXISTS model_badge (
  id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  model_id UUID NOT NULL UNIQUE REFERENCES ai_model(id) ON DELETE CASCADE,
  "type"   TEXT NOT NULL DEFAULT 'NEW',     -- NEW | HOT | CHEAP | PREMIUM
  color    TEXT NOT NULL DEFAULT '#06b6d4',
  icon     TEXT
);

CREATE TABLE IF NOT EXISTS model_pricing (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  model_id          UUID NOT NULL REFERENCES ai_model(id) ON DELETE CASCADE,
  provider          TEXT NOT NULL DEFAULT 'default',
  input_cost_per_1k  NUMERIC(16,6) NOT NULL DEFAULT 0,   -- harga dalam RUPIAH per 1k token
  output_cost_per_1k NUMERIC(16,6) NOT NULL DEFAULT 0,
  is_active         BOOLEAN NOT NULL DEFAULT true,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (model_id, provider)
);
CREATE INDEX IF NOT EXISTS idx_pricing_model ON model_pricing(model_id);

CREATE TABLE IF NOT EXISTS model_health_log (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  model_id    UUID REFERENCES ai_model(id) ON DELETE CASCADE,
  provider_id UUID REFERENCES provider(id) ON DELETE SET NULL,
  status      TEXT NOT NULL CHECK (status IN ('ok','slow','error','down')),
  latency_ms  INTEGER,
  status_code INTEGER,
  checked_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_health_model ON model_health_log(model_id);
CREATE INDEX IF NOT EXISTS idx_health_checked ON model_health_log(checked_at);

-- ---------------------------------------------------------------------------
-- Akun & keamanan
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS user_session (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  session_token TEXT NOT NULL UNIQUE,
  user_id       UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  user_agent    TEXT,
  ip            TEXT,
  expires       TIMESTAMPTZ NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_session_user ON user_session(user_id);

CREATE TABLE IF NOT EXISTS oauth_account (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id             UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  "type"              TEXT NOT NULL DEFAULT 'oauth',
  provider            TEXT NOT NULL,      -- github | discord | telegram
  provider_account_id TEXT NOT NULL,
  refresh_token       TEXT,
  access_token        TEXT,
  expires_at          BIGINT,
  token_type          TEXT,
  scope               TEXT,
  id_token            TEXT,
  session_state       TEXT,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_account_id)
);

CREATE TABLE IF NOT EXISTS verification_token (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  identifier TEXT NOT NULL,          -- email
  token      TEXT NOT NULL UNIQUE,
  purpose    TEXT NOT NULL DEFAULT 'RESET_PASSWORD',  -- RESET_PASSWORD | VERIFY_EMAIL
  expires    TIMESTAMPTZ NOT NULL,
  UNIQUE (identifier, token)
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_verification_token_reset_identifier
  ON verification_token(identifier) WHERE purpose='RESET_PASSWORD';

CREATE TABLE IF NOT EXISTS user_auth_method (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  method     TEXT NOT NULL CHECK (method IN ('totp','email_otp','passkey')),
  secret     TEXT,                  -- terenkripsi AES-256-GCM
  label      TEXT,
  confirmed  BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, method)
);

CREATE TABLE IF NOT EXISTS audit_log (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID REFERENCES "user"(id) ON DELETE SET NULL,
  action     TEXT NOT NULL,
  target     TEXT,
  before     TEXT,
  after      TEXT,
  ip         TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_log(action);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at DESC);

-- ---------------------------------------------------------------------------
-- Konten & operasional
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS article (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  author_id    UUID REFERENCES "user"(id) ON DELETE SET NULL,
  author_role  TEXT NOT NULL DEFAULT 'USER',
  title        TEXT NOT NULL,
  slug         TEXT NOT NULL UNIQUE,
  content      TEXT NOT NULL,
  excerpt      TEXT,
  image        TEXT,
  status       TEXT NOT NULL DEFAULT 'PENDING'
               CHECK (status IN ('PENDING','PUBLISHED','DELETED_LINK','REJECTED')),
  is_admin_post BOOLEAN NOT NULL DEFAULT false,
  published_at TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_article_status ON article(status, published_at DESC);

CREATE TABLE IF NOT EXISTS article_link_violation (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  article_id      UUID NOT NULL REFERENCES article(id) ON DELETE CASCADE,
  detected_pattern TEXT NOT NULL,
  matched_text    TEXT NOT NULL,
  action_taken    TEXT NOT NULL,     -- REJECTED | AUTO_DELETED
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS platform_setting (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS waitlist (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email      TEXT NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_waitlist_created ON waitlist(created_at);

CREATE TABLE IF NOT EXISTS uptime_monitor (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name        TEXT NOT NULL,
  kind        TEXT NOT NULL DEFAULT 'HTTP' CHECK (kind IN ('HTTP','TCP','PRISMA','PROCESS')),
  url         TEXT NOT NULL,
  interval_sec INTEGER NOT NULL DEFAULT 60,
  timeout_ms  INTEGER NOT NULL DEFAULT 8000,
  slow_ms     INTEGER NOT NULL DEFAULT 3000,
  is_active   BOOLEAN NOT NULL DEFAULT true,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS uptime_heartbeat (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  monitor_id  UUID NOT NULL REFERENCES uptime_monitor(id) ON DELETE CASCADE,
  status      TEXT NOT NULL CHECK (status IN ('up','slow','down')),
  latency_ms  INTEGER NOT NULL DEFAULT 0,
  status_code INTEGER,
  message     TEXT,
  checked_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_hb_monitor ON uptime_heartbeat(monitor_id, checked_at DESC);

-- ---------------------------------------------------------------------------
-- Seed katalog dasar (data baru, BUKAN salinan database lama). Idempoten.
-- ---------------------------------------------------------------------------

INSERT INTO market_category (name, slug, icon) VALUES
  ('Soul & Skill',   'soul-skill', ''),
  ('Premiums App',  'premiums-app', ''),
  ('Akun',          'akun', ''),
  ('Tools & Software', 'tools-software', ''),
  ('Ebook',         'ebook', ''),
  ('Others',        'others', '')
ON CONFLICT (slug) DO UPDATE SET name=EXCLUDED.name, is_active=true;

-- Non-aktifkan kategori lama yang diganti (API & Tools) selamanya.
UPDATE market_category SET is_active=false WHERE slug='api-tools';

INSERT INTO ai_model (name, "type", description, context_length) VALUES
  ('gpt-4o',            'TEXT',   'Model flagship multimodal', 128000),
  ('gpt-4o-mini',       'TEXT',   'Murah dan cepat', 128000),
  ('claude-3-5-sonnet', 'TEXT',   'Penalaran panjang', 200000),
  ('text-embedding-3-small', 'EMBEDDING', 'Embedding 1536 dim', 8191)
ON CONFLICT (name) DO NOTHING;

INSERT INTO model_pricing (model_id, provider, input_cost_per_1k, output_cost_per_1k)
SELECT id, 'default',
       CASE name WHEN 'gpt-4o' THEN 3.0 WHEN 'gpt-4o-mini' THEN 0.4
                 WHEN 'claude-3-5-sonnet' THEN 4.0 ELSE 0.02 END,
       CASE name WHEN 'gpt-4o' THEN 9.0 WHEN 'gpt-4o-mini' THEN 1.2
                 WHEN 'claude-3-5-sonnet' THEN 12.0 ELSE 0 END
FROM ai_model WHERE name IN ('gpt-4o','gpt-4o-mini','claude-3-5-sonnet','text-embedding-3-small')
ON CONFLICT (model_id, provider) DO NOTHING;

INSERT INTO token_package (name, token_count, price, duration_days) VALUES
  ('Starter', 100000,  10000, 30),
  ('Pro',     500000,  45000, 30),
  ('Bisnis',  2000000, 150000, 60)
ON CONFLICT DO NOTHING;

UPDATE token_package SET is_active=false WHERE name IN ('Starter','Pro','Bisnis');

-- Daily / monthly package catalog per request:
-- free signup 2M/24h, 5.2M/24h Rp4k, 25M/48h Rp15k, 50M/72h Rp30k, 100M/96h Rp50k, 500M/168h Rp225k
-- monthly 50M/30d Rp60k, 100M Rp120k, 500M Rp500k
UPDATE token_package SET name='Free - Just Signup', token_count=2000000, kind='daily', duration_days=1, price=0 WHERE lower(name)='free - just signup';
INSERT INTO token_package (name, token_count, price, duration_days, kind, list_price) VALUES
  ('Free - Just Signup', 2000000, 0, 1, 'daily', NULL),
  ('5.2M / 24h', 5200000, 4000, 1, 'daily', 4000),
  ('25M / 48h',     25000000, 15000, 2,  'daily',   15000),
  ('50M / 72h',     50000000, 30000, 3,  'daily',   30000),
  ('100M / 96h',    100000000, 50000, 4, 'daily',   50000),
  ('500M / 168h',   500000000, 225000, 7, 'daily',  225000)
ON CONFLICT DO NOTHING;

INSERT INTO token_package (name, token_count, price, duration_days, kind, list_price) VALUES
  ('50M / 30d',     50000000,  60000,  30, 'monthly', 60000),
  ('100M / 30d',    100000000, 120000, 30, 'monthly', 120000),
  ('500M / 30d',    500000000, 500000, 30, 'monthly', 500000)
ON CONFLICT DO NOTHING;

UPDATE token_package SET is_active=false
 WHERE price > 0 AND name NOT IN ('Mini','Pro','Plus','Pro Max');
INSERT INTO token_package
  (name, token_count, bonus_tokens, price, duration_days, kind, list_price, sort_order, is_active)
VALUES
  ('Mini',     10000000,        0,  10000,  3, 'daily',  10000, 10, true),
  ('Pro',      55000000,  5000000,  50000,  7, 'daily',  50000, 20, true),
  ('Plus',    120000000, 20000000, 100000, 14, 'daily', 100000, 30, true),
  ('Pro Max', 250000000, 50000000, 200000, 30, 'monthly',200000,40, true)
ON CONFLICT (lower(name), token_count) DO UPDATE SET
  bonus_tokens=EXCLUDED.bonus_tokens, price=EXCLUDED.price,
  duration_days=EXCLUDED.duration_days, kind=EXCLUDED.kind,
  list_price=EXCLUDED.list_price, sort_order=EXCLUDED.sort_order, is_active=true;

INSERT INTO platform_setting (key, value) VALUES
  ('site_name', 'Node2Mint'),
  ('market_fee_percent', '10'),
  ('referral_commission_percent', '10'),
  ('cached_token_discount', '0.5'),
  ('daily_reward_amount', '100'),
  ('boost_price_per_day', '5000'),
  ('boost_price_one_off', '1000'),
  ('boost_max_verified_per_day', '2'),
  ('verified_store_monthly_price', '15000'),
  ('admin_store_user_email', ''),
  ('withdrawal_min', '50000'),
  ('withdrawal_fee_percent', '5'),
  ('topup_bank_name', ''),
  ('topup_bank_account', ''),
  ('topup_bank_holder', ''),
  ('topup_qris_image_url', ''),
  ('topup_qris_merchant', ''),
  -- Referral reward per undangan (inviter + invitee masing-masing): 1M token.
  ('referral_reward_tokens', '1000000'),
  -- Configurable 25% token usage surcharge (diterapkan SEBELUM multiplier).
  ('usage_surcharge_rate', '0.25'),
  ('maintenance_mode', 'false'),
  -- /v1 origin/referer denylist: comma-separated origins/referrers rejected on
  -- /v1/* API-key routes (scrapers, hotlinkers). Empty = denylist off.
  ('v1_origin_denylist', ''),
  -- Admin IP allowlist: comma-separated CIDR. Empty = semua IP (role-only gate).
  ('admin_ip_allowlist', ''),
  -- Fraud: order velocity. Max orders per user within the window before flagging.
  ('fraud_order_velocity_max', '3'),
  ('fraud_order_velocity_window', '60'),
  -- URL publik footer Community. Kosong = tautan tidak ditampilkan.
  ('community_discord_url', ''),
  ('community_telegram_url', ''),
  ('community_x_url', ''),
  ('community_youtube_url', ''),
  ('market_banners', '[]')
ON CONFLICT (key) DO NOTHING;
DELETE FROM platform_setting WHERE key LIKE 'topup_ewallet_%';

-- ---------------------------------------------------------------------------
-- Port new-api: kode redeem, kuota per API key, model routing per provider
-- ---------------------------------------------------------------------------

-- Kode redeem: tukar kode -> saldo (fitur inti new-api). Satu kode bisa
-- multi-pakai atau sekali pakai; sekali terepakai oleh satu user, selesai.
CREATE TABLE IF NOT EXISTS redeem_code (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code        TEXT NOT NULL UNIQUE,            -- "AISELL-XXXX-XXXX"
  amount      NUMERIC(16,6) NOT NULL CHECK (amount > 0),
  max_uses    INTEGER,                         -- NULL = sekali pakai
  used_count  INTEGER NOT NULL DEFAULT 0,
  created_by  UUID REFERENCES "user"(id) ON DELETE SET NULL,
  redeemed_by UUID REFERENCES "user"(id) ON DELETE SET NULL,  -- NULL = masih aktif (sekali pakai)
  redeemed_at TIMESTAMPTZ,
  expires_at  TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_redeem_code ON redeem_code(code);
ALTER TABLE redeem_code ADD COLUMN IF NOT EXISTS reward_type TEXT NOT NULL DEFAULT 'WALLET'
  CHECK (reward_type IN ('WALLET','API_TOKEN','ACCESS_DAYS','BOOST','VERIFIED_30D'));
ALTER TABLE redeem_code DROP CONSTRAINT IF EXISTS redeem_code_reward_type_check;
ALTER TABLE redeem_code ADD CONSTRAINT redeem_code_reward_type_check
  CHECK (reward_type IN ('WALLET','API_TOKEN','ACCESS_DAYS','BOOST','VERIFIED_30D'));
CREATE TABLE IF NOT EXISTS redeem_claim (
  redeem_code_id UUID NOT NULL REFERENCES redeem_code(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  claimed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (redeem_code_id, user_id)
);

-- Kuota kredit per API key (NULL = tanpa batas; biaya chat mengurangi
-- wallet_balance seperti biasa, credit_limit hanya membatasi key ini).
ALTER TABLE api_key ADD COLUMN IF NOT EXISTS credit_limit NUMERIC(16,6);
ALTER TABLE api_key ADD COLUMN IF NOT EXISTS credit_used  NUMERIC(16,6) NOT NULL DEFAULT 0
  CHECK (credit_used >= 0);
-- Key mentah TERENKRIPSI (AES-256-GCM, internal/secret) supaya pemilik bisa
-- menyalin ulang key lewat tombol copy di /api-keys. NULL = key lama (sebelum
-- fitur ini) yang hanya punya hash: tidak bisa ditampilkan ulang.
ALTER TABLE api_key ADD COLUMN IF NOT EXISTS key_enc TEXT;
-- Whitelist model per API key (port new-api token.models): CSV nama model,
-- '' = boleh semua katalog. Ditegakkan di ChatWithAPIKey/AnthropicMessages.
ALTER TABLE api_key ADD COLUMN IF NOT EXISTS models TEXT NOT NULL DEFAULT '';

-- Routing model per provider (CSV nama model, kosong = melayani semua katalog).
ALTER TABLE provider ADD COLUMN IF NOT EXISTS models TEXT NOT NULL DEFAULT '';

-- Ticker logo landing: pita logo berjalan, dikelola dari dashboard admin.
-- image_url biasanya /api/media/{id} dari upload /api/media.
CREATE TABLE IF NOT EXISTS ticker_logo (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    label       TEXT NOT NULL DEFAULT '',
    image_url   TEXT NOT NULL,
    sort        INTEGER NOT NULL DEFAULT 0,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Provider AI bawaan: asset lokal agar ticker tetap hidup tanpa CDN eksternal.
INSERT INTO ticker_logo (label, image_url, sort, is_active)
SELECT v.label, v.image_url, v.sort, TRUE
FROM (VALUES
    ('Anthropic', '/provider-logos/anthropic.svg', 10),
    ('OpenAI', '/provider-logos/openai.svg', 20),
    ('Gemini', '/provider-logos/gemini.svg', 30),
    ('DeepSeek', '/provider-logos/deepseek.svg', 40),
    ('Meta AI', '/provider-logos/meta.svg', 50),
    ('Mistral AI', '/provider-logos/mistralai.svg', 60),
    ('xAI', '/provider-logos/grok.svg', 70),
    ('Moonshot AI', '/provider-logos/moonshot.svg', 80),
    ('Zhipu AI', '/provider-logos/zai.svg', 90),
    ('Alibaba', '/provider-logos/alibaba.svg', 100),
    ('MiniMax', '/provider-logos/minimax.svg', 110)
) AS v(label, image_url, sort)
WHERE NOT EXISTS (SELECT 1 FROM ticker_logo WHERE ticker_logo.image_url = v.image_url);

-- Tolak NaN di kolom uang. Postgres menerima 'NaN' sebagai NUMERIC, dan
-- CHECK (x >= 0) LOLOS untuk NaN (setiap perbandingan bernilai false, sehingga
-- penolakan `x < 0` pun tidak menangkapnya). Saldo NaN = "tak terbatas" karena
-- semua perbandingan saldo jadi salah arah. Gerbang utamanya di Go (num.Float);
-- ini lapis kedua yang tidak bisa dilewati jalur mana pun.
ALTER TABLE "user" DROP CONSTRAINT IF EXISTS user_wallet_not_nan;
ALTER TABLE "user" ADD CONSTRAINT user_wallet_not_nan CHECK (wallet_balance <> 'NaN'::numeric);
ALTER TABLE market_store DROP CONSTRAINT IF EXISTS market_store_wallet_not_nan;
ALTER TABLE market_store ADD CONSTRAINT market_store_wallet_not_nan CHECK (wallet_balance <> 'NaN'::numeric);
ALTER TABLE credit_transaction DROP CONSTRAINT IF EXISTS credit_amount_not_nan;
ALTER TABLE credit_transaction ADD CONSTRAINT credit_amount_not_nan CHECK (amount <> 'NaN'::numeric);
ALTER TABLE withdrawal DROP CONSTRAINT IF EXISTS withdrawal_amount_not_nan;
ALTER TABLE withdrawal ADD CONSTRAINT withdrawal_amount_not_nan CHECK (amount <> 'NaN'::numeric);
ALTER TABLE market_product DROP CONSTRAINT IF EXISTS market_product_price_not_nan;
ALTER TABLE market_product ADD CONSTRAINT market_product_price_not_nan CHECK (price <> 'NaN'::numeric);
ALTER TABLE usage_log DROP CONSTRAINT IF EXISTS usage_cost_not_nan;
ALTER TABLE usage_log ADD CONSTRAINT usage_cost_not_nan CHECK (cost <> 'NaN'::numeric);
ALTER TABLE market_product DROP CONSTRAINT IF EXISTS market_product_affiliate_not_nan;
ALTER TABLE market_product ADD CONSTRAINT market_product_affiliate_not_nan CHECK (affiliate_percent <> 'NaN'::numeric);
ALTER TABLE market_boost DROP CONSTRAINT IF EXISTS market_boost_cost_not_nan;
ALTER TABLE market_boost ADD CONSTRAINT market_boost_cost_not_nan CHECK (cost <> 'NaN'::numeric AND price <> 'NaN'::numeric);
ALTER TABLE market_order DROP CONSTRAINT IF EXISTS market_order_amounts_not_nan;
ALTER TABLE market_order ADD CONSTRAINT market_order_amounts_not_nan CHECK (
  total_price <> 'NaN'::numeric AND price_at_time <> 'NaN'::numeric AND subtotal <> 'NaN'::numeric
  AND fee_amount <> 'NaN'::numeric AND seller_payout <> 'NaN'::numeric
  AND affiliate_commission_amount <> 'NaN'::numeric
);
ALTER TABLE token_package DROP CONSTRAINT IF EXISTS token_package_prices_not_nan;
ALTER TABLE token_package ADD CONSTRAINT token_package_prices_not_nan CHECK (
  price <> 'NaN'::numeric AND (list_price IS NULL OR list_price <> 'NaN'::numeric)
  AND (promo_price IS NULL OR promo_price <> 'NaN'::numeric)
);
ALTER TABLE user_token_package DROP CONSTRAINT IF EXISTS user_token_package_price_not_nan;
ALTER TABLE user_token_package ADD CONSTRAINT user_token_package_price_not_nan
  CHECK (purchased_price IS NULL OR purchased_price <> 'NaN'::numeric);
ALTER TABLE subscription DROP CONSTRAINT IF EXISTS subscription_price_not_nan;
ALTER TABLE subscription ADD CONSTRAINT subscription_price_not_nan CHECK (price <> 'NaN'::numeric);
ALTER TABLE user_topup_accumulation DROP CONSTRAINT IF EXISTS user_topup_accumulation_not_nan;
ALTER TABLE user_topup_accumulation ADD CONSTRAINT user_topup_accumulation_not_nan CHECK (total_topup <> 'NaN'::numeric);
ALTER TABLE daily_reward_claim DROP CONSTRAINT IF EXISTS daily_reward_amount_not_nan;
ALTER TABLE daily_reward_claim ADD CONSTRAINT daily_reward_amount_not_nan CHECK (amount <> 'NaN'::numeric);
ALTER TABLE withdrawal DROP CONSTRAINT IF EXISTS withdrawal_amounts_not_nan;
ALTER TABLE withdrawal ADD CONSTRAINT withdrawal_amounts_not_nan
  CHECK (amount <> 'NaN'::numeric AND fee_amount <> 'NaN'::numeric AND net_amount <> 'NaN'::numeric);
ALTER TABLE crypto_payment DROP CONSTRAINT IF EXISTS crypto_payment_amount_not_nan;
ALTER TABLE crypto_payment ADD CONSTRAINT crypto_payment_amount_not_nan CHECK (expected_amount <> 'NaN'::numeric);
ALTER TABLE referral_code DROP CONSTRAINT IF EXISTS referral_code_reward_not_nan;
ALTER TABLE referral_code ADD CONSTRAINT referral_code_reward_not_nan CHECK (reward <> 'NaN'::numeric);
ALTER TABLE referral_commission DROP CONSTRAINT IF EXISTS referral_commission_amount_not_nan;
ALTER TABLE referral_commission ADD CONSTRAINT referral_commission_amount_not_nan CHECK (amount <> 'NaN'::numeric);
ALTER TABLE affiliate_conversion DROP CONSTRAINT IF EXISTS affiliate_conversion_amount_not_nan;
ALTER TABLE affiliate_conversion ADD CONSTRAINT affiliate_conversion_amount_not_nan CHECK (commission_amount <> 'NaN'::numeric);
ALTER TABLE ai_model DROP CONSTRAINT IF EXISTS ai_model_multiplier_not_nan;
ALTER TABLE ai_model ADD CONSTRAINT ai_model_multiplier_not_nan CHECK (token_multiplier <> 'NaN'::numeric);
ALTER TABLE model_pricing DROP CONSTRAINT IF EXISTS model_pricing_costs_not_nan;
ALTER TABLE model_pricing ADD CONSTRAINT model_pricing_costs_not_nan
  CHECK (input_cost_per_1k <> 'NaN'::numeric AND output_cost_per_1k <> 'NaN'::numeric);
ALTER TABLE redeem_code DROP CONSTRAINT IF EXISTS redeem_code_amount_not_nan;
ALTER TABLE redeem_code ADD CONSTRAINT redeem_code_amount_not_nan CHECK (amount <> 'NaN'::numeric);
ALTER TABLE api_key DROP CONSTRAINT IF EXISTS api_key_credit_not_nan;
ALTER TABLE api_key ADD CONSTRAINT api_key_credit_not_nan
  CHECK ((credit_limit IS NULL OR credit_limit <> 'NaN'::numeric) AND credit_used <> 'NaN'::numeric);

-- ---------------------------------------------------------------------------
-- Notifikasi dashboard (bell): satu baris per pengguna, selalu discoping oleh
-- user_id di query SELECT maupun UPDATE. target_user_id = NULL berarti "semua
-- user" (broadcast system); sebaliknya hanya untuk user yang disebutkan.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS notification (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       UUID REFERENCES "user"(id) ON DELETE CASCADE,
  actor_id      UUID REFERENCES "user"(id) ON DELETE SET NULL,
  type          TEXT NOT NULL CHECK (type IN ('PURCHASE','SALE','CHAT','SYSTEM','UPDATE')),
  title         TEXT NOT NULL,
  body          TEXT,
  is_read       BOOLEAN NOT NULL DEFAULT false,
  is_broadcast  BOOLEAN NOT NULL DEFAULT false,
  reference_id  TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_notification_user ON notification(user_id, is_read, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notification_broadcast ON notification(is_broadcast, created_at DESC) WHERE is_broadcast;
CREATE INDEX IF NOT EXISTS idx_notification_created ON notification(created_at DESC);

-- ---------------------------------------------------------------------------
-- Fraud: event log for velocity / pattern detection. One row per flagged event;
-- reviewed by admin (user.fraud_flag is the aggregate view).
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS fraud_event (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  kind          TEXT NOT NULL CHECK (kind IN ('ORDER_VELOCITY','LOGIN_VELOCITY','MULTI_ACCOUNT')),
  detail        TEXT NOT NULL,                  -- free-form, e.g. "3 orders in 60s"
  ip            TEXT,                           -- raw peer IP, for abuse investigation
  is_reviewed   BOOLEAN NOT NULL DEFAULT false,
  is_actioned   BOOLEAN NOT NULL DEFAULT false, -- true = user banned after review
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_fraud_user ON fraud_event(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_fraud_pending ON fraud_event(is_reviewed) WHERE NOT is_reviewed;

-- ---------------------------------------------------------------------------
-- Token packages: daily/monthly catalog, duration overwrite/extend semantics,
-- configurable surcharge, and referral reward amounts.
-- ---------------------------------------------------------------------------

-- Paket harian vs bulanan: kategori untuk filter katalog harga.
-- 'daily' = reset 24/48/72/96/168h ; 'monthly' = 30 hari.
ALTER TABLE token_package ADD COLUMN IF NOT EXISTS kind TEXT
  NOT NULL DEFAULT 'monthly'
  CHECK (kind IN ('daily','monthly'));

-- Harga list (Rp) — untuk UI yang menampilkan strikthrough saat promo aktif.
ALTER TABLE token_package ADD COLUMN IF NOT EXISTS list_price NUMERIC(12,2)
  CHECK (list_price >= 0);

-- Promo price (Rp): ketika diisi, UI tunjukkan list_price coret + promo price.
ALTER TABLE token_package ADD COLUMN IF NOT EXISTS promo_price NUMERIC(12,2)
  CHECK (promo_price >= 0);

-- Duration overwrite (hari): ketika NULL → pakai duration_days (semantik lama).
-- Ketika terisi → menimpa masa berlaku (overwrite) sehingga paket murah bisa
-- berlaku lebih lama/kurang lama tanpa tabel terpisah.
ALTER TABLE token_package ADD COLUMN IF NOT EXISTS effective_days INTEGER;
ALTER TABLE token_package ADD COLUMN IF NOT EXISTS sort_order INTEGER NOT NULL DEFAULT 0;

-- user_token_package: paket yang sudah dibeli user mendapatkan kolom tambahan
-- untuk dukung overwrite / extend semantics.
ALTER TABLE user_token_package ADD COLUMN IF NOT EXISTS purchased_price NUMERIC(12,2)
  CHECK (purchased_price >= 0);
ALTER TABLE user_token_package ADD COLUMN IF NOT EXISTS effective_expires_at TIMESTAMPTZ;
-- Saldo token bisa dialokasikan ke paket lama (extend) atau paket baru (overwrite).
-- Kolom ini memudahkan audit: paket yang menjadi sumber/overwriter.
ALTER TABLE user_token_package ADD COLUMN IF NOT EXISTS parent_package_id UUID
  REFERENCES token_package(id) ON DELETE SET NULL;

-- Referral reward: token reward per undangan (inviter + invitee masing-masing).
-- Diubah admin di /api/admin/settings. Default 1M token.
INSERT INTO platform_setting (key, value) VALUES
  ('referral_reward_tokens', '1000000')
ON CONFLICT (key) DO NOTHING;

-- Configurable 25% token usage surcharge (diterapkan SEBELUM multiplier).
INSERT INTO platform_setting (key, value) VALUES
  ('usage_surcharge_rate', '0.25')
ON CONFLICT (key) DO NOTHING;

-- Retry aman untuk operasi bernilai uang/token. response disimpan dalam
-- transaksi yang sama, sehingga klik ulang tidak mendebit atau memberi dua kali.
CREATE TABLE IF NOT EXISTS idempotency_request (
  user_id      UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  scope        TEXT NOT NULL CHECK (scope IN ('package_buy', 'redeem', 'market_order')),
  request_key  TEXT NOT NULL CHECK (char_length(request_key) BETWEEN 8 AND 128),
  request_hash TEXT NOT NULL,
  response     JSONB,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, scope, request_key)
);
CREATE INDEX IF NOT EXISTS idx_idempotency_request_created
  ON idempotency_request(created_at);

-- ============================================================================
-- Migrasi satu-kali: hapus total sisa Midtrans (cutover DOKU, 9 Okt 2026).
-- Aman dijalankan berkali-kali. Baris yang dihapus hanya sandbox/KEDALUWARSA
-- tanpa saldo nyata — diverifikasi sebelum cutover (2 baris, sandbox, PENDING,
-- expires_at sudah lewat, Rp110.000 sandbox). Jejak audit_log TIDAK dihapus.
-- ============================================================================
DO $$
BEGIN
  IF to_regclass('public.midtrans_payment') IS NOT NULL THEN
    DELETE FROM credit_transaction c
     WHERE c.type NOT IN ('TOPUP_MIDTRANS')
       AND EXISTS (SELECT 1 FROM midtrans_payment m WHERE m.credit_transaction_id = c.id);
    DROP TABLE IF EXISTS midtrans_payment;
  END IF;
END $$;

DELETE FROM credit_transaction
 WHERE type = 'TOPUP_MIDTRANS'
   AND status IN ('PENDING', 'FAILED', 'EXPIRED')
   AND COALESCE(reference_id, '') = 'midtrans';
