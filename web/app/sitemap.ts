import type { MetadataRoute } from 'next';

// Canonical host: never localhost.
const base = (process.env.NEXT_PUBLIC_SITE_URL || 'https://www.node2mint.com').replace(/\/$/, '');
const LOCALES = ['en', 'id', 'zh', 'ru', 'ja'];
const routes = ['', '/pricing', '/market', '/about', '/artikel', '/docs', '/guides', '/faq', '/status', '/changelog', '/terms', '/privacy', '/refund', '/bantuan', '/contact'];

// Semua URL publik memakai prefix locale; /en adalah x-default. URL tanpa prefix
// melakukan 308 di proxy dan sengaja tidak masuk sitemap.
export default function sitemap(): MetadataRoute.Sitemap {
  return routes.flatMap((path) => {
    const languages: Record<string, string> = {};
    for (const locale of LOCALES) languages[locale] = `${base}/${locale}${path}`;
    languages['x-default'] = `${base}/en${path}`;
    return LOCALES.map((l) => `/${l}${path}`).map((suffix) => ({
      url: `${base}${suffix}`,
      alternates: { languages },
    }));
  });
}
