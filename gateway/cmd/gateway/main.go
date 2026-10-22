package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"ai-sell/gateway/internal/apilog"
	"ai-sell/gateway/internal/auth"
	"ai-sell/gateway/internal/handler"
	"ai-sell/gateway/internal/provider"
	"ai-sell/gateway/internal/rate"
	"ai-sell/gateway/internal/secret"
	"ai-sell/gateway/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()

	// Kunci lemah = token bisa dipalsukan. Diperiksa di sini, bukan di init(),
	// supaya paket auth tetap bisa diimpor oleh test.
	if err := auth.Validate(); err != nil {
		log.Fatal(err)
	}
	// Sama seperti JWT_SECRET: tanpa kunci enkripsi, kunci provider baru & 2FA
	// tidak bisa disimpan aman. Gagal cepat, jangan jalankan setengah-setengah.
	if err := secret.Validate(); err != nil {
		log.Fatal("secret:", err)
	}
	if err := handler.ValidateTurnstileConfig(); err != nil {
		log.Fatal("turnstile:", err)
	}
	if err := handler.ValidateOAuthConfig(); err != nil {
		log.Fatal("oauth:", err)
	}

	// 1. DB.
	db, err := store.Connect(ctx)
	if err != nil {
		log.Fatal("db:", err)
	}
	defer db.Close()

	// 2. Redis.
	// ponytail: password Redis dari env REDIS_PASSWORD; kosong = tanpa auth
	// supaya stack dev lama (redis baru, belum di-restart) tetap jalan.
	rdb := redis.NewClient(&redis.Options{
		Addr:     getenv("REDIS_ADDR", "localhost:6379"),
		Password: os.Getenv("REDIS_PASSWORD"),
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("redis:", err)
	}
	defer rdb.Close()

	// Rate limiter configs.
	loginWindow := 15 * time.Minute
	loginLimit := 10 // 10 percobaan login per IP per 15 menit
	apiWindow := time.Hour
	apiLimit := 3000 // 3000 request per user per jam; landing + dashboard polling cepat menghabiskan 600
	pubLimit := 1200 // 1200 request publik per IP per jam: landing memantau /status & /models berkala
	chatLimit := 20  // 20 chat request per user per jam
	// Keadaan "rate limit mati" tidak boleh senyap: kalau ini menyala di
	// produksi, login bisa di-brute-force tanpa hambatan.
	if getenv("RATE_LIMIT_DISABLE", "") == "true" {
		log.Printf("[WARN] rate limit DIMATIKAN oleh RATE_LIMIT_DISABLE=true — hanya untuk dev")
	}

	// 3. Provider registry.
	providers, err := provider.LoadFromDB(ctx, db)
	if err != nil {
		log.Fatal("providers:", err)
	}

	// 4. Router.
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)
	r.Use(corsMiddleware)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	// Route dikelompokkan per tingkat akses supaya penjaga (auth/peran) ditulis
	// sekali per kelompok, bukan diulang di tiap baris.
	registerPublicRoutes(r, db, rdb, pubLimit, apiWindow)
	registerAuthRoutes(r, db, rdb, loginLimit, loginWindow)
	registerUserRoutes(r, db, providers, rdb, apiLimit, chatLimit, apiWindow)
	registerAdminRoutes(r, db, providers)
	registerAPIKeyRoutes(r, db, providers, rdb, apiLimit, apiWindow)

	addr := getenv("PORT", "8080")
	// Uptime Kuma: push status/latensi gateway tiap 10 menit (mati kalau token kosong),
	// plus jadwal pemeriksaan monitor DB tiap 10 menit supaya /status tidak "unknown".
	handler.StartKumaPusher("http://localhost:"+addr, 10*time.Minute)
	handler.StartUptimeScheduler(db, 10*time.Minute)
	// Status per model (board ala AI-gateway di /status): probe provider aktif.
	handler.StartModelHealthProbe(db, 10*time.Minute)
	handler.StartOrderReconciler(db, time.Minute)
	handler.StartCryptoWatcher(db, time.Minute)
	handler.StartDokuReconciler(db, time.Minute)
	handler.StartEmailDispatcher(db, time.Minute)
	handler.StartSellerTelegramDispatcher(db, time.Minute)
	// Perpanjang otomatis langganan verified yang jatuh tempo (potong dompet).
	handler.StartSubscriptionReconciler(db, time.Minute)
	handler.StartTelegramMembershipRecheck(db, 12*time.Hour)
	handler.StartAccountDeletionSweeper(db, time.Hour)
	log.Println("gateway on :" + addr)
	log.Fatal(http.ListenAndServe(":"+addr, r))
}

