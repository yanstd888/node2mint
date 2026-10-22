import { headers } from 'next/headers';
import LegalLayout from '@/components/LegalLayout';

const PDP_SOURCE = 'https://peraturan.bpk.go.id/Details/229798/uu-no-27-tahun-2022-10';

const COPY = {
  id: {
    title: 'Kebijakan Privasi',
    subtitle: 'Node2Mint dioperasikan oleh seorang pengembang individual. Pertanyaan terkait data pribadi dapat dikirim melalui halaman kontak.',
    sections: [
      ['Data yang diproses', ['Kami memproses data pendaftaran (nama, email, dan kata sandi), catatan transaksi, penggunaan API (termasuk jumlah token per kunci API), serta informasi teknis yang diperlukan untuk menjalankan, mengamankan, dan mengaudit layanan (alamat IP yang diringkas secara kriptografis, agen pengguna, dan ID sesi). Kami tidak meminta lokasi GPS, daftar kontak, kamera, atau mikrofon.']],
      ['Prompt, respons, dan penyedia AI', ['Setiap permintaan API atau ruang uji dapat memuat prompt, riwayat pesan, konten teks, nama model, serta parameter permintaan. Jangan kirim data pribadi sensitif, rahasia, atau data pihak lain tanpa dasar yang sah. Gerbang API meneruskan isi permintaan tersebut ke penyedia AI aktif yang dipetakan untuk model yang Anda pilih. Jika penyedia pertama gagal, gerbang API dapat mencoba penyedia aktif lain yang melayani model itu; akibatnya, isi permintaan dapat diterima oleh lebih dari satu penyedia.', 'Nama dan endpoint penyedia dikelola sebagai konfigurasi aktif per model dan dapat berubah. Kode saat ini tidak menyediakan daftar penyedia per model kepada pengguna umum. Karena itu, kami tidak dapat menyatakan penyedia tertentu menerima setiap permintaan atau menjanjikan jangka waktu penyimpanan penyedia. Penanganan dan penyimpanan oleh masing-masing penyedia tunduk pada pemberitahuan privasi dan ketentuan penyedia tersebut. Sebelum mengandalkan layanan untuk data sensitif, operator perlu memublikasikan identitas penyedia untuk setiap model dan tautan kebijakannya.']],
      ['Tujuan penggunaan', ['Data digunakan untuk autentikasi dan otorisasi akun/kunci API; pemrosesan pembayaran, penahanan dana (rekening penampungan) pasar digital, dan dompet; penghitungan penggunaan token; pencegahan penyalahgunaan; dukungan pengguna; serta kewajiban operasional dan audit. Kami tidak menjual, memperdagangkan, atau menyewakan data pribadi kepada platform iklan.']],
      ['Dompet dan token API', ['Dompet dan token API adalah sistem nilai yang terpisah. Saldo dompet mencatat isi saldo, pembelian pasar digital, komisi, dan penarikan. Token API mencatat kuota untuk panggilan model. Penggunaan API tidak otomatis memotong saldo dompet, dan pembelian pasar digital tidak memakai kuota token.']],
      ['Keamanan dan akses', ['Kata sandi disimpan sebagai ringkasan kriptografis Argon2. Kunci API disimpan sebagai ringkasan kriptografis dan hanya ditampilkan saat dibuat. Kunci penyedia/gerbang pembayaran disimpan terenkripsi di sisi server. Akses data dibatasi untuk kebutuhan dukungan teknis dan transaksi, dengan catatan audit.']],
      ['Hak subjek data dan PIA', ['UU Nomor 27 Tahun 2022 tentang Pelindungan Data Pribadi mengatur hak subjek data dan, untuk pemrosesan tertentu yang berisiko tinggi, Penilaian Dampak Pelindungan Data Pribadi pada Pasal 34. “PIA” bukan nama permintaan hak subjek data dan bukan pengganti verifikasi identitas. Untuk meminta akses, perbaikan, penghapusan sesuai hukum, atau informasi pemrosesan, hubungi kami dengan subjek “Permintaan PDP” dan bukti identitas yang proporsional. Kami akan menilai permintaan sesuai kewajiban hukum yang berlaku; halaman ini tidak menetapkan SLA atau pengecualian retensi yang belum diverifikasi.']],
      ['Cookie', ['Situs menggunakan cookie esensial untuk sesi, preferensi bahasa, dan persetujuan spanduk cookie. Kami tidak menyatakan penggunaan atau ketiadaan pelacak lain di luar yang dapat diverifikasi dari implementasi saat ini. Perubahan material pada praktik ini akan diperbarui di halaman ini.']],
      ['Pembayaran', ['Isi saldo dapat memakai gerbang yang aktif, transfer manual, atau USDT TRC-20. Kami menerima data yang diperlukan untuk mencocokkan transaksi, seperti status, jumlah, metode, dan referensi transaksi. Kami tidak menyimpan nomor kartu penuh, CVC, atau PIN. Data pembayaran tidak diteruskan ke penyedia AI sebagai bagian dari permintaan model.']],
      ['Perubahan', ['Pembaruan kebijakan ini dipublikasikan pada halaman ini dengan tanggal berlaku. Tidak ada janji periode pemberitahuan khusus yang dibuat oleh halaman ini.']],
    ],
    sourceHeading: 'Hak subjek data dan PIA',
    source: 'Sumber resmi: UU No. 27 Tahun 2022 tentang Pelindungan Data Pribadi.',
  },
  en: {
    title: 'Privacy Policy',
    subtitle: 'Node2Mint is operated by an individual developer. Send personal-data questions through the contact page.',
    sections: [
      ['Data we process', ['We process registration data (name, email address, and password), transaction records, API use (including token counts per API key), and technical information needed to operate, secure, and audit the service (cryptographically hashed IP addresses, user-agent data, and session IDs). We do not request GPS location, contact lists, camera access, or microphone access.']],
      ['Prompts, responses, and AI providers', ['Each API or playground request may contain prompts, message history, text content, a model name, and request parameters. Do not send sensitive personal data, confidential information, or another party’s data without a lawful basis. The API gateway sends those request contents to the active AI provider mapped to the model you choose. If the primary provider fails, the API gateway may try another active provider that serves that model; as a result, more than one provider may receive request contents.', 'Provider names and endpoints are managed as active configuration for each model and may change. The current code does not provide a list of providers for each model to general users. Therefore, we cannot state that a particular provider receives every request or promise a provider retention period. Each provider’s handling and retention are subject to that provider’s privacy notice and terms. Before relying on the service for sensitive data, the operator needs to publish the provider identity and policy link for each model.']],
      ['Purposes of use', ['We use data for account and API-key authentication and authorization; payment processing, digital-marketplace escrow, and wallets; token-use calculation; abuse prevention; user support; and operational and audit obligations. We do not sell, trade, or rent personal data to advertising platforms.']],
      ['Wallet and API tokens', ['Wallets and API tokens are separate value systems. Wallet balances record top-ups, digital-marketplace purchases, commissions, and withdrawals. API tokens record quota for model calls. API use does not automatically deduct wallet balance, and digital-marketplace purchases do not use token quota.']],
      ['Security and access', ['Passwords are stored as Argon2 cryptographic hashes. API keys are stored as cryptographic hashes and shown only when created. Provider and payment-gateway keys are stored encrypted on the server side. Data access is limited to technical-support and transaction needs, with audit records.']],
      ['Data-subject rights and DPIA', ['Law No. 27 of 2022 on Personal Data Protection governs data-subject rights and, for certain high-risk processing, a Personal Data Protection Impact Assessment in Article 34. “DPIA” is not the name of a data-subject-rights request and does not replace identity verification. To request access, correction, deletion as permitted by law, or processing information, contact us with the subject “PDP Request” and proportionate proof of identity. We will assess requests under applicable legal obligations; this page does not set an SLA or unverified retention exception.']],
      ['Cookies', ['The site uses essential cookies for sessions, language preference, and cookie-banner consent. We do not claim the presence or absence of trackers beyond what can be verified from the current implementation. Material changes to these practices will be updated on this page.']],
      ['Payments', ['Top-ups may use an active gateway, manual transfer, or USDT TRC-20. We receive data needed to match transactions, such as status, amount, method, and transaction reference. We do not store full card numbers, CVCs, or PINs. Payment data is not sent to AI providers as part of model requests.']],
      ['Changes', ['Updates to this policy are published on this page with an effective date. This page does not promise a specific notice period.']],
    ],
    sourceHeading: 'Data-subject rights and DPIA',
    source: 'Official source: Law No. 27 of 2022 on Personal Data Protection.',
  },
} as const;

export default async function PrivacyPage() {
  const copy = (await headers()).get('x-locale') === 'en' ? COPY.en : COPY.id;

  return <LegalLayout title={copy.title} subtitle={copy.subtitle}>
    {copy.sections.map(([heading, paragraphs]) => <section key={heading}><h2 className="font-medium text-text">{heading}</h2>{paragraphs.map((paragraph, index) => <p className={index ? 'mt-3' : undefined} key={paragraph}>{paragraph}</p>)}{heading === copy.sourceHeading && <p className="mt-3"><a href={PDP_SOURCE} className="text-cyan-300 hover:text-cyan-200">{copy.source}</a></p>}</section>)}
  </LegalLayout>;
}