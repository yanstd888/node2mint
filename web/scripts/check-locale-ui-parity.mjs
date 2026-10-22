import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('..', import.meta.url));
const sourceFiles = [
  'lib/i18n.tsx',
  'lib/dict/account.ts',
  'lib/dict/seller.ts',
  'lib/dict/shell.ts',
  'lib/dict/wallet.ts',
  'lib/dict/public-pages.ts',
  'lib/dict/admin.ts',
].map((file) => path.join(root, file));
const focus = ['sb.', 'notif.', 'auth.', 'market.', 'mkt.', 'pd.', 'show.', 'cart.'];
const keyPattern = /'([A-Za-z0-9_.-]+)'\s*:\s*'/g;
const keys = new Set();
for (const file of sourceFiles) {
  const source = fs.readFileSync(file, 'utf8');
  // dict/admin.ts punya blok `en` DAN `id`; hanya `en` yang jadi daftar kunci
  // acuan supaya key yang cuma ada di `id` tidak ikut dituntut di cn/ru/jp.
  const scan = file.endsWith('dict/admin.ts')
    ? source.slice(source.indexOf('export const en'), source.indexOf('export const id'))
    : source;
  for (const match of scan.matchAll(keyPattern)) {
    if (focus.some((prefix) => match[1].startsWith(prefix))) keys.add(match[1]);
  }
}

// cn/ru/jp dirakit t() dari tiga sumber: locale-overrides.generated.json
// (dibuat skrip), locale-ui-overrides.json (manual), dan manualLocaleOverrides
// di lib/i18n.tsx. Cek harus melihat ketiganya — memeriksa JSON saja memberi
// laporan palsu "missing" untuk key yang justru sudah ada di i18n.tsx.
const overrides = JSON.parse(fs.readFileSync(path.join(root, 'lib/dict/locale-ui-overrides.json'), 'utf8'));
const generated = JSON.parse(fs.readFileSync(path.join(root, 'lib/dict/locale-overrides.generated.json'), 'utf8'));
const i18nSource = fs.readFileSync(path.join(root, 'lib/i18n.tsx'), 'utf8');
const manualBlock = i18nSource.slice(i18nSource.indexOf('const manualLocaleOverrides'), i18nSource.indexOf('const localeOverrides'));
const literalPattern = /'([A-Za-z0-9_.-]+)'\s*:\s*'/g;
const manual = {};
for (const lang of ['cn', 'ru', 'jp']) {
  const start = manualBlock.indexOf(`\n  ${lang}: {`);
  const end = manualBlock.indexOf('\n  },', start);
  manual[lang] = new Set([...manualBlock.slice(start, end).matchAll(literalPattern)].map((m) => m[1]));
}

const languages = ['cn', 'ru', 'jp'];
const errors = [];
for (const lang of languages) {
  const missing = [...keys].filter((key) =>
    typeof overrides[lang]?.[key] !== 'string'
    && typeof generated[lang]?.[key] !== 'string'
    && !manual[lang].has(key));
  if (missing.length) errors.push(`${lang}: missing ${missing.join(', ')}`);
}

if (errors.length) {
  console.error(errors.join('\n'));
  process.exit(1);
}
const proxy = fs.readFileSync(path.join(root, 'proxy.ts'), 'utf8');
const routingChecks = [
  "const guardPath = req.nextUrl.pathname.replace(PREFIX, '') || '/';",
  "guardPath === '/admin' || guardPath.startsWith('/admin/')",
  'const localized = (path: string)',
];
for (const check of routingChecks) {
  if (!proxy.includes(check)) {
    console.error(`routing guard check missing: ${check}`);
    process.exit(1);
  }
}
console.log(`locale UI parity ok: ${keys.size} keys × ${languages.length} locales; prefixed admin guard ok`);