// Publik: katalog, media, artikel, dan klik afiliasi (pengunjung belum tentu
// punya akun). Rate limit per IP supaya endpoint tanpa sesi tidak bisa dipakai
// membanjiri server (klik afiliasi menulis baris DB tiap panggilan).
func registerPublicRoutes(r chi.Router, db handler.DB, rdb *redis.Client, limit int, window time.Duration) {
	r.Group(func(r chi.Router) {
		r.Use(rateLimitByIP(rdb, limit, window))
		r.Handle("/api/healthz", rateLimitBurst()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })))
		// Webhook pembayaran: publik, diverifikasi tanda tangan provider.
		r.Post("/api/webhooks/stripe", handler.StripeWebhook(db))
		r.Post("/api/webhooks/doku", handler.DokuWebhook(db))
		r.Post("/api/telegram/webhook", handler.TelegramWebhook(db))
		r.Get("/api/market/products", handler.ListProducts(db))
		r.Get("/api/market/featured", handler.HomeFeatured(db))
		r.Get("/api/market/products/{id}", handler.GetProduct(db))
		r.Get("/api/market/categories", handler.ListCategories(db))
		r.Get("/api/market/reviews", handler.ListReviews(db))
		r.Get("/api/media/{id}", handler.ServeMedia(db))
		r.Get("/api/ticker", handler.TickerList(db))
		r.Get("/api/models", handler.PublicModels(db))
		// Katalog kompatibel OpenAI yang dapat ditemukan tanpa API key.
		r.Handle("/models", rateLimitBurst()(handler.ListModelsV1(db)))
		r.Handle("/api/status", rateLimitBurst()(handler.PublicStatus(db)))
		r.Handle("/api/waitlist", rateLimitBurst()(handler.JoinWaitlist(db)))
		r.Get("/api/public/community", handler.PublicCommunitySettings(db))
		r.Get("/api/market/banners", handler.PublicMarketBanners(db))
		r.Get("/api/articles", handler.ListArticles(db))
		r.Get("/api/articles/{slug}", handler.GetArticle(db))
		r.Get("/api/packages", handler.Packages(db))
		r.Get("/api/store/{id}", handler.PublicStore(db))
		r.Get("/api/affiliate/links/{code}/click", handler.AffiliateClick(db))
		// Metadata provider dibaca UI sebelum login; bukan percobaan autentikasi.
		r.Get("/api/auth/providers", handler.AuthProviders())
	})
}

// Auth publik — rate limited by IP.
func registerAuthRoutes(r chi.Router, db handler.DB, rdb *redis.Client, limit int, window time.Duration) {
	r.Group(func(r chi.Router) {
		r.Use(handler.PublicIPAllowlist())
		r.Use(rateLimitByIP(rdb, limit, window))
		r.Post("/api/auth/login", handler.Login(db))
		r.Post("/api/auth/register", handler.Register(db))
		r.Post("/api/auth/forgot", handler.ForgotPassword(db))
		r.Post("/api/auth/reset", handler.ResetPassword(db))
		// Login sosial: Google OAuth + widget Telegram. Provider tanpa
		// kredensial dilaporkan nonaktif oleh /api/auth/providers.
		r.Post("/api/auth/oauth/start", handler.OAuthStart)
		r.Get("/api/auth/google", handler.GoogleStart)
		r.Get("/api/auth/google/callback", handler.GoogleCallback(db))
		r.Get("/api/auth/telegram", handler.TelegramStart)
		r.Get("/api/auth/telegram/callback", handler.TelegramCallback(db))
	})

	// Endpoint transaksi (checkout/order) ada di grup user di bawah: route itu
	// butuh uid, jadi tidak boleh ditempatkan di kelompok publik ini.
}

