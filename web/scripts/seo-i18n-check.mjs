import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const web = resolve(import.meta.dirname, '..');
const read = (file) => readFileSync(resolve(web, file), 'utf8');
const failures = [];
const check = (condition, message) => {
  if (!condition) failures.push(message);
};
const section = (source, start, end) => source.slice(source.indexOf(start), end ? source.indexOf(end, source.indexOf(start)) : undefined);

const proxy = read('proxy.ts');
const layout = read('app/layout.tsx');
const sitemap = read('app/sitemap.ts');
const i18n = read('lib/i18n.tsx');
const header = read('components/PublicHeader.tsx');
const footer = read('components/PublicFooter.tsx');
const languageMenu = read('components/LanguageMenu.tsx');
const legalLayout = read('components/LegalLayout.tsx');
const terms = read('app/terms/page.tsx');
const privacy = read('app/privacy/page.tsx');
const refund = read('app/refund/page.tsx');
const securityTxt = read('public/security.txt');

for (const [name, source] of Object.entries({ proxy, layout, sitemap })) {
  const seoHost = source.match(/const (?:SITE|base)\s*=[\s\S]{0,180}/)?.[0] ?? '';
  check(!/localhost/i.test(seoHost), `${name}: production SEO host must not be localhost`);
}

const locales = ['en', 'id', 'zh', 'ru', 'ja'];
for (const [name, source] of Object.entries({ proxy, layout, sitemap })) {
  for (const locale of locales) check(source.includes(`'${locale}'`), `${name}: missing locale ${locale}`);
}

check(/PUBLIC_ROOTS[^\n]*'contact'/.test(proxy), 'proxy.ts: contact must be public and locale-prefixed');
check(/PUBLIC_ROOTS[^\n]*'contact'/.test(layout), 'layout.tsx: contact must be indexable');
check(/routes[^\n]*'\/contact'/.test(sitemap), 'sitemap.ts: missing contact route');
check(!/['"]\/(?:referral|toko|store)['"]/.test(sitemap), 'sitemap.ts: private referral/store route must not be listed');
check(!/new Date\(|lastModified|changeFrequency/.test(sitemap), 'sitemap.ts: do not invent uniform lastmod or change frequency');
check(proxy.includes('<${SITE}/en${rest}>; rel="alternate"; hreflang="x-default"'), 'proxy.ts: x-default must be the canonical English URL');
check(layout.includes("'x-default': `/en${suffix}`"), 'layout.tsx: metadata x-default must be English');
check(layout.includes("if (!PUBLIC_ROOTS.has(path.split('/')[1] || '')") && layout.includes('robots: { index: false, follow: false }'), 'layout.tsx: private routes need explicit noindex metadata');
check(/if \(match && !isPublicPath\(path\)\)/.test(proxy), 'proxy.ts: locale prefixes on private routes must canonicalize away');

const pageMeta = [...layout.matchAll(/title: '([^']+)', description: '([^']+)'/g)];
for (const [field, index] of [['title', 1], ['description', 2]]) {
  const values = pageMeta.map((match) => match[index]);
  check(new Set(values).size === values.length, `layout.tsx: duplicate page ${field}`);
}

const navLinks = [...header.matchAll(/\['nav\.[^']+', '([^']+)'\]/g)].map((match) => match[1]);
check(navLinks.length === 6, 'PublicHeader.tsx: public navigation should remain the six primary destinations');
check(!navLinks.includes('/pricing#models') && !navLinks.includes('/status'), 'PublicHeader.tsx: models/status belong to their destination/footer, not the primary menu');
check(header.includes('href={nav(href)}') && footer.includes('href={nav(link.href)}'), 'public navigation links must preserve the active locale prefix');
check(!/href=["']\/(?:en|id|zh|ru|ja)\//.test(`${header}\n${footer}\n${languageMenu}`), 'public chrome must not hard-code a locale prefix');
check(footer.includes('<!--email_off-->') && footer.includes('<!--/email_off-->'), 'PublicFooter.tsx: support email must opt out of Cloudflare HTML rewriting');
check(/^Contact: mailto:support@node2mint\.com/m.test(securityTxt), 'public/security.txt: must publish the existing support contact');
check(terms.includes("title: 'Terms of Use'") && privacy.includes("title: 'Privacy Policy'") && refund.includes("title: 'Refund Policy'"), 'legal pages: English Terms, Privacy, and Refund copy must be present');
check(legalLayout.includes('Indonesian text controls'), 'LegalLayout.tsx: English legal pages must retain the Indonesian-text controlling notice');
check(layout.includes("'/articles': { title: 'Panduan API AI & Artikel Pasar Digital | Node2Mint'") && layout.includes("'/help': { title: 'Pusat Bantuan Node2Mint'"), 'app/layout.tsx: Indonesian localized metadata must use the canonical /articles and /help paths');

for (const [name, source] of Object.entries({ header, footer, languageMenu })) {
  for (const tag of source.matchAll(/<img\b[^>]*>/g)) {
    check(/\balt=/.test(tag[0]), `${name}: image missing alt attribute`);
  }
}

const supportKeys = ['sup.kicker', 'sup.h', 'sup.sub', 'sup.emailH', 'sup.emailP', 'sup.emailCta', 'sup.copyEmail', 'sup.copied', 'sup.hoursH', 'sup.hoursP', 'sup.needH', 'sup.need1', 'sup.need2', 'sup.need3', 'sup.need4', 'sup.escalateH', 'sup.escalateP', 'sup.resetH', 'sup.resetP', 'sup.resetBtn', 'sup.faq', 'sup.docs', 'sup.status'];
for (const [locale, source] of Object.entries({ cn: section(i18n, '  cn: {', '  ru: {'), ru: section(i18n, '  ru: {', '  jp: {'), jp: section(i18n, '  jp: {', '};\n\nconst localeOverrides') })) {
  for (const key of supportKeys) check(source.includes(`'${key}'`), `i18n.tsx: ${locale} support translation missing ${key}`);
  check(source.includes("'foot.credit'"), `i18n.tsx: ${locale} footer credit translation missing`);
}
const jp = section(i18n, '  jp: {', '};\n\nconst localeOverrides');
check(!/models\.showAll': 'Show all/.test(jp), 'i18n.tsx: Japanese model CTA still falls back to English');
check(i18n.includes("'mkt.verified': 'Terverifikasi'"), 'i18n.tsx: Indonesian marketplace verification label still leaks English');
check(i18n.includes("'faq.h': 'Pertanyaan, terjawab.'"), 'i18n.tsx: Indonesian FAQ heading still leaks English');

if (failures.length) {
  console.error(`SEO/i18n static check failed (${failures.length}):`);
  for (const failure of failures) console.error(`- ${failure}`);
  process.exitCode = 1;
} else {
  console.log('SEO/i18n static check passed: locale routing, canonical metadata, sitemap, links, translations, and image alt attributes.');
}
