import './globals.css';
import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { Geist, Geist_Mono } from 'next/font/google';
import { cookies, headers } from 'next/headers';
import { AuthProvider } from '@/lib/auth';
import { LocaleProvider } from '@/lib/i18n';
import { ToastProvider } from '@/components/ui/toast';
import { ConfirmProvider } from '@/components/ui/confirm';
import AppShell from '@/components/AppShell';
import CookieBanner from '@/components/CookieBanner';
import ServiceBanner from '@/components/ServiceBanner';

// CSP nonce dibuat per request di middleware; static HTML tidak bisa membawa nonce itu.
export const dynamic = 'force-dynamic';

const sans = Geist({ variable: '--font-sans', subsets: ['latin'] });
const mono = Geist_Mono({ variable: '--font-mono', subsets: ['latin'] });

// Host produksi: jangan pernah localhost (canonical/OG/JSON-LD memakai ini).
const SITE = (process.env.NEXT_PUBLIC_SITE_URL || 'https://www.node2mint.com').replace(/\/$/, '');

const description = 'Node2Mint combines a multi-model AI API, token packages, a digital product marketplace, affiliate commissions, and referral token rewards in one account.';
const descriptionId = 'Node2Mint menggabungkan API AI multi-model, paket token, pasar produk digital, komisi afiliasi, dan hadiah token rujukan dalam satu akun.';
const PAGE_META: Record<string, { title: string; description: string }> = {
  '/': { title: 'Node2Mint | Multi-Model AI API & Digital Marketplace', description },
  '/pricing': { title: 'AI Token Packages & Model Pricing | Node2Mint', description: 'Compare Node2Mint AI token packages, model access, and transparent pricing for OpenAI-compatible API usage.' },
  '/market': { title: 'Digital Product Marketplace for AI Creators | Node2Mint', description: 'Browse digital prompts, datasets, templates, tools, and services from verified Node2Mint marketplace stores.' },
  '/articles': { title: 'AI API Guides & Marketplace Articles | Node2Mint', description: 'Read Node2Mint articles about multi-model AI APIs, token usage, digital products, and creator commerce.' },
  '/about': { title: 'About Node2Mint | AI API & Digital Marketplace', description: 'Learn how Node2Mint brings multi-model AI access, token usage, and digital creator commerce together in one account.' },
  '/docs': { title: 'OpenAI-Compatible AI API Documentation | Node2Mint', description: 'Integrate Node2Mint multi-model AI API with OpenAI-compatible endpoints, examples, model catalog, and token guidance.' },
  '/guides': { title: 'AI API & Digital Marketplace Guides | Node2Mint', description: 'Learn Node2Mint API setup, token packages, wallet rules, marketplace selling, referral, and affiliate workflows.' },
  '/faq': { title: 'Node2Mint FAQ: AI API, Tokens & Marketplace', description: 'Answers about Node2Mint wallets, AI token packages, API access, marketplace orders, referral, and affiliate rewards.' },
  '/status': { title: 'Node2Mint Service Status & AI Model Health', description: 'Check Node2Mint API service status, model availability, and recent operational updates.' },
  '/help': { title: 'Node2Mint Help Center', description: 'Find Node2Mint help for accounts, AI API access, token packages, wallets, marketplace orders, and security.' },
  '/contact': { title: 'Contact Node2Mint Support', description: 'Contact Node2Mint support for account access, payments, marketplace orders, refunds, and security help.' },

  '/changelog': { title: 'Node2Mint Changelog', description: 'Read Node2Mint service, API, marketplace, payment, and security updates.' },
  '/terms': { title: 'Terms of Use | Node2Mint', description: 'Terms for using the Node2Mint AI API, wallets, token packages, and digital marketplace.' },
  '/privacy': { title: 'Privacy Policy | Node2Mint', description: 'Learn how Node2Mint processes account, API, payment, wallet, and digital marketplace data.' },
  '/refund': { title: 'Refund Policy | Node2Mint', description: 'Rules for marketplace disputes, balance refunds, and Node2Mint API token usage.' },
};
const PAGE_META_ID: Record<string, { title: string; description: string }> = {
  '/': { title: 'Node2Mint | API AI Multi-Model & Pasar Digital', description: descriptionId },
  '/pricing': { title: 'Paket Token AI & Harga Model | Node2Mint', description: 'Bandingkan paket token AI Node2Mint, akses model, dan harga transparan untuk penggunaan API yang kompatibel dengan OpenAI.' },
  '/market': { title: 'Pasar Produk Digital untuk Kreator AI | Node2Mint', description: 'Jelajahi prompt, dataset, templat, alat, dan layanan digital dari toko pasar digital Node2Mint terverifikasi.' },
  '/articles': { title: 'Panduan API AI & Artikel Pasar Digital | Node2Mint', description: 'Baca artikel Node2Mint tentang API AI multi-model, penggunaan token, produk digital, dan perdagangan kreator.' },
  '/about': { title: 'Tentang Node2Mint | API AI & Pasar Digital', description: 'Pelajari cara Node2Mint menyatukan akses AI multi-model, penggunaan token, dan perdagangan kreator digital dalam satu akun.' },
  '/docs': { title: 'Dokumentasi API AI yang Kompatibel dengan OpenAI | Node2Mint', description: 'Integrasikan API AI multi-model Node2Mint dengan endpoint yang kompatibel dengan OpenAI, contoh, katalog model, dan panduan token.' },
  '/guides': { title: 'Panduan API AI & Pasar Digital | Node2Mint', description: 'Pelajari penyiapan API Node2Mint, paket token, aturan dompet, penjualan di pasar digital, rujukan, dan afiliasi.' },
  '/faq': { title: 'FAQ Node2Mint: API AI, Token & Pasar Digital', description: 'Jawaban tentang dompet Node2Mint, paket token AI, akses API, pesanan pasar digital, rujukan, dan hadiah afiliasi.' },
  '/status': { title: 'Status Layanan & Kesehatan Model AI Node2Mint', description: 'Periksa status layanan API Node2Mint, ketersediaan model, dan pembaruan operasional terbaru.' },
  '/help': { title: 'Pusat Bantuan Node2Mint', description: 'Temukan bantuan Node2Mint untuk akun, akses API AI, paket token, dompet, pesanan pasar digital, dan keamanan.' },
  '/contact': { title: 'Hubungi Dukungan Node2Mint', description: 'Hubungi dukungan Node2Mint untuk akses akun, pembayaran, pesanan pasar digital, pengembalian dana, dan bantuan keamanan.' },
  '/changelog': { title: 'Catatan Perubahan Node2Mint', description: 'Lihat pembaruan layanan, API, pasar digital, pembayaran, dan keamanan Node2Mint.' },
  '/terms': { title: 'Syarat Penggunaan | Node2Mint', description: 'Syarat penggunaan layanan API AI, dompet, paket token, dan pasar digital Node2Mint.' },
  '/privacy': { title: 'Kebijakan Privasi | Node2Mint', description: 'Pelajari cara Node2Mint memproses akun, penggunaan API, pembayaran, dompet, dan data pasar digital.' },
  '/refund': { title: 'Kebijakan Pengembalian Dana | Node2Mint', description: 'Aturan sengketa pasar digital, pengembalian saldo, dan penggunaan token API Node2Mint.' },
};