// Protected — RequireAuth db-aware: token yang sesinya sudah dicabut ditolak.
func registerUserRoutes(r chi.Router, db handler.DB, providers *provider.Registry, rdb *redis.Client, apiLimit, chatLimit int, window time.Duration) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth(db))
		r.Use(rateLimitByUser(rdb, apiLimit, window))

		// Akun & sesi.
		r.Post("/api/auth/logout", handler.Logout(db))
		r.Get("/api/dashboard", handler.Dashboard(db))
		r.Get("/api/dashboard/usage", handler.DashboardUsage(db))
		r.Get("/api/analytics", handler.Analytics(db))
		r.Get("/api/account/profile", handler.Profile(db))
		r.Get("/api/account/export", handler.AccountExport(db))
		r.Post("/api/account/delete", handler.AccountDeleteRequest(db))
		r.Patch("/api/account/profile", handler.UpdateProfile(db))
		r.Patch("/api/account/locale", handler.UpdateAccountLocale(db))
		r.Post("/api/account/payout-accounts", handler.CreatePayoutAccount(db))
		r.Delete("/api/account/payout-accounts/{id}", handler.DeletePayoutAccount(db))
		r.Get("/api/account/sessions", handler.Sessions(db))
		r.Delete("/api/account/sessions/{id}", handler.DeleteSession(db))
		r.Post("/api/account/2fa/setup", handler.Setup2FA(db))
		r.Get("/api/account/2fa", handler.TwoFAStatus(db))
		r.Post("/api/account/2fa/enable", handler.Enable2FA(db))
		r.Post("/api/account/2fa/disable", handler.Disable2FA(db))

		// Kunci API.
		r.Get("/api/keys", handler.ListKeys(db))
		r.Get("/api/keys/usage", handler.KeyUsageStats(db))
		r.Post("/api/keys", handler.CreateKey(db))
		r.Patch("/api/keys/{id}", handler.UpdateKeyStatus(db))
		r.Delete("/api/keys/{id}", handler.RevokeKey(db))
		r.Delete("/api/keys/{id}/permanent", handler.DeleteKey(db))
		r.Get("/api/keys/{id}/reveal", handler.RevealKey(db))

		// Chat playground (JWT). Rate limit lebih ketat: tiap panggilan berbiaya.
		r.Group(func(r chi.Router) {
			r.Use(rateLimitByUser(rdb, chatLimit, window))
			r.Get("/api/chat/history", handler.ChatHistoryList(db))
			r.Post("/api/chat/history", handler.ChatHistoryCreate(db))
			r.Post("/api/chat", handler.Chat(db, providers))
			// Playground = alias jalur chat JWT (halaman lama memanggil ini).
			r.Post("/api/playground/execute", handler.Chat(db, providers))
		})

		// Dompet.
		r.Get("/api/wallet", handler.Wallet(db))
		r.Get("/api/wallet/transactions", handler.WalletTransactions(db))
		r.Get("/api/wallet/payment-methods", handler.PaymentMethods(db))
		r.Post("/api/wallet/topup", handler.Topup(db))
		r.Get("/api/wallet/withdrawals", handler.Withdrawals(db))
		r.Post("/api/wallet/withdrawals", handler.CreateWithdrawal(db))
		// Top-up otomatis via payment gateway.
		r.Post("/api/wallet/stripe/checkout", handler.StripeCheckout(db))
		r.Post("/api/wallet/doku/checkout", handler.DokuCheckout(db))
		r.Get("/api/wallet/doku/pending", handler.DokuPending(db))
		r.Get("/api/wallet/doku/{invoiceNumber}", handler.DokuStatus(db))
		r.Post("/api/wallet/crypto/onchain", handler.CryptoOnchainInvoice(db))
		r.Get("/api/wallet/crypto/{id}", handler.CryptoStatus(db))
		r.Get("/api/wallet/crypto/{id}/qr", handler.CryptoQRCode(db))

		// Kode redeem (port new-api): tukar kode → saldo.
		r.Post("/api/redeem", handler.Redeem(db))

		// Paket token. GET publik (pricing anonim); beli tetap wajib login.
		r.Post("/api/packages/{id}/buy", handler.BuyPackage(db))

		// Hadiah & referral.
		r.Get("/api/reward/daily", handler.DailyRewardStatus(db))
		r.Post("/api/reward/daily", handler.ClaimDailyReward(db))
		r.Get("/api/kyc", handler.KYCStatus(db))
		r.Post("/api/kyc", handler.KYCSubmit(db))
		r.Get("/api/telegram/status", handler.TelegramStatus(db))
		r.Post("/api/telegram/link", handler.TelegramLink(db))
		r.Post("/api/telegram/verify", handler.TelegramVerify(db))
		r.Get("/api/seller/telegram/status", handler.SellerTelegramStatus(db))
		r.Post("/api/seller/telegram/link", handler.SellerTelegramLink(db))
		r.Post("/api/seller/telegram/unlink", handler.SellerTelegramUnlink(db))
		r.Get("/api/gate/check", handler.TelegramGateCheck(db))
		r.Get("/api/referral", handler.Referral(db))

		// Notifikasi dashboard selalu discoping ke user dari sesi.
		r.Get("/api/notifications", handler.ListNotifications(db))
		r.Get("/api/notifications/unread", handler.UnreadNotificationCount(db))
		r.Post("/api/notifications/{id}/read", handler.MarkNotificationRead(db))
		r.Post("/api/notifications/read-all", handler.MarkAllNotificationsRead(db))

		// Afiliasi.
		r.Get("/api/affiliate/links", handler.AffiliateLinks(db))
		r.Post("/api/affiliate/links", handler.CreateAffiliateLink(db))

		// Keranjang.
		r.Get("/api/cart", handler.Cart(db))
		r.Post("/api/cart", handler.AddToCart(db))
		r.Patch("/api/cart/{id}", handler.UpdateCartItem(db))
		r.Delete("/api/cart/{id}", handler.DeleteCartItem(db))

		// Riwayat lintas marketplace + paket token (tanpa mengubah endpoint chat/escrow marketplace).
		r.Get("/api/purchases", handler.PurchaseHistory(db))

		// Marketplace.
		r.Post("/api/market/orders", handler.CreateOrder(db))
		r.Post("/api/market/checkout", handler.Checkout(db))
		r.Get("/api/market/orders", handler.BuyerOrders(db))
		// Isi barang instan per pesanan: hanya pembelinya (handler yang memeriksa).
		r.Get("/api/market/orders/{id}/items", handler.BuyerOrderItems(db))
		r.Post("/api/market/orders/{id}/confirm", handler.ConfirmOrder(db))
		r.Post("/api/market/orders/{id}/dispute", handler.CreateDispute(db))
		r.Get("/api/market/orders/{id}/dispute", handler.GetDispute(db))
		r.Post("/api/market/orders/{id}/dispute/respond", handler.SellerRespond(db))
		// Chat pesanan: hanya buyer + seller order yang boleh akses.
		r.Get("/api/market/orders/{id}/chat", handler.OrderChatList(db))
		r.Post("/api/market/orders/{id}/chat", handler.OrderChatSend(db))
		r.Post("/api/market/reviews", handler.CreateReview(db))
		r.Get("/api/market/wishlist", handler.ListWishlist(db))
		r.Post("/api/market/wishlist/{id}", handler.AddWishlist(db))
		r.Delete("/api/market/wishlist/{id}", handler.RemoveWishlist(db))

		// Toko seller.
		r.Get("/api/store", handler.GetStore(db))
		r.Get("/api/store/orders", handler.StoreOrders(db))
		r.Post("/api/store/orders/{id}/delivered", handler.MarkOrderDelivered(db))
		r.Get("/api/store/disputes", handler.StoreDisputes(db))
		r.Post("/api/store", handler.OpenStore(db))
		r.Post("/api/store/verify", handler.VerifyStore(db))
		// Langganan verified berulang (perpanjang otomatis dari dompet).
		r.Get("/api/store/subscription", handler.GetSubscription(db))
		r.Post("/api/store/subscription", handler.SetSubscriptionAutoRenew(db))
		r.Post("/api/store/subscription/renew", handler.RenewSubscription(db))
		r.Post("/api/store/wallet/transfer", handler.TransferStoreWallet(db))
		r.Get("/api/store/products", handler.StoreProducts(db))
		r.Post("/api/store/products", handler.CreateStoreProduct(db))
		r.Patch("/api/store/products/{id}", handler.UpdateStoreProduct(db))
		r.Delete("/api/store/products/{id}", handler.DeleteStoreProduct(db))
		// Undo hapus lunak: mengembalikan produk yang baru dihapus seller.
		r.Post("/api/store/products/{id}/restore", handler.RestoreStoreProduct(db))
		r.Post("/api/store/products/{id}/boost", handler.BoostProduct(db))
		r.Post("/api/store/boost-packs/{count}/buy", handler.BuyBoostPack(db))

		// Media.
		r.Post("/api/media", handler.UploadMedia(db))

		// Artikel (kiriman user masuk PENDING).
		r.Post("/api/articles", handler.WriteArticle(db))
	})
}

