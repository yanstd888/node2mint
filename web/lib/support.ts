// Satu tempat untuk jalur bantuan. F-06 (juri QA): Halaman bantuan/refund/lupa-password
// berakhir buntu — tidak ada form, email tidak bisa diklik, dan "hubungi admin"
// tanpa tujuan. Alamat ini dipakai oleh Help, FAQ, Refund, Terms, Privacy, 404,
// dan forgot-password supaya tidak ada lagi jalan buntu.
export const SUPPORT_EMAIL = 'support@node2mint.com';

export const supportMailto = (subject: string) => `mailto:${SUPPORT_EMAIL}?subject=${encodeURIComponent(subject)}`;