const PRIVATE_META: Record<string, { title: string; description: string }> = {
  '/sign-in': { title: 'Sign In | Node2Mint', description: 'Sign in to manage your Node2Mint API, token packages, wallet, and digital marketplace activity.' },
  '/forgot-password': { title: 'Forgot Password | Node2Mint', description: 'Request a secure Node2Mint password reset link.' },
  '/reset-password': { title: 'Reset Password | Node2Mint', description: 'Set a new password for your Node2Mint account.' },
  '/dashboard': { title: 'Dashboard | Node2Mint', description: 'Manage your Node2Mint API usage, tokens, wallet, and account activity.' },
  '/api-keys': { title: 'API Keys | Node2Mint', description: 'Manage Node2Mint API keys and access settings.' },
  '/pengaturan': { title: 'Settings | Node2Mint', description: 'Manage your Node2Mint account and notification settings.' },
  '/wallet': { title: 'Wallet | Node2Mint', description: 'Manage your Node2Mint balance, token packages, and payment activity.' },
  '/payment': { title: 'Payment | Node2Mint', description: 'Complete and review Node2Mint wallet and marketplace payments.' },
  '/toko': { title: 'Seller Store | Node2Mint', description: 'Manage your Node2Mint digital products, sales, and seller settings.' },
  '/pesanan': { title: 'Orders | Node2Mint', description: 'Review your Node2Mint digital marketplace orders and disputes.' },
  '/chat': { title: 'Chat | Node2Mint', description: 'Message buyers and sellers about Node2Mint marketplace orders.' },
  '/afiliasi': { title: 'Affiliate | Node2Mint', description: 'Review your Node2Mint affiliate links, commissions, and activity.' },
  '/admin': { title: 'Admin | Node2Mint', description: 'Manage Node2Mint users, packages, models, providers, orders, and settings.' },
};
const PRIVATE_META_ID: Record<string, { title: string; description: string }> = {
  '/sign-in': { title: 'Masuk | Node2Mint', description: 'Masuk untuk mengelola API, paket token, dompet, dan aktivitas pasar digital Node2Mint.' },
  '/forgot-password': { title: 'Lupa Kata Sandi | Node2Mint', description: 'Minta tautan aman untuk mengatur ulang kata sandi Node2Mint.' },
  '/reset-password': { title: 'Atur Ulang Kata Sandi | Node2Mint', description: 'Tetapkan kata sandi baru untuk akun Node2Mint Anda.' },
  '/dashboard': { title: 'Dasbor | Node2Mint', description: 'Kelola penggunaan API, token, dompet, dan aktivitas akun Node2Mint.' },
  '/api-keys': { title: 'Kunci API | Node2Mint', description: 'Kelola kunci API dan pengaturan akses Node2Mint.' },
  '/pengaturan': { title: 'Pengaturan | Node2Mint', description: 'Kelola akun dan pengaturan notifikasi Node2Mint.' },
  '/wallet': { title: 'Dompet | Node2Mint', description: 'Kelola saldo, paket token, dan aktivitas pembayaran Node2Mint.' },
  '/payment': { title: 'Pembayaran | Node2Mint', description: 'Selesaikan dan tinjau pembayaran dompet serta pasar digital Node2Mint.' },
  '/toko': { title: 'Toko Penjual | Node2Mint', description: 'Kelola produk digital, penjualan, dan pengaturan penjual Node2Mint.' },
  '/pesanan': { title: 'Pesanan | Node2Mint', description: 'Tinjau pesanan dan sengketa pasar digital Node2Mint.' },
  '/chat': { title: 'Obrolan | Node2Mint', description: 'Kirim pesan kepada pembeli dan penjual tentang pesanan pasar digital Node2Mint.' },
  '/afiliasi': { title: 'Afiliasi | Node2Mint', description: 'Tinjau tautan afiliasi, komisi, dan aktivitas Node2Mint.' },
  '/admin': { title: 'Administrator | Node2Mint', description: 'Kelola pengguna, paket, model, penyedia, pesanan, dan pengaturan Node2Mint.' },
};