// Admin.
func registerAdminRoutes(r chi.Router, db handler.DB, providers *provider.Registry) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth(db), auth.RequireAdmin, handler.AdminIPAllowlist())

		r.Get("/api/admin/summary", handler.AdminSummary(db))
		r.Get("/api/admin/users", handler.AdminUsers(db))
		r.Patch("/api/admin/users/{id}/role", handler.AdminSetRole(db))
		r.Post("/api/admin/users/{id}/ban", handler.AdminBanUser(db))
		r.Post("/api/admin/users/{id}/unban", handler.AdminUnbanUser(db))
		r.Delete("/api/admin/users/{id}", handler.AdminDeleteUser(db))
		r.Get("/api/admin/providers", handler.AdminProviders(db))
		// Tambah provider dari UI (parity new-api "add channel").
		r.Post("/api/admin/providers", handler.AdminProviderCreate(db, providers))
		r.Patch("/api/admin/providers/{id}", handler.AdminProviderUpdate(db, providers))
		r.Get("/api/admin/providers/{id}/keys", handler.AdminProviderKeys(db))
		r.Post("/api/admin/providers/{id}/keys", handler.AdminProviderAddKey(db, providers))
		r.Delete("/api/admin/providers/{id}/keys/{keyId}", handler.AdminProviderDeleteKey(db, providers))
		r.Post("/api/admin/providers/{id}/models/import", handler.AdminProviderImportModels(db, providers))
		r.Post("/api/admin/providers/check", handler.CheckProviderDraft(db))
		// Satu handler untuk toggle + ganti kunci + prioritas: tiga aksi pada
		// baris yang sama, jadi tiga route terpisah hanya menambah jalan masuk.
		r.Patch("/api/admin/providers", handler.AdminToggleProvider(db, providers))
		r.Post("/api/admin/provider/toggle", handler.AdminToggleProvider(db, providers))

		r.Get("/api/admin/orders", handler.AdminOrders(db))
		r.Post("/api/admin/orders/{id}/complete", handler.OrderComplete(db))
		// Paksa jalankan auto-cancel order yang overdue (lazy juga di AdminOrders).
		r.Post("/api/admin/orders/reconcile", handler.AdminReconcile(db))

		// Ticker logo landing: dikelola admin.
		r.Get("/api/admin/ticker", handler.AdminTickerList(db))
		r.Get("/api/admin/monitors", handler.AdminMonitors(db))
		r.Post("/api/admin/ticker", handler.AdminTickerCreate(db))
		r.Patch("/api/admin/ticker", handler.AdminTickerUpdate(db))
		r.Delete("/api/admin/ticker/{id}", handler.AdminTickerDelete(db))
		r.Post("/api/admin/orders/{id}/refund", handler.AdminRefundOrder(db))

		r.Get("/api/admin/disputes", handler.DisputeList(db))
		r.Post("/api/admin/disputes/{id}/resolve", handler.ResolveDispute(db))

		r.Get("/api/admin/products", handler.AdminProducts(db))
		r.Patch("/api/admin/products", handler.AdminPatchProduct(db)) // {id} di path: itu yang dipanggil halaman /admin/products.
		r.Patch("/api/admin/products/{id}", handler.AdminPatchProduct(db))
		r.Patch("/api/admin/stores/{id}", handler.AdminPatchStore(db))

		r.Get("/api/admin/wallet", handler.AdminWallet(db))
		// {action} = approve|reject — satu route supaya handler dapat membaca
		// nilainya dari path, bukan ditegmati dari URL.
		r.Post("/api/admin/withdrawals/{id}/{action}", handler.DecideWithdrawal(db))
		r.Get("/api/admin/kyc", handler.AdminKYCList(db))
		r.Get("/api/admin/kyc/{id}/document", handler.AdminKYCDocument(db))
		r.Post("/api/admin/kyc/{id}/{action}", handler.AdminKYCDecision(db))
		r.Post("/api/admin/topup/{id}/{action}", handler.SettleTopup(db))

		// Admin token package CRUD + katalog harga.
		r.Get("/api/admin/packages", handler.AdminPackagesList(db))
		r.Post("/api/admin/packages", handler.AdminPackageCreate(db))
		r.Get("/api/admin/packages/{id}", handler.AdminPackageGet(db))
		r.Patch("/api/admin/packages/{id}", handler.AdminPackageUpdate(db))
		r.Delete("/api/admin/packages/{id}", handler.AdminPackageDelete(db))

		r.Get("/api/admin/uptime", handler.AdminUptimeGet(db))
		r.Post("/api/admin/uptime", handler.AdminUptimePost(db))
		r.Get("/api/admin/health", handler.AdminHealth(db))
		r.Get("/api/admin/audit", handler.AdminAudit(db))
		r.Get("/api/admin/fraud", handler.AdminFraudEvents(db))
		r.Post("/api/admin/fraud/{id}/review", handler.AdminFraudReview(db))
		r.Get("/api/admin/requests", handler.AdminRequestLogs(db))
		r.Get("/api/admin/settings", handler.AdminSettingsGet(db))
		r.Patch("/api/admin/settings", handler.AdminSettingsPatch(db))

		r.Get("/api/admin/articles", handler.AdminArticles(db))
		r.Post("/api/admin/articles", handler.AdminArticleWrite(db))
		r.Patch("/api/admin/articles/{id}", handler.AdminArticlePatch(db))
		r.Delete("/api/admin/articles/{id}", handler.AdminArticleDelete(db))

		// Katalog model + harga per 1k token (port new-api "model management").
		r.Get("/api/admin/models", handler.AdminModelsList(db))
		r.Post("/api/admin/models", handler.AdminModelsCreate(db, providers))
		r.Patch("/api/admin/models/{id}", handler.AdminModelsPatch(db, providers))
		r.Delete("/api/admin/models/{id}", handler.AdminModelsDelete(db))

		// Uji koneksi provider sekali (is_healthy + model_health_log).
		r.Post("/api/admin/provider/{id}/check", handler.AdminProviderCheck(db))

		// Kode redeem: daftar, buat batch, hapus.
		r.Get("/api/admin/redeem", handler.AdminRedeemList(db))
		r.Post("/api/admin/redeem", handler.AdminRedeemCreate(db))
		r.Delete("/api/admin/redeem/{id}", handler.AdminRedeemDelete(db))
		r.Get("/api/admin/notifications", handler.AdminListNotifications(db))
		r.Post("/api/admin/notifications", handler.AdminCreateNotification(db))
		r.Delete("/api/admin/notifications/{id}", handler.AdminDeleteNotification(db))
	})
}

