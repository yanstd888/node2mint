// Kamus: kerangka aplikasi (sidebar, topbar, lonceng notifikasi, keranjang, footer app).
export const en = {
  'sb.nav': 'Main navigation', 'sb.close': 'Close menu', 'sb.logout': 'Sign out',
  'sb.user': 'Node2Mint',
  'sb.dashboard': 'Dashboard', 'sb.pricing': 'Packs & Pricing', 'sb.market': 'Marketplace',
  'sb.playground': 'Playground', 'sb.orders': 'Orders', 'sb.library': 'Buyer Library', 'sb.wishlist': 'Wishlist', 'sb.wallet': 'Wallet',
  'sb.store': 'My Store', 'sb.affiliate': 'Affiliate', 'sb.referral': 'Referral', 'sb.apikeys': 'API Keys',
  'sb.help': 'Help Center', 'sb.settings': 'Account Settings',
  'sb.gTransaksi': 'Transactions & Finance', 'sb.gKemitraan': 'Partnerships & Integration',
  'sb.walletAmt': 'Wallet {amount}', 'sb.skip': 'Skip to content',
  'sb.expand': 'Expand sidebar', 'sb.collapse': 'Collapse sidebar', 'sb.account': 'Account menu',
  'sb.openMenu': 'Open menu',
  'notif.title': 'Notifications', 'notif.all': 'All', 'notif.markAll': 'Mark all as read',
  'notif.markOne': 'Mark as read', 'notif.fail': 'Could not load notifications.',
  'notif.loading': 'Loading notifications', 'notif.empty': 'No notifications yet.',
  'notif.openChat': 'Open order chat {title}', 'notif.more': 'Load more', 'notif.noMore': 'No more notifications',
  'sb.menu': 'Menu',
} as const;

export const id: Record<keyof typeof en, string> = {
  'sb.nav': 'Navigasi utama', 'sb.close': 'Tutup menu', 'sb.logout': 'Keluar',
  'sb.user': 'Node2Mint',
  'sb.dashboard': 'Dasbor', 'sb.pricing': 'Paket & Harga', 'sb.market': 'Pasar Digital',
  'sb.playground': 'Ruang Uji', 'sb.orders': 'Pesanan', 'sb.library': 'Pustaka Pembeli', 'sb.wishlist': 'Favorit', 'sb.wallet': 'Dompet',
  'sb.store': 'Toko Saya', 'sb.affiliate': 'Afiliasi', 'sb.referral': 'Rujukan', 'sb.apikeys': 'Kunci API',
  'sb.help': 'Pusat Bantuan', 'sb.settings': 'Pengaturan Akun',
  'sb.gTransaksi': 'Transaksi & Keuangan', 'sb.gKemitraan': 'Kemitraan & Integrasi',
  'sb.walletAmt': 'Dompet {amount}', 'sb.skip': 'Lompat ke konten',
  'sb.expand': 'Buka bilah samping', 'sb.collapse': 'Ciutkan bilah samping', 'sb.account': 'Menu akun',
  'sb.openMenu': 'Buka menu',
  'notif.title': 'Notifikasi', 'notif.all': 'Semua', 'notif.markAll': 'Tandai semua dibaca',
  'notif.markOne': 'Tandai dibaca', 'notif.fail': 'Gagal memuat notifikasi.',
  'notif.loading': 'Memuat notifikasi', 'notif.empty': 'Belum ada notifikasi.',
  'notif.openChat': 'Buka obrolan pesanan {title}', 'notif.more': 'Muat lebih banyak', 'notif.noMore': 'Tidak ada notifikasi lagi',
  'sb.menu': 'Menu',
};
// Cek mandiri: jalankan hasil kompilasi shell.ts — gagal kalau en/id tidak
// sepadan, ada kunci kosong, atau slot {name} berbeda antar bahasa.
export function cekKamus(d: Record<string, string>[] = [en, id]) {
  const ks = d.map((x) => Object.keys(x).sort().join('\n'));
  if (ks[1] !== ks[0]) throw new Error('kunci en/id tidak sepadan');
  for (const dict of d) for (const [k, v] of Object.entries(dict)) {
    if (!v.trim()) throw new Error(`nilai kosong: ${k}`);
    const a = [...String(en[k as keyof typeof en]).matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort().join(',');
    const b = [...v.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort().join(',');
    if (a !== b) throw new Error(`slot beda di ${k}: ${a} vs ${b}`);
  }
  return true;
}
if (typeof process !== 'undefined' && process.argv[1]?.includes('shell')) {
  cekKamus();
  console.log('shell.ts: kamus OK');
}