// Judul/deskripsi/robots statis; canonical + hreflang + OG url digabung di
// generateMetadata karena butuh path URL asli (path hasil rewrite mencerminkan
// route internal, bukan alamat yang dibaca pengunjung/perayap).
const BASE_META = {
  title: {
    default: 'Node2Mint | Multi-Model AI API & Digital Marketplace',
    template: '%s | Node2Mint',
  },
  description,
  applicationName: 'Node2Mint',
  category: 'technology',
  icons: [{ rel: 'icon', url: '/favicon.ico' }],
  robots: { index: true, follow: true },
};

// Organisasi + WebSite sekali di root. SameAs sengaja dibiarkan kosong sampai
// admin mengisi profil komunitas; tautan karangan merusak graf entitas.
const JSON_LD = {
  '@context': 'https://schema.org',
  '@graph': [
    {
      '@type': 'Organization',
      '@id': `${SITE}/#organization`,
      name: 'Node2Mint',
      url: SITE,
      logo: `${SITE}/node2mint-icon.ico`,
      email: 'support@node2mint.com',
      description,
    },
    {
      '@type': 'WebSite',
      '@id': `${SITE}/#website`,
      name: 'Node2Mint',
      url: SITE,
      inLanguage: ['en', 'id', 'zh', 'ru', 'ja'],
      publisher: { '@id': `${SITE}/#organization` },
    },
  ],
};

