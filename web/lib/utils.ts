import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// Ledger tetap IDR. Bahasa Indonesia memakai IDR; semua bahasa lain memakai USD.
export const IDR_PER_USD = 18000;
const formatIDR = (n: unknown) => `Rp${(Number.isFinite(Number(n)) ? Number(n) : 0).toLocaleString('id-ID')}`;
export const money = (n: unknown, lang = 'id') => {
  const safe = Number.isFinite(Number(n)) ? Number(n) : 0;
  return lang === 'id'
    ? formatIDR(safe)
    : new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', maximumFractionDigits: 2 }).format(safe / IDR_PER_USD);
};
// API lama dipertahankan agar semua harga yang sudah memakainya ikut format bahasa aktif.
export const rupiah = (n: unknown) => money(n, typeof document === 'undefined' ? 'id' : document.documentElement.lang);
export const amountToIDR = (n: unknown, lang = 'id') => {
  const value = Number(n);
  return Math.round((Number.isFinite(value) ? value : 0) * (lang === 'id' ? 1 : IDR_PER_USD));
};

export const angka = (n: unknown) => {
  const v = Number(n);
  return (Number.isFinite(v) ? v : 0).toLocaleString('id-ID');
};

// Gateway kadang membalas array telanjang, kadang dibungkus {items: [...]}. Panggil
// dengan nama kunci yang mungkin dipakai supaya halaman tidak menebak satu bentuk.
export const asList = <T,>(d: unknown, ...keys: string[]): T[] => {
  if (Array.isArray(d)) return d as T[];
  for (const k of keys) {
    const v = (d as Record<string, unknown> | null)?.[k];
    if (Array.isArray(v)) return v as T[];
  }
  return [];
};

// Tanggal ringkas; toleran terhadap nilai kosong/format aneh (→ '-', bukan "Invalid Date").
export const tanggal = (v: unknown, denganWaktu = false, lang: string = 'id') => {
  if (!v) return '-';
  const d = new Date(String(v));
  if (Number.isNaN(d.getTime())) return '-';
  return d.toLocaleDateString(localeFor(lang), {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    ...(denganWaktu ? { hour: '2-digit', minute: '2-digit' } : {}),
  });
};

// Tanggal + jam. Zona waktu dipatok WIB (UTC+7) supaya masa aktif paket token
// sama untuk semua pembaca, apa pun zona perangkatnya. `lang` memilih nama bulan.
const localeFor = (lang: string) => ({ id: 'id-ID', cn: 'zh-CN', ru: 'ru-RU', jp: 'ja-JP' }[lang] || 'en-US');

export const tanggalJam = (v: unknown, lang: string = 'id') => {
  if (!v) return '-';
  const d = new Date(String(v));
  if (Number.isNaN(d.getTime())) return '-';
  const tanggal = d.toLocaleDateString(localeFor(lang), {
    day: 'numeric', month: 'short', year: 'numeric', timeZone: 'Asia/Jakarta',
  });
  const jam = d.toLocaleTimeString(localeFor(lang), {
    hour: '2-digit', minute: '2-digit', timeZone: 'Asia/Jakarta',
  });
  return `${tanggal}, ${jam} (WIB/UTC+7)`;
};

// UUID penuh membuat detail bisa memanggil endpoint item langsung; judul/toko tetap SEO-friendly.
export const productUrl = (storeName: string | undefined, title: string, id: string) =>
  `/market/${slug(storeName)}/${slug(title)}-${id}`;

export const paymentUrl = (kind: 'product' | 'package' | 'cart', amount: number, label: string, extra: Record<string, string | number> = {}) => {
  const query = new URLSearchParams({ kind, amount: String(amount), label, ...Object.fromEntries(Object.entries(extra).map(([k, v]) => [k, String(v)])) });
  return `/payment?${query}`;
};

export const slug = (s: string | undefined) =>
  (s ?? '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 50) || 'item';
