import type { MetadataRoute } from 'next';

// Canonical host: never localhost. Locale variants (/en /id /zh /ru /ja) are
// prefixes of these same paths.
const base = (process.env.NEXT_PUBLIC_SITE_URL || 'https://www.node2mint.com').replace(/\/$/, '');

// Pelatih model (korpus training) ditolak; perayap pencarian/kutipan dibiarkan.
const TRAINING_BOTS = [
  'CCBot',
  'GPTBot',
  'ClaudeBot',
  'anthropic-ai',
  'Google-Extended',
  'Applebot-Extended',
  'meta-externalagent',
  'Bytespider',
  'Amazonbot',
  'Diffbot',
  'ImagesiftBot',
  'Omgilibot',
];

// Area aplikasi tidak untuk diindeks: halaman berisi UI pribadi (saldo, kunci
// API). Bot kutipan tetap diizinkan masuk ke halaman publik.
const DISALLOW = ['/admin/', '/dashboard/', '/wallet/', '/api-keys/', '/sign-in', '/api/', '/orders/', '/settings/', '/store/', '/affiliate/', '/referral/', '/cart/', '/payment', '/chat/', '/reward/', '/tulis-artikel', '/forgot-password', '/reset-password'];

export default function robots(): MetadataRoute.Robots {
  return {
    rules: [
      { userAgent: '*', allow: '/', disallow: DISALLOW },
      { userAgent: TRAINING_BOTS, disallow: '/' },
      // Perayap pencarian + pengambilan konteks sitasi: jangan sampai terkena
      // aturan lain.
      { userAgent: ['OAI-SearchBot', 'ChatGPT-User', 'Claude-User', 'Claude-SearchBot', 'PerplexityBot', 'Perplexity-User', 'Googlebot', 'Bingbot', 'Applebot', 'DuckDuckBot', 'YandexBot'], allow: '/', disallow: DISALLOW },
    ],
    sitemap: `${base}/sitemap.xml`,
  };
}