// BCP-47 untuk atribut lang; kamus internal memakai cn/jp.
const HTML_LANG: Record<string, string> = { en: 'en', id: 'id', zh: 'zh', ru: 'ru', ja: 'ja' };
const PUBLIC_ROOTS = new Set(['', 'pricing', 'market', 'articles', 'artikel', 'about', 'docs', 'guides', 'faq', 'status', 'changelog', 'terms', 'privacy', 'refund', 'help', 'bantuan', 'contact']);

// hreflang/canonical per halaman tidak bisa ditulis di metadata statis: canonical
// yang dirender akan memakai path internal hasil rewrite. Header x-locale-path
// dari middleware membawa path URL asli (tanpa prefix locale).
export async function generateMetadata(): Promise<Metadata> {
  const h = await headers();
  const path = (h.get('x-locale-path') || '/').replace(/\/$/, '') || '/';
  const suffix = path === '/' ? '' : path;
  const locale = h.get('x-locale') || 'en';
  const localizedBase = locale === 'id'
    ? { ...BASE_META, title: { default: PAGE_META_ID['/'].title, template: '%s | Node2Mint' }, description: descriptionId }
    : BASE_META;
  const page = locale === 'id'
    ? (PAGE_META_ID[path] ?? PRIVATE_META_ID[path] ?? PAGE_META_ID['/'])
    : (PAGE_META[path] ?? PRIVATE_META[path] ?? PAGE_META['/']);
  if (!PUBLIC_ROOTS.has(path.split('/')[1] || '')) {
    return {
      metadataBase: new URL(SITE),
      ...localizedBase,
      title: page.title,
      description: page.description,
      robots: { index: false, follow: false },
    };
  }
  return {
    metadataBase: new URL(SITE),
    ...localizedBase,
    title: page.title,
    description: page.description,
    alternates: {
      canonical: `/${locale}${suffix}`,
      languages: { en: `/en${suffix}`, id: `/id${suffix}`, zh: `/zh${suffix}`, ru: `/ru${suffix}`, ja: `/ja${suffix}`, 'x-default': `/en${suffix}` },
    },
    openGraph: {
      type: 'website',
      siteName: 'Node2Mint',
      url: `${SITE}/${locale}${suffix}`,
      title: page.title,
      description: page.description,
      images: [{ url: `${SITE}/opengraph-image`, width: 1200, height: 630, alt: locale === 'id' ? 'Node2Mint — API AI Multi-Model dan Pasar Digital' : 'Node2Mint — Multi-Model AI API & Digital Marketplace' }],
    },
    twitter: {
      card: 'summary_large_image',
      title: page.title,
      description: page.description,
      images: [`${SITE}/opengraph-image`],
    },
  };
}

export default async function RootLayout({ children }: { children: ReactNode }) {
  const h = await headers();
  const cookieStore = await cookies();
  const locale = h.get('x-locale') || cookieStore.get('locale')?.value || 'en';
  const jsonLd = locale === 'id'
    ? { ...JSON_LD, '@graph': JSON_LD['@graph'].map((item, index) => index === 0 ? { ...item, description: descriptionId } : item) }
    : JSON_LD;
  return (
    <html lang={HTML_LANG[locale] || 'en'} className="dark">
      <body className={`${sans.variable} ${mono.variable} min-h-dvh bg-bg font-sans text-text antialiased`}>
        <script nonce={h.get('x-nonce') || undefined} type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd).replaceAll('<', '\\u003c') }} />
        <LocaleProvider serverPrefix={h.get('x-locale') ? `/${h.get('x-locale')}` : ''} serverLang={locale}>
          <AuthProvider>
            <ToastProvider>
              <ConfirmProvider>
                <AppShell>{children}</AppShell>
                <ServiceBanner />
                <CookieBanner />
              </ConfirmProvider>
            </ToastProvider>
          </AuthProvider>
        </LocaleProvider>
      </body>
    </html>
  );
}