// API v1 — otentikasi via API key (untuk akses programmatic).
// Bentuk endpoint mengikuti OpenAI/Anthropic supaya klien bisa langsung pakai.
func registerAPIKeyRoutes(r chi.Router, db handler.DB, providers *provider.Registry, rdb *redis.Client, limit int, window time.Duration) {
	r.Group(func(r chi.Router) {
		r.Use(handler.V1RequestLog(db), auth.RequireAPIKey(db), auth.RequireScope("chat"), handler.V1OriginDeny(db), rateLimitByAPIKey(rdb, db, limit, window))
		// Semua kegagalan rute programmatic tetap JSON; browser /api mempertahankan
		// respons lamanya agar UI tidak berubah.
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			apilog.SetError(r.Context(), "not_found")
			auth.WriteV1Error(w, http.StatusNotFound, "endpoint not found", "not_found")
		})
		r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
			apilog.SetError(r.Context(), "method_not_allowed")
			auth.WriteV1Error(w, http.StatusMethodNotAllowed, "method not allowed", "method_not_allowed")
		})
		// Path lama tetap jalan (backward-compat).
		r.Post("/api/v1/chat", handler.ChatWithAPIKey(db, providers))
		r.Post("/v1/chat/completions", handler.ChatWithAPIKey(db, providers))
		r.Post("/v1/messages", handler.AnthropicMessages(db, providers))
		r.Get("/v1/models", handler.ListModelsV1(db))
		// Cek sisa kuota memakai key itu sendiri (parity new-api-key-tool).
		r.Get("/v1/key", handler.KeyQuota(db))
	})
}

