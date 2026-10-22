'use client';
// Render QR sebagai SVG, di browser, tanpa mengirim data ke mana pun.
// Pakai `qrcode-generator` (0 dependency) — encoder sendiri pernah dicoba di sini
// dan Reed-Solomon/placement-nya salah (gagal decode di zxing), jadi pakai yang teruji.
import qrcode from 'qrcode-generator';

export type EcLevel = 'L' | 'M' | 'Q' | 'H';

/**
 * Bangun string SVG QR untuk `text`.
 * `size` = lebar/tinggi piksel SVG; `margin` = quiet zone dalam modul (min 4 sesuai spec).
 */
export function qrSvg(
  text: string,
  opts: { size?: number; margin?: number; ec?: EcLevel; dark?: string; light?: string } = {},
): string {
  const { size = 220, margin = 4, ec = 'M', dark = '#000000', light = '#ffffff' } = opts;
  // typeNumber 0 = pilih versi otomatis sesuai panjang data.
  const qr = qrcode(0, ec);
  qr.addData(text);
  qr.make();
  const count = qr.getModuleCount();
  const dim = count + margin * 2;
  const scale = size / dim;

  let path = '';
  for (let r = 0; r < count; r++) {
    for (let c = 0; c < count; c++) {
      if (qr.isDark(r, c)) {
        path += `M${(c + margin) * scale} ${(r + margin) * scale}h${scale}v${scale}h-${scale}z`;
      }
    }
  }
  return (
    `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" ` +
    `viewBox="0 0 ${size} ${size}" shape-rendering="crispEdges" role="img">` +
    `<rect width="${size}" height="${size}" fill="${light}"/>` +
    `<path d="${path}" fill="${dark}"/></svg>`
  );
}

/** Data URI untuk dipakai sebagai src <img>. */
export function qrDataUri(text: string, opts: { size?: number; margin?: number; ec?: EcLevel } = {}): string {
  const svg = qrSvg(text, { dark: '#000000', light: '#ffffff', ...opts });
  // encodeURIComponent: btoa tidak aman untuk karakter non-Latin pada URI otpauth.
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
}
