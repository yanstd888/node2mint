import { NextRequest, NextResponse } from 'next/server';

// Middleware tiga tugas: (1) nonce CSP per request, (2) guard /admin di sisi
// server, (3) locale URL /en /id /zh /ru /ja. Nonce membuat 'unsafe-inline'
// tidak lagi dibutuhkan di produksi; Next membaca CSP dari header REQUEST lalu
// menempelkan nonce ke skripnya sendiri (resep resmi Next), termasuk ke skrip
// inline nonce di layout.

const dev = process.env.NODE_ENV !== 'production';
const countryLocale: Record<string, 'id' | 'cn' | 'ru' | 'jp'> = { ID: 'id', CN: 'cn', RU: 'ru', JP: 'jp' };

// Fallback produksi: canonical/hreflang tidak pernah menunjuk localhost.
const SITE = (process.env.NEXT_PUBLIC_SITE_URL || 'https://www.node2mint.com').replace(/\/$/, '');

// Segmen URL → kode kamus internal. URL memakai zh/ja (BCP-47) tetapi kamus
// tetap cn/jp — tidak ada berkas route duplikat per bahasa.
const LOCALE_URLS = ['en', 'id', 'zh', 'ru', 'ja'] as const;
const URL_LOCALE: Record<string, 'en' | 'id' | 'cn' | 'ru' | 'jp'> = { en: 'en', id: 'id', zh: 'cn', ru: 'ru', ja: 'jp' };
const LANG_URL: Record<'en' | 'id' | 'cn' | 'ru' | 'jp', string> = { en: 'en', id: 'id', cn: 'zh', ru: 'ru', jp: 'ja' };
const PREFIX = /^\/(en|id|zh|ru|ja)(?=\/|$)/;
const PUBLIC_ROOTS = new Set(['', 'pricing', 'market', 'artikel', 'about', 'docs', 'guides', 'faq', 'status', 'changelog', 'terms', 'privacy', 'refund', 'bantuan', 'contact']);
const ENGLISH_ROUTE: Record<string, string> = {
  articles: 'artikel', 'sign-in': 'masuk', help: 'bantuan', 'forgot-password': 'lupa-password',
  cart: 'keranjang', orders: 'pesanan', settings: 'pengaturan', store: 'toko', affiliate: 'afiliasi', 'write-article': 'tulis-artikel',
};
const LEGACY_ROUTE: Record<string, string> = { ...Object.fromEntries(Object.entries(ENGLISH_ROUTE).map(([external, internal]) => [internal, external])), daftar: 'sign-up', pengaturan: 'settings' };

function mapFirst(path: string, routes: Record<string, string>) {
  const parts = path.split('/');
  if (routes[parts[1]]) parts[1] = routes[parts[1]];
  return parts.join('/');
}

function isPublicPath(path: string) {
  return PUBLIC_ROOTS.has(path.split('/')[1] || '');
}

function requestIp(req: NextRequest) {
  return (req.headers.get('x-real-ip') || req.headers.get('cf-connecting-ip') || '')
    .split(',')[0]
    .trim();
}

function ipInAllowlist(ip: string, entry: string) {
  const [base, prefixText] = entry.trim().split('/');
  if (!prefixText) return ip === base;
  const prefix = Number(prefixText);
  if (ip.includes(':') || base.includes(':')) return prefix === 128 && ip === base;
  const toInt = (value: string) => {
    const octets = value.split('.').map(Number);
    return octets.length === 4 && octets.every((octet) => Number.isInteger(octet) && octet >= 0 && octet <= 255)
      ? (((octets[0] << 24) >>> 0) + (octets[1] << 16) + (octets[2] << 8) + octets[3]) >>> 0
      : null;
  };
  const address = toInt(ip);
  const network = toInt(base);
  if (address === null || network === null || !Number.isInteger(prefix) || prefix < 0 || prefix > 32) return false;
  const mask = prefix === 0 ? 0 : (0xffffffff << (32 - prefix)) >>> 0;
  return (address & mask) === (network & mask);
}

function siteIpAllowed(req: NextRequest) {
  if (process.env.NODE_ENV !== 'production') return true;
  const ip = requestIp(req);
  if (!ip) return false;
  return (process.env.SITE_IP_ALLOWLIST || '')
    .split(',')
    .filter(Boolean)
    .some((entry) => ipInAllowlist(ip, entry));
}