func getenv(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

// rateLimitByIP: middleware rate limiter berdasarkan alamat IP. Loopback tidak
// dibatasi agar browser dan alat lokal tidak saling menghabiskan jatah publik;
// semua trafik non-loopback tetap memakai bucket Redis yang sama.
func rateLimitByIP(rdb *redis.Client, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if getenv("RATE_LIMIT_DISABLE", "") == "true" {
				next.ServeHTTP(w, r)
				return
			}
			// Loopback TIDAK dikecualikan: di belakang reverse proxy di host yang
			// sama, peer semua trafik internet adalah 127.0.0.1 — pengecualian
			// itu mematikan limiter untuk semua orang tanpa terlihat. Developer
			// yang perlu bebas hambatan memakai RATE_LIMIT_DISABLE.
			ip := handler.RealIP(r)
			key := rate.IPRateKey(ip) + ":" + strconv.Itoa(limit)
			ok, err := rate.Allow(r.Context(), rdb, key, limit, window)
			if err != nil || !ok {
				if !ok {
					rate.SetRateLimitHeaders(w, limit, window)
					auth.WriteBrowserError(w, "too many requests", http.StatusTooManyRequests)
					return
				}
				// Redis error: fail-open sementara untuk dev.
				log.Printf("rate limiter redis error: %v", err)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// rateLimitByUser: middleware rate limiter berdasarkan user_id JWT.
// Harus dipasang setelah auth.RequireAuth.
func rateLimitByUser(rdb *redis.Client, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if getenv("RATE_LIMIT_DISABLE", "") == "true" {
				next.ServeHTTP(w, r)
				return
			}
			claims := auth.UserFromContext(r.Context())
			if claims == nil {
				auth.WriteBrowserError(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			key := rate.UserRateKey(claims.UserID) + ":" + strconv.Itoa(limit)
			ok, err := rate.Allow(r.Context(), rdb, key, limit, window)
			if err != nil || !ok {
				if !ok {
					rate.SetRateLimitHeaders(w, limit, window)
					auth.WriteBrowserError(w, "too many requests", http.StatusTooManyRequests)
					return
				}
				log.Printf("rate limiter redis error: %v", err)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// rateLimitByAPIKey: *** limiter per API key.
// Kuota key (api_key.rate_limit, request/menit) menang atas default global —
// key yang dibatasi ketat memang untuk itu. 0/NULL = pakai default global.
// Harus dipasang setelah auth.RequireAPIKey.
// userV1RateLimit: aggregate cap across all API keys belonging to one user
// on /v1 routes (Task 19). 60 requests/minute = 1/sec sustained.
const userV1RateLimit = 60

func rateLimitByAPIKey(rdb *redis.Client, db handler.DB, fallback int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uid, _ := auth.APIKeyFromContext(r.Context())
			keyID := auth.APIKeyIDFromContext(r.Context())
			if uid == "" || keyID == "" {
				apilog.SetError(r.Context(), "invalid_api_key")
				auth.WriteV1Error(w, http.StatusUnauthorized, "unauthorized", "invalid_api_key")
				return
			}
			// Aggregate cap across all keys belonging to one user.
			userLimit := userV1RateLimit
			userBucket := rate.UserRateKey(uid) + ":v1:" + strconv.Itoa(userLimit)
			userOK, userErr := rate.Allow(r.Context(), rdb, userBucket, userLimit, time.Minute)
			if userErr != nil {
				log.Printf("user rate limiter redis error: %v", userErr)
			}
			if !userOK {
				rate.SetRateLimitHeaders(w, userLimit, time.Minute)
				apilog.SetError(r.Context(), "user_rate_limit_exceeded")
				auth.WriteV1Error(w, http.StatusTooManyRequests, "too many requests", "user_rate_limit_exceeded")
				return
			}
			limit := fallback
			if planLimit, _ := handler.APIRateLimits(r.Context(), db, uid); planLimit > 0 {
				limit = planLimit
			}
			window := time.Minute
			// Bucket per KEY (bukan per user): dua key satu user punya jatah sendiri.
			key := rate.APIKeyRateKey(keyID) + ":" + strconv.Itoa(limit)
			ok, err := rate.Allow(r.Context(), rdb, key, limit, window)
			if err != nil || !ok {
				if !ok {
					rate.SetRateLimitHeaders(w, limit, window)
					apilog.SetError(r.Context(), "rate_limit_exceeded")
					auth.WriteV1Error(w, http.StatusTooManyRequests, "too many requests", "rate_limit_exceeded")
					return
				}
				log.Printf("rate limiter redis error: %v", err)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// securityHeaders: mengatur header keamanan dasar untuk semua response.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		// HSTS hanya di production (HTTPS). Di dev/localhost, skip untuk menghindari masalah.
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware: mengizinkan origin dari env CORS_ORIGIN (default: localhost:5000 untuk dev).
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := getenv("CORS_ORIGIN", "http://localhost:5000")
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// pubBurstLimiter: stdlib token-bucket (1 r/s, burst 3) protecting the hottest
// anonymous endpoints from sub-second spam. Redis fixed-window limiter still
// applies per-IP on the surrounding group for hourly quota.
var pubBurstLimiter = rate.NewTokenBucket(1.0, 3)

func rateLimitBurst() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if getenv("RATE_LIMIT_DISABLE", "") == "true" {
				next.ServeHTTP(w, r)
				return
			}
			if !pubBurstLimiter.Allow() {
				rate.SetRateLimitHeaders(w, 3, time.Second)
				auth.WriteBrowserError(w, "too many requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
