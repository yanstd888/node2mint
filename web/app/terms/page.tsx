import { headers } from 'next/headers';
import LegalLayout from '@/components/LegalLayout';

const COPY = {
  id: {
    title: 'Syarat Penggunaan',
    subtitle: 'Node2Mint dioperasikan oleh seorang pengembang individual. Pertanyaan dikirim melalui halaman kontak.',
    sections: [
      ['Layanan', 'Node2Mint menyediakan akses model AI, paket token, dompet digital, dan pasar produk digital. Operator adalah perorangan. Layanan disediakan sebagaimana tersedia, dengan hak dan kewajiban yang tidak dapat dikesampingkan menurut hukum tetap berlaku.'],
      ['Dompet dan token API', 'Dompet menyimpan saldo untuk isi saldo, pembelian pasar digital, komisi, dan penarikan. Token API adalah kuota untuk panggilan model, bukan uang dan tidak dapat ditarik atau dikonversi menjadi saldo dompet. Saldo dompet tidak dipakai otomatis untuk tagihan API. Satu saldo token API aktif dipakai lintas model yang diizinkan; setiap kunci API mencatat penggunaan dan dapat memiliki batas aksesnya sendiri.'],
      ['Akun dan keamanan', 'Anda bertanggung jawab atas aktivitas akun, kata sandi, kunci API, dan materi yang Anda unggah. Jaga kerahasiaan kunci API; jika mencurigai kebocoran, segera cabut kunci dari halaman Kunci API. Dilarang menggunakan layanan untuk kegiatan ilegal, materi terlarang, pelanggaran hak pihak lain, atau serangan terhadap infrastruktur.'],
      ['Pasar digital dan produk digital', 'Produk pasar digital dapat berupa prompt, dataset, alat, template, dan layanan digital. Penjual bertanggung jawab atas hak atau izin atas barang yang dijual, akurasi stok, dan kelengkapan informasi. Pembeli membayar dari dompet. Sistem menahan dana dalam rekening penampungan sampai pembeli mengonfirmasi penerimaan atau kondisi pelepasan rekening penampungan sistem terpenuhi. Penjual dilarang menyalin, meniru, atau menjual produk pihak lain tanpa hak.'],
      ['Penjual, rujukan, dan afiliasi', 'Rujukan yang memenuhi pemeriksaan anti-penyalahgunaan dapat memberi token API dan masa aktif saat pendaftaran menggunakan kode rujukan. Komisi rujukan tambahan dari pembelian paket token, bila tersedia, adalah program terpisah dan bukan saldo dompet. Afiliasi dapat memberi komisi saldo dompet dari penjualan pasar digital yang dikaitkan dengan tautan afiliasi. Ketentuan detail, termasuk kelayakan dan biaya penarikan, ditampilkan dalam layanan saat transaksi dilakukan.'],
      ['Keamanan dan moderasi', 'Platform dapat meninjau, menyembunyikan, atau menghapus konten yang diduga melanggar hukum atau ketentuan ini, serta membatasi akun untuk melindungi pengguna dan platform. Kami tidak menjamin semua konten akan ditinjau.'],
      ['Pajak', 'Anda bertanggung jawab melaporkan dan membayar pajak atas pendapatan dari pasar digital, komisi afiliasi, atau hadiah rujukan sesuai ketentuan pajak yang berlaku.'],
      ['Perubahan, hukum, dan sengketa', 'Kami dapat memperbarui layanan atau syarat ini dengan memublikasikan versi baru di halaman ini. Syarat ini tunduk pada hukum Indonesia sejauh diperbolehkan hukum. Untuk masalah pesanan, pengembalian dana, atau akun, gunakan jalur sengketa atau Pusat bantuan terlebih dahulu dan sertakan ID transaksi serta bukti yang relevan. Halaman ini tidak menghapus hak hukum yang bersifat wajib, tidak mewajibkan pelepasan mediasi atau banding, dan tidak menetapkan forum pengadilan eksklusif tanpa dasar yang dipublikasikan.'],
    ],
  },
  en: {
    title: 'Terms of Use',
    subtitle: 'Node2Mint is operated by an individual developer. Send questions through the contact page.',
    sections: [
      ['Services', 'Node2Mint provides access to AI models, token packages, a digital wallet, and a digital-product marketplace. The operator is an individual. The services are provided as available; rights and obligations that cannot be waived under applicable law remain in effect.'],
      ['Wallet and API tokens', 'The wallet holds balances for top-ups, digital-marketplace purchases, commissions, and withdrawals. API tokens are quota for model calls, not money, and cannot be withdrawn or converted into wallet balance. Wallet balances are not used automatically for API billing. One active API-token balance is used across permitted models; each API key records usage and may have its own access limits.'],
      ['Account and security', 'You are responsible for account activity, your password, API keys, and materials you upload. Keep API keys confidential; if you suspect exposure, revoke the key promptly from the API Keys page. You must not use the services for illegal activities, prohibited material, infringement of others’ rights, or attacks on infrastructure.'],
      ['Digital marketplace and digital products', 'Digital-marketplace products may include prompts, datasets, tools, templates, and digital services. Sellers are responsible for the rights or permissions for goods they sell, stock accuracy, and complete information. Buyers pay from their wallet. The system holds funds in escrow until the buyer confirms receipt or the system’s escrow-release conditions are met. Sellers must not copy, imitate, or sell another party’s products without rights.'],
      ['Sellers, referrals, and affiliates', 'Referrals that pass anti-abuse checks may grant API tokens and an active period at registration when a referral code is used. Additional referral commissions from token-package purchases, when available, are a separate program and not wallet balance. Affiliates may earn wallet-balance commissions from digital-marketplace sales attributed to an affiliate link. Detailed terms, including eligibility and withdrawal fees, are shown in the service when the transaction is made.'],
      ['Security and moderation', 'The platform may review, hide, or remove content suspected of violating the law or these terms, and may restrict accounts to protect users and the platform. We do not guarantee that all content will be reviewed.'],
      ['Taxes', 'You are responsible for reporting and paying taxes on income from the digital marketplace, affiliate commissions, or referral rewards in accordance with applicable tax rules.'],
      ['Changes, law, and disputes', 'We may update the services or these terms by publishing a new version on this page. These terms are governed by Indonesian law to the extent permitted by law. For order, refund, or account issues, first use the dispute channel or Help Center and include the transaction ID and relevant evidence. This page does not remove mandatory legal rights, require a waiver of mediation or appeal, or set an exclusive court forum without a published basis.'],
    ],
  },
} as const;

export default async function TermsPage() {
  const copy = (await headers()).get('x-locale') === 'en' ? COPY.en : COPY.id;

  return <LegalLayout title={copy.title} subtitle={copy.subtitle}>
    {copy.sections.map(([heading, body]) => <section key={heading}><h2 className="font-medium text-text">{heading}</h2><p>{body}</p></section>)}
  </LegalLayout>;
}