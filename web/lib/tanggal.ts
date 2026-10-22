// Tanggal ikut bahasa UI. lib/utils.tanggal() masih id-ID tetap (dipakai halaman lain);
// tambahkan parameter `lang` di sana kalau pemakai bertambah.
export const tanggalLang = (v: unknown, lang: string, denganWaktu = false) => {
  if (!v) return '-';
  const d = new Date(String(v));
  if (Number.isNaN(d.getTime())) return '-';
  return d.toLocaleDateString(lang === 'id' ? 'id-ID' : 'en-US', {
    day: 'numeric', month: 'short', year: 'numeric',
    ...(denganWaktu ? { hour: '2-digit', minute: '2-digit' } : {}),
  });
};