export async function proxy(req: NextRequest) {
  const nonce = Buffer.from(crypto.randomUUID()).toString('base64');
  const guardLocale = req.nextUrl.pathname.match(PREFIX)?.[1];
  const guardPath = req.nextUrl.pathname.replace(PREFIX, '') || '/';
  const localized = (path: string) => `${guardLocale ? `/${guardLocale}` : ''}${path}`;

  const csp = [
    "default-src 'self'",
    // 'unsafe-inline' diabaikan browser begitu nonce ada — di dev tetap
    // dicantumkan karena HMR menyuntik skrip tanpa nonce.
    `script-src 'self' ${dev ? "'unsafe-inline' 'unsafe-eval'" : `'nonce-${nonce}' 'strict-dynamic'`} https://telegram.org https://challenges.cloudflare.com`,
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob: https: http:",
    "font-src 'self' data:",
    // DOKU Checkout dibuka sebagai redirect tingkat-atas, bukan iframe atau SDK.
    `connect-src 'self'${dev ? ' ws: wss:' : ''}`,
    "frame-src 'self' https://oauth.telegram.org https://challenges.cloudflare.com",
    "frame-ancestors 'none'",
    "base-uri 'self'",
    "form-action 'self'",
  ].join('; ');

  const headers = new Headers(req.headers);
  headers.set('x-nonce', nonce);
  headers.set('Content-Security-Policy', csp);

  // Hanya halaman login/daftar yang dibatasi IP. Halaman publik lain tetap terbuka;
  // endpoint auth tetap fail-closed di gateway.
  const authPage = ['/masuk', '/sign-in', '/sign-up', '/daftar'].includes(guardPath);
  if (authPage && !siteIpAllowed(req)) {
    const res = NextResponse.rewrite(new URL('/coming-soon', req.url), { request: { headers } });
    res.headers.set('Content-Security-Policy', csp);
    return res;
  }

  // Guard halaman admin: peran dibaca dari gateway memakai cookie sesi, bukan
  // dari localStorage (yang bisa diakali dari konsol). Aktif di semua
  // lingkungan — login dev tetap jalan, yang ditolak hanya sesi non-admin.
  // Gagal jaringan = lepas (fail-open) supaya gangguan gateway tidak mengunci
  // admin dari panelnya; otorisasi endpoint tetap ditegakkan gateway.
  if (guardPath === '/admin' || guardPath.startsWith('/admin/')) {
    try {
      const res = await fetch(new URL('/api/dashboard', req.url), {
        headers: { cookie: req.headers.get('cookie') || '' },
        cache: 'no-store',
      });
      if (!res.ok) return NextResponse.redirect(new URL(localized('/masuk'), req.url));
      const { role } = (await res.json()) as { role?: string };
      if (role !== 'ADMIN' && role !== 'SUPER_ADMIN') {
        return NextResponse.redirect(new URL(localized('/dashboard'), req.url));
      }
    } catch {
      /* gateway tidak terjawab: biarkan halaman memutuskan sendiri */
    }
  }

  // Locale URL: /<locale>/<halaman> dipetakan ke route internal tanpa prefix,
  // jadi satu berkas route melayani semua bahasa. Cookie ikut diisi supaya
  // kamus klien memakai bahasa yang sama dengan URL.
  //
  // Next menjalankan middleware DUA kali untuk URL berprefiks: sekali dengan
  // /zh/…, sekali lagi setelah rewrite dengan path internal. Nilai yang sudah
  // dipasang karena itu diteruskan, bukan ditimpa.
  const match = req.nextUrl.pathname.match(PREFIX);
  const urlLocale = match?.[1];
  const rawPath = match ? req.nextUrl.pathname.slice(match[0].length) || '/' : req.nextUrl.pathname;
  const publicPath = mapFirst(rawPath, LEGACY_ROUTE);
  const path = mapFirst(publicPath, ENGLISH_ROUTE);
  const priorLocale = req.headers.get('x-locale');
  const priorPath = req.headers.get('x-locale-path');

  // URL lama berbahasa Indonesia hanya kompatibilitas; alamat yang terlihat
  // selalu memakai base English tanpa memindahkan folder route Next.
  if (!priorPath && rawPath !== publicPath) {
    return NextResponse.redirect(new URL(`${match ? `/${urlLocale}` : ''}${publicPath}${req.nextUrl.search}`, req.url), 301);
  }

  if (match && !isPublicPath(path)) {
    return NextResponse.redirect(new URL(`${path}${req.nextUrl.search}`, req.url), 301);
  }

  const preferred = req.cookies.get('locale')?.value as keyof typeof LANG_URL | undefined;
  // Prefix locale eksplisit selalu mengalahkan cookie lama. Cookie diperbarui
  // di bawah agar /id/... tetap benar meski sesi sebelumnya memakai /en/....

  // Path SEO selalu path saat ini; prefix locale sudah dibuang.
  const seoPath = priorPath ?? publicPath;
  const locale = urlLocale ?? (priorLocale && priorPath ? priorLocale : preferred ?? null);

  if (locale) headers.set('x-locale', locale);
  else headers.delete('x-locale');
  headers.set('x-locale-path', seoPath);

  const res = match || path !== req.nextUrl.pathname
    ? NextResponse.rewrite(Object.assign(req.nextUrl.clone(), { pathname: path }), { request: { headers } })
    : NextResponse.next({ request: { headers } });
  res.headers.set('Content-Security-Policy', csp);

  // hreflang di header Link: berlaku untuk semua halaman publik, termasuk yang
  // metadatanya statis. URL absolut www supaya tidak ada host ganda.
  const rest = seoPath && seoPath !== '/' ? seoPath : '';
  res.headers.set(
    'Link',
    [...LOCALE_URLS.map((code) => `<${SITE}/${code}${rest}>; rel="alternate"; hreflang="${code}"`), `<${SITE}/en${rest}>; rel="alternate"; hreflang="x-default"`].join(', '),
  );

  // Cloudflare supplies this edge header only when IP Geolocation is enabled.
  // Never overwrite a language explicitly selected by the visitor or the URL.
  const cookie = { path: '/', maxAge: 60 * 60 * 24 * 365, sameSite: 'lax' as const, secure: true };
  if (urlLocale) {
    res.cookies.set('locale', URL_LOCALE[urlLocale], cookie);
  } else if (!req.cookies.has('locale')) {
    const localeFromCountry = countryLocale[req.headers.get('cf-ipcountry')?.toUpperCase() || ''];
    if (localeFromCountry) res.cookies.set('locale', localeFromCountry, cookie);
  }
  return res;
}

// Jangan sentuh jalur API (gateway sudah memasang header sendiri) dan aset statis.
export const config = {
  matcher: ['/((?!api/|v1/|models|_next/static|_next/image|favicon.ico|media/|robots.txt|sitemap.xml).*)'],
};
