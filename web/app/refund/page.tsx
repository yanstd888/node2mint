import { headers } from 'next/headers';
import LegalLayout from '@/components/LegalLayout';

const COPY = {
  id: {
    title: 'Kebijakan Pengembalian Dana',
    subtitle: 'Node2Mint dioperasikan oleh seorang pengembang individual. Pertanyaan pengembalian dana dikirim melalui halaman kontak.',
    sections: [
      ['Ruang lingkup', 'Halaman ini menjelaskan alur yang didukung sistem, bukan jaminan pengembalian dana di luar hukum yang berlaku. Pengembalian dana pasar digital dikreditkan ke dompet pembeli oleh sistem; halaman ini tidak menjanjikan pengembalian ke metode pembayaran asal.'],
      ['Sengketa pasar digital', 'Pembeli dapat membuka satu sengketa untuk pesanan berstatus PAID atau PROCESSING, atau dalam 7 hari setelah pesanan COMPLETED. Sertakan alasan dan bukti yang relevan. Selama sengketa terbuka, status pesanan menjadi DISPUTED dan dana dalam rekening penampungan tidak dilepas. Penjual dapat mengirim tanggapan melalui alur sengketa.'],
      ['Keputusan dan pengembalian pasar digital', 'Jika keputusan mendukung pembeli dan saldo toko penjual mencukupi, sistem mengembalikan seluruh subtotal pesanan ke dompet pembeli. Sistem tidak melakukan pengembalian dana parsial. Jika saldo toko tidak cukup, sengketa tetap dalam peninjauan manual; hasil dan waktu penyelesaiannya belum dipublikasikan. Keputusan admin bukan pengganti hak yang diwajibkan hukum dan harus menyertakan catatan penyelesaian pada sengketa bila tersedia.'],
      ['Produk digital dan pengiriman segera', 'Produk digital dapat tersedia segera setelah pembelian. Akses atau pengunduhan tidak otomatis menghilangkan hak pembeli untuk membuka sengketa dalam jendela sistem di atas. Namun, sifat pengiriman digital dan bukti akses dapat dipertimbangkan saat sengketa dinilai.'],
      ['Token dan galat penyedia', 'Token yang berhasil digunakan untuk respons model tidak dapat dikonversi menjadi uang. Jangan anggap semua galat penyedia sebagai penggunaan normal atau semua galat sebagai pengembalian dana otomatis: kode yang ada melakukan rekonsiliasi penggunaan terukur, tetapi tidak menetapkan kebijakan publik untuk kredit atau pengembalian dana atas kegagalan penyedia. Laporkan galat dengan waktu, model, dan ID permintaan bila tersedia agar catatan penggunaan dapat ditinjau. Operator perlu menetapkan remediasi tertulis untuk galat penyedia sebelum menjanjikan hasil.'],
      ['Paket token yang belum digunakan', 'Kode saat ini tidak menetapkan hak pengembalian dana mandiri untuk paket token yang belum digunakan. Anda dapat meminta peninjauan melalui Pusat bantuan dengan ID transaksi dan bukti pembayaran; permintaan tersebut bukan janji bahwa pengembalian dana akan disetujui.'],
      ['Biaya penarikan 5%', 'Sistem gerbang menghitung biaya penarikan dari tarif konfigurasi dan menggunakan 5% sebagai nilai default bila konfigurasi tidak valid. Sebelum Anda mengirim permintaan, sistem menampilkan biaya dan jumlah bersih yang akan dikirim; nilai pada layar konfirmasi yang berlaku untuk permintaan Anda. Jangan gunakan halaman ini sebagai janji bahwa tarif selalu 5% sampai operator menetapkan tarif kontraktual yang tetap.'],
      ['Banding dan bantuan', 'Tidak ada endpoint banding terpisah dalam kode saat ini. Jika Anda menilai ada kesalahan prosedur atau bukti baru, hubungi Pusat bantuan dengan ID pesanan/sengketa, alasan, dan bukti. Permintaan dapat ditinjau secara manual, tetapi halaman ini tidak menjanjikan tingkat banding kedua, SLA, atau diskresi tanpa kriteria. Hak yang diwajibkan hukum tetap berlaku.'],
    ],
  },
  en: {
    title: 'Refund Policy',
    subtitle: 'Node2Mint is operated by an individual developer. Send refund questions through the contact page.',
    sections: [
      ['Scope', 'This page explains system-supported flows, not a guarantee of refunds beyond applicable law. The system credits digital-marketplace refunds to the buyer’s wallet; this page does not promise a refund to the original payment method.'],
      ['Digital-marketplace disputes', 'A buyer may open one dispute for an order in PAID or PROCESSING status, or within 7 days after an order is COMPLETED. Include relevant reasons and evidence. While a dispute is open, the order status becomes DISPUTED and escrow funds are not released. A seller may submit a response through the dispute flow.'],
      ['Digital-marketplace decisions and refunds', 'If a decision supports the buyer and the seller’s store balance is sufficient, the system returns the full order subtotal to the buyer’s wallet. The system does not issue partial refunds. If the store balance is insufficient, the dispute remains under manual review; the outcome and resolution timing have not been published. An admin decision does not replace mandatory legal rights and should include a resolution record on the dispute when available.'],
      ['Digital products and immediate delivery', 'Digital products may be available immediately after purchase. Access or download does not automatically remove a buyer’s right to open a dispute within the system window above. However, the nature of digital delivery and evidence of access may be considered when a dispute is assessed.'],
      ['Tokens and provider errors', 'Tokens successfully used for a model response cannot be converted into money. Do not treat every provider error as normal usage or every error as an automatic refund: the existing code reconciles measured usage, but it does not establish a public policy for credits or refunds caused by provider failure. Report errors with the time, model, and request ID when available so usage records can be reviewed. The operator needs to establish written remediation for provider errors before promising an outcome.'],
      ['Unused token packages', 'The current code does not establish a self-service refund right for unused token packages. You may request review through the Help Center with a transaction ID and payment evidence; that request is not a promise that a refund will be approved.'],
      ['5% withdrawal fee', 'The gateway system calculates the withdrawal fee from a configured rate and uses 5% as the default when the configuration is invalid. Before you submit a request, the system shows the fee and net amount to be sent; the value on the confirmation screen applies to your request. Do not treat this page as a promise that the rate will always be 5% until the operator sets a fixed contractual rate.'],
      ['Appeals and help', 'There is no separate appeal endpoint in the current code. If you believe there was a procedural error or have new evidence, contact the Help Center with the order or dispute ID, reasons, and evidence. A request may be reviewed manually, but this page does not promise a second appeal level, SLA, or discretion without criteria. Mandatory legal rights remain in effect.'],
    ],
  },
} as const;

export default async function RefundPage() {
  const copy = (await headers()).get('x-locale') === 'en' ? COPY.en : COPY.id;

  return <LegalLayout title={copy.title} subtitle={copy.subtitle}>
    {copy.sections.map(([heading, body]) => <section key={heading}><h2 className="font-medium text-text">{heading}</h2><p>{body}</p></section>)}
  </LegalLayout>;
}