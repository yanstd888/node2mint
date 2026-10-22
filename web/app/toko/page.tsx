'use client';
// Toko seller: buka/ubah toko, kelola produk (tambah, sunting, hapus, boost).
import { useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { api } from '@/lib/api';
import { patch, post, pesanError } from '@/lib/http';
import { useFetch } from '@/lib/useApi';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Dialog } from '@/components/ui/dialog';
import { Badge } from '@/components/ui/badge';
import { Bintang } from '@/components/ui/bintang';
import { useToast } from '@/components/ui/toast';
import { ImageField } from '@/components/ui/ImageField';
import { Pagination, useClientPagination } from '@/components/ui/Pagination';
import { rupiah, angka, tanggal, asList } from '@/lib/utils';
import { sisaHari } from '@/lib/boost-cost';
import { useT, type Key } from '@/lib/i18n';
import { Store as StoreIcon, Plus, Trash2, Rocket, MessageCircle, ShieldCheck } from 'lucide-react';

type Toko = { id: string; name: string; description?: string; logo?: string; is_active?: boolean; is_verified?: boolean; wallet_balance?: number; created_at?: string };
type Produk = { id: string; title: string; description?: string; price: number; stock: number; category_id?: string; category?: string; is_active?: boolean; is_instant?: boolean; allow_affiliate?: boolean; affiliate_percent?: number };
type TelegramSeller = { connected: boolean; telegram_username?: string; notifications_enabled?: boolean };
type ProdukBoost = Produk & { is_boosted?: boolean; boosted_until?: string };
// Info toko + harga efektif dari server (bukan angka hardcode di UI).
type InfoToko = Toko & {
  verified_until?: string;
  boost_price_per_day?: number;
  boost_price_one_off?: number;
  boost_max_per_day?: number;
  verified_monthly_price?: number;
  boost_credits?: number;
  free_boost_limit?: number;
};
// Langganan verified + pengaturan perpanjangan otomatis.
type Langganan = {
  id: string;
  status: string;
  price: number;
  auto_renew: boolean;
  renew_days: number;
  current_period_end: string;
  cancel_at_period_end?: boolean;
  last_error?: string;
};
type Kategori = { id: string; name: string; slug?: string };
type Ulasan = { id: string; buyer_name?: string; rating?: number; comment?: string; created_at?: string };
type PesananMasuk = { id: string; quantity: number; subtotal: number; seller_payout?: number; status: string; created_at?: string; product_title: string; is_instant?: boolean };

const statusLabel = (status: string, t: ReturnType<typeof useT>['t']) => ({
  PAID: t('ord.statusPaid'), PROCESSING: t('ord.statusProcessing'), COMPLETED: t('ord.statusCompleted'),
  DISPUTED: t('ord.statusDisputed'), REFUNDED: t('ord.statusRefunded'), CANCELLED: t('ord.statusCancelled'),
}[status] ?? t('ord.statusUnknown'));

const KOSONG = { title: '', description: '', category_id: '', price: '', stock: '', image_url: '', is_instant: false, instant_items: '', allow_affiliate: false, affiliate_percent: '' };
const MIN_DESKRIPSI = 20;
const MAX_DESKRIPSI = 2000;
// Ambang stok default sama dengan server: tiga unit. Nilai per-produk dapat
// ditambahkan saat endpoint pengaturan stok dikembangkan.
const STOK_RENDAH = 3;

// Satu baris nonblank = satu barang. Hitungan yang ditampilkan harus sama
// dengan yang dihitung server (instantItems di shop.go).
const barisInstan = (t: string) => t.split('\n').filter((x) => x.trim() !== '').length;
const MAX_INSTAN = 10000;

const CATEGORY_LABEL: Record<string, Key> = {
  'premiums-app': 'mkt.catPremiumsApp',
  'soul-skill': 'mkt.catSoulSkill',
  'tools-software': 'mkt.catToolsSoftware',
  ebook: 'mkt.catEbook',
  akun: 'mkt.catAccounts',
  others: 'mkt.catOthers',
};

// Tab halaman toko. Urutan ini juga urutan tombolnya.
const TAB_KEYS = ['toko', 'produk', 'pesanan', 'ulasan'] as const;
type TabKey = (typeof TAB_KEYS)[number];
// Peta eksplisit (bukan `sel.tab${...}`): kunci kamus harus literal supaya
// salah ketik jadi error TypeScript, bukan teks kosong di layar.
const TAB_LABEL = {
  toko: 'sel.tabToko', produk: 'sel.tabProduk', pesanan: 'sel.tabPesanan', ulasan: 'sel.tabUlasan',
} as const satisfies Record<TabKey, string>;


export default function TokoPage() {
  const { t } = useT();
  const labelKategori = (category: Kategori) => {
    const key = CATEGORY_LABEL[category.slug ?? ''];
    return key ? t(key) : category.name;
  };
  const toko = useFetch<unknown>('/store');
  const kategori = useFetch<{ categories?: Kategori[] }>('/market/categories');
  const daftarKategori = kategori.data?.categories ?? [];
  // Ulasan menempel pada toko (bukan per produk). Menunggu id toko dulu supaya
  // tidak memanggil endpoint tanpa store_id (yang membalas SEMUA ulasan).
  const storeEnvelope = toko.data as { store?: InfoToko } | null;
  const info = storeEnvelope?.store ?? (toko.data as InfoToko | null);
  const ulasan = useFetch<unknown>(`/market/reviews?store_id=${info?.id ?? ''}`, !info?.id);
  const [form, setForm] = useState({ name: '', description: '', logo: '' });
  const [bukaBusy, setBukaBusy] = useState(false);
  const [verifyBusy, setVerifyBusy] = useState(false);
  const [confirmVerify, setConfirmVerify] = useState(false);
  // Popup konfirmasi boost: target produk + jumlah kredit yang dipakai.
  const [boostTarget, setBoostTarget] = useState<ProdukBoost | null>(null);
  const [boostCount, setBoostCount] = useState(1);
  const [boostModal, setBoostModal] = useState(false);
  // Langganan verified: konfirmasi sebelum mengubah auto-renew / memperpanjang.
  const [subAksi, setSubAksi] = useState<'auto-on' | 'auto-off' | 'renew' | null>(null);
  const [subBusy, setSubBusy] = useState(false);
  const [dialog, setDialog] = useState<{ open: boolean; id?: string }>({ open: false });
  const [item, setItem] = useState(KOSONG);
  const [simpanBusy, setSimpanBusy] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);
  const [nominal, setNominal] = useState('');
  const [pindahBusy, setPindahBusy] = useState(false);
  const toast = useToast();
  // Hapus produk permanen — konfirmasi dua langkah, pola sama dengan API Keys & refund admin.
  const [hapusId, setHapusId] = useState<string | null>(null);
  // Tab halaman: satu halaman panjang berisi toko+review+form+produk+pesanan
  // bikin bingung begitu produk banyak. Pisah jadi tab, tab aktif disimpan di
  // URL (?tab=) supaya bisa di-bookmark dan tidak reset saat reload.
  const [tab, setTab] = useState<TabKey>('produk');
  useEffect(() => {
    const q = new URLSearchParams(location.search).get('tab');
    if (q && TAB_KEYS.includes(q as TabKey)) setTab(q as TabKey);
  }, []);
  const gantiTab = (k: TabKey) => {
    setTab(k);
    const u = new URL(location.href);
    u.searchParams.set('tab', k);
    history.replaceState(null, '', u);
  };
  const hubungkanTelegramSeller = async () => {
    setSellerTelegramBusy(true);
    try {
      const res = await post<{ url?: string }>('/seller/telegram/link', {});
      if (res.data.url) window.location.assign(res.data.url);
    } catch (e) {
      toast.err(pesanError(e, t('sel.telegramErr')));
    } finally {
      setSellerTelegramBusy(false);
    }
  };
  const putuskanTelegramSeller = async () => {
    setSellerTelegramBusy(true);
    try {
      await post('/seller/telegram/unlink', {});
      await sellerTelegram.reload();
    } catch (e) {
      toast.err(pesanError(e, t('sel.telegramErr')));
    } finally {
      setSellerTelegramBusy(false);
    }
  };

  const ada = !!info?.id;
  const sellerTelegram = useFetch<TelegramSeller>('/seller/telegram/status', !ada);
  const [sellerTelegramBusy, setSellerTelegramBusy] = useState(false);
  // Sunting toko mulai dari nilai tersimpan, bukan form kosong: logo yang
  // sudah diunggah harus tampak di ImageField (kalau kosong, menekan Simpan
  // mengirim logo:undefined dan server mempertahankan yang lama — user tidak
  // pernah tahu logonya masih ada).
  useEffect(() => {
    if (!info?.id) return;
    setForm({ name: info.name || '', description: info.description || '', logo: info.logo || '' });
  }, [info?.id, info?.name, info?.description, info?.logo]);
  // Hanya kolom saldo toko yang ada di API; dana escrow tidak dipisah di skema,
  // jadi angkanya = yang tersedia untuk dipindah. Escrow tampil per pesanan.
  const saldoToko = Number(info?.wallet_balance) || 0;
  const nominalNum = Number(nominal) || 0;
  const nominalSalah = nominal !== '' && nominalNum > saldoToko;
  const nominalSah = nominalNum > 0 && !nominalSalah;
  // Produk hanya dimuat kalau tokonya sudah ada; tanpa ini user tanpa toko
  // menerima 404 "daftarkan toko dulu" sebagai error merah (produk.err).
  const produk = useFetch<unknown>('/store/products', !ada);
  // Status langganan verified (sisa masa + auto-renew). Hanya kalau toko ada.
  const langganan = useFetch<{ subscription?: Langganan | null }>('/store/subscription', !ada);
  const sub = langganan.data?.subscription ?? null;
  const daftar = asList<ProdukBoost>(produk.data, 'products');
  // Pesanan masuk toko (GET /api/store/orders, terpaginasi). 404 "toko belum
  // terdaftar" tampil sebagai pesan biasa, bukan error merah.
  const [poPage, setPoPage] = useState(1);
  const pesananMasuk = useFetch<{ has_more?: boolean }>(`/store/orders?page=${poPage}`, !ada);
  const daftarPesanan = asList<PesananMasuk>(pesananMasuk.data, 'orders');
  // Rata-rata & jumlah ulasan datang dari gateway (bukan dihitung ulang di sini):
  // dua tempat menghitung rata-rata = dua angka yang bisa berbeda.
  const ulasanRingkas = ulasan.data as { average?: number; count?: number } | null;
  const rata = Math.round(Number(ulasanRingkas?.average) || 0);
  const daftarUlasan = asList<Ulasan>(ulasan.data, 'reviews');
  const halamanUlasan = useClientPagination(daftarUlasan);
  const halamanProduk = useClientPagination(daftar);

  // Harga efektif: pakai nilai dari server kalau ada, kalau tidak jatuh ke
  // default yang sama dengan backend supaya popup tidak pernah menampilkan
  // angka yang berbeda dari yang dipotong.
  const hargaVerified = Number(info?.verified_monthly_price) || 15000;
  const sisaBoost = Number(info?.boost_credits) || 0;
  const bukaPembayaran = (kind: 'store_verify' | 'boost_pack', id: string, amount: number, label: string) => location.assign(`/payment?kind=${kind}&id=${encodeURIComponent(id)}&amount=${amount}&label=${encodeURIComponent(label)}`);

  const simpanToko = async (e: React.FormEvent) => {
    e.preventDefault();
    setBukaBusy(true);
    try {
      // logo kosong dikirim sebagai undefined → server COALESCE, tidak menghapus logo lama.
      await post('/store', { name: form.name.trim(), description: form.description.trim(), logo: form.logo || undefined });
      toast.ok(ada ? t('sel.toastStoreUpdated') : t('sel.toastStoreOpened'));
      toko.reload();
      produk.reload();
    } catch (e) {
      toast.err(pesanError(e, t('sel.errStore')));
    } finally {
      setBukaBusy(false);
    }
  };

  const beliVerified = async () => bukaPembayaran('store_verify', 'verified', hargaVerified, t('sel.verifiedLabel'));

  // Auto-renew / perpanjang. Semua lewat konfirmasi: menyalakan auto-renew
  // berarti user setuju dipotong otomatis tiap periode, jadi tidak boleh
  // terjadi karena salah klik.
  const ubahAutoRenew = async (nyala: boolean) => {
    setSubBusy(true);
    try {
      await post('/store/subscription', { auto_renew: nyala });
      toast.ok(nyala ? t('sel.toastAutoOn') : t('sel.toastAutoOff'));
      setSubAksi(null);
      langganan.reload();
      toko.reload();
    } catch (e) {
      toast.err(pesanError(e, t('sel.errSub')));
    } finally {
      setSubBusy(false);
    }
  };

  const perpanjangSekarang = async () => {
    setSubBusy(true);
    try {
      await post('/store/subscription/renew', {});
      toast.ok(t('sel.toastRenewed'));
      setSubAksi(null);
      langganan.reload();
      toko.reload();
    } catch (e) {
      toast.err(pesanError(e, t('sel.errSub')));
    } finally {
      setSubBusy(false);
    }
  };

  // Transfer saldo toko → dompet user. Server memeriksa kepemilikan + kecukupan
  // saldo di dalam satu UPDATE, jadi angka di layar cuma cek cepat; balapan
  // tetap kalah dengan aman dan pesannya datang dari gateway.
  const pindahSaldo = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!nominalSah) return;
    setPindahBusy(true);
    try {
      await post('/store/wallet/transfer', { amount: nominalNum });
      toast.ok(t('sel.transferOk', { amount: rupiah(nominalNum) }));
      setNominal('');
      toko.reload();
    } catch (e) {
      toast.err(pesanError(e, t('sel.transferErr')));
    } finally {
      setPindahBusy(false);
    }
  };

  const simpanProduk = async () => {
    setSimpanBusy(true);
    try {
      const body = {
        title: item.title.trim(),
        description: item.description.trim(),
        category_id: item.category_id,
        price: Number(item.price),
        // Stok produk instan ditentukan server dari jumlah isian; angka manual
        // di formulir tidak ikut dikirim supaya tidak ada dua sumber angka.
        stock: item.is_instant ? undefined : Number(item.stock),
        is_instant: item.is_instant,
        allow_affiliate: item.allow_affiliate,
        affiliate_percent: item.allow_affiliate ? Number(item.affiliate_percent) : 0,
        instant_items: item.is_instant ? item.instant_items.split('\n') : undefined,
        image_url: item.image_url || undefined,
      };
      if (dialog.id) await patch(`/store/products/${dialog.id}`, body);
      else await post('/store/products', body);
      toast.ok(dialog.id ? t('sel.toastProductUpdated') : t('sel.toastProductAdded'));
      setDialog({ open: false });
      setItem(KOSONG);
      produk.reload();
    } catch (e) {
      toast.err(pesanError(e, t('sel.errProduct')));
    } finally {
      setSimpanBusy(false);
    }
  };

  const hapus = async (id: string) => {
    setBusy(id);
    try {
      // Hapus LUNAK (server: deleted_at) — produk hilang dari katalog tapi
      // barisnya (pesanan/keranjang/link/boost/isian) tetap utuh, jadi Undo
      // cukup memanggil /restore. Server menolak 409 kalau produk sudah
      // punya pesanan; pesan itu datang lewat pesanError.
      await api.del(`/store/products/${id}`);
      toast.ok(t('sel.toastDeleted'), {
        label: t('sel.undo'),
        run: async () => {
          try { await post(`/store/products/${id}/restore`, {}); toast.ok(t('sel.toastRestored')); }
          catch (e) { toast.err(pesanError(e, t('sel.errRestore'))); }
          produk.reload();
        },
      });
      produk.reload();
    } catch (e) {
      toast.err(pesanError(e, t('sel.errDelete')));
    } finally {
      setBusy(null);
      setHapusId(null);
    }
  };

  // Boost: buka konfirmasi dulu (durasi + biaya nyata dari server), baru potong
  // dompet. Sebelumnya ini memanggil API tanpa konfirmasi dan tanpa memberi tahu
  // berapa yang dipotong — user bisa kehilangan saldo tanpa sadar.
  const bukaBoost = (p: ProdukBoost) => {
    if (sisaBoost <= 0) {
      document.getElementById('boost-packages')?.scrollIntoView({ behavior: 'smooth', block: 'center' });
      toast.err(t('sel.boostUnavailable'));
      return;
    }
    setBoostTarget(p);
    setBoostCount(1);
    setBoostModal(true);
  };

  const jalankanBoost = async () => {
    const target = boostTarget;
    if (!target) return;
    setBusy(`boost:${target.id}`);
    try {
      await post(`/store/products/${target.id}/boost`, { count: boostCount });
      toast.ok(t('sel.boostActivatedFor', { count: boostCount, hours: boostCount * 6 }));
      setBoostModal(false);
      setBoostTarget(null);
      produk.reload();
      toko.reload(); // saldo dompet berubah
    } catch (e) {
      toast.err(pesanError(e, t('sel.errBoost')));
    } finally {
      setBusy(null);
    }
  };

  const tandaiTerkirim = async (id: string) => {
    setBusy(`delivered:${id}`);
    try {
      await post(`/store/orders/${id}/delivered`, {});
      toast.ok(t('sel.toastDelivered'));
      pesananMasuk.reload();
    } catch (e) {
      toast.err(pesanError(e, t('sel.errDelivered')));
    } finally {
      setBusy(null);
    }
  };

  // Berkas TXT: satu baris = satu barang. Isi berkas menggantikan textarea, dan
  // <input type="file"> tidak bisa dikosongkan dari kode — nilai value-nya
  // direset manual lewat ref supaya berkas yang sama bisa dipilih dua kali.
  const berkasRef = useRef<HTMLInputElement>(null);
  const [namaBerkas, setNamaBerkas] = useState('');
  const bacaBerkas = async (f: File | undefined) => {
    if (!f) return;
    const isi = await f.text();
    setItem((prev) => ({ ...prev, instant_items: isi }));
    setNamaBerkas(f.name);
    if (berkasRef.current) berkasRef.current.value = '';
  };

  const jumlahInstan = barisInstan(item.instant_items);
  const instanSah = !item.is_instant || (jumlahInstan > 0 && jumlahInstan <= MAX_INSTAN);
  const deskripsiPanjang = item.description.trim().length;
  const sah = !!item.title.trim() && !!item.category_id
    && deskripsiPanjang >= MIN_DESKRIPSI && deskripsiPanjang <= MAX_DESKRIPSI
    && Number(item.price) > 0 && instanSah && (item.is_instant || Number(item.stock) >= 0)
    && (!item.allow_affiliate || (Number(item.affiliate_percent) >= 1 && Number(item.affiliate_percent) <= 100));

  return (
    <div className="p-6 md:p-10 max-w-4xl mx-auto">
      <header className="mb-8">
        <p className="label mb-1">{t('sel.kicker')}</p>
        <h1 className="text-2xl font-semibold tracking-tight">{t('sel.title')}</h1>
      </header>

      {toko.err && <p role="alert" className="mb-6 text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{toko.err}</p>}

      {/* Bar tab: hanya tampil kalau toko sudah ada — tanpa toko, yang relevan
          cuma form buka toko di tab "toko". */}
      {ada && (
        <div className="tab-pills mb-8" role="tablist" aria-label={t('sel.tabsAria')}>
          {TAB_KEYS.map((x) => (
            <button key={x} role="tab" aria-selected={tab === x} onClick={() => gantiTab(x)} className="tab-pill">
              {t(TAB_LABEL[x])}
              {x === 'pesanan' && daftarPesanan.length > 0 ? ` (${daftarPesanan.length})` : ''}
              {x === 'produk' && daftar.length > 0 ? ` (${daftar.length})` : ''}
            </button>
          ))}
        </div>
      )}

      {toko.loading ? (
        <div className="card p-5 mb-8"><div className="skel h-5 w-40 mb-3" /><div className="skel h-3 w-64" /></div>
      ) : ada && tab === 'toko' ? (
        <section aria-label={t('sel.storeInfoAria')} className="card p-5 mb-8">
          <div className="flex flex-wrap items-center gap-3">
            <h2 className="font-semibold text-lg">{info?.name}</h2>
            <Badge tone={info?.is_active ? 'ok' : 'danger'}>{info?.is_active ? t('sel.active') : t('sel.inactive')}</Badge>
            {/* Verified: tampilkan SISA masa, bukan cuma badge. Tanpa tanggal,
                seller tidak tahu kapan langganannya habis. */}
            {info?.is_verified ? (
              <span title={t('sel.verifiedTip')}>
                <Badge tone={sisaHari(info?.verified_until) <= 7 ? 'warn' : 'ok'} className="flex items-center gap-1">
                  <ShieldCheck className="h-3 w-3" aria-hidden="true" />
                  {info?.verified_until
                    ? t('sel.verifiedTill', { date: tanggal(info.verified_until), left: sisaHari(info.verified_until) })
                    : t('sel.verified')}
                </Badge>
              </span>
            ) : (
              <div className="flex flex-wrap items-center gap-2">
                <button type="button" disabled={verifyBusy} className="btn btn-sm" onClick={() => setConfirmVerify(true)}>{t('sel.buyVerifiedBtn')}</button>
                <Link href="/contact?topic=verified-gratis" className="inline-flex h-8 items-center rounded-lg border border-cyan-300/40 px-3 text-xs font-medium text-cyan-200 hover:bg-cyan-300/10">{t('sel.freeVerified')}</Link>
                <span className="text-xs text-mute">{t('sel.termsApply')}</span>
              </div>
            )}
          </div>
          <p className="text-sm text-mute mt-2">{info?.description || t('mkt.noDesc')}</p>
          <div className="mt-5 border-t border-border pt-5 space-y-3">
            <div className="flex flex-wrap items-center gap-3">
              <h3 className="label normal-case tracking-normal text-sm text-text">{t('sel.telegramH')}</h3>
              {sellerTelegram.data?.connected && <Badge tone="ok">{t('sel.telegramConnected', { user: sellerTelegram.data.telegram_username ? ` (@${sellerTelegram.data.telegram_username})` : '' })}</Badge>}
            </div>
            <p className="text-xs text-mute">{sellerTelegram.data?.connected ? t('sel.telegramConnected', { user: '' }) : t('sel.telegramNotConnected')}</p>
            {sellerTelegram.data?.connected ? (
              <Button size="sm" variant="outline" disabled={sellerTelegramBusy} onClick={putuskanTelegramSeller}>{t('sel.telegramUnlink')}</Button>
            ) : (
              <Button size="sm" disabled={sellerTelegramBusy} onClick={hubungkanTelegramSeller}>{t('sel.telegramConnect')}</Button>
            )}
          </div>
          <p className="text-xs text-mute mt-3 tabular-nums">
            {t('sel.sellerBalance', { amount: rupiah(info?.wallet_balance), date: tanggal(info?.created_at) })}
          </p>

          {/* Langganan Verified Store: sisa masa, durasi, dan perpanjangan
              otomatis. Semua angka dari server. */}
          {info?.is_verified && (
            <div className="mt-5 border-t border-border pt-5 space-y-3">
              <h3 className="label normal-case tracking-normal text-sm text-text">{t('sel.subH')}</h3>
              <dl className="flex flex-wrap items-baseline gap-x-2 text-sm">
                <dt className="text-mute">{t('sel.subEnds')}</dt>
                <dd className="font-medium tabular-nums">
                  {info?.verified_until ? t('sel.subEndsVal', { date: tanggal(info.verified_until), left: sisaHari(info.verified_until) }) : '—'}
                </dd>
              </dl>
              <p className="text-xs text-mute tabular-nums">{t('sel.subPrice', { price: rupiah(hargaVerified), days: sub?.renew_days || 30 })}</p>

              {/* Gagal tagih otomatis: beri tahu apa adanya, jangan diam. */}
              {sub?.last_error && (
                <p role="alert" className="text-xs text-danger bg-danger/10 border border-danger/30 rounded-lg px-3 py-2">
                  {t('sel.subLastError', { reason: sub.last_error })}
                </p>
              )}

              <div className="flex flex-wrap items-center gap-2">
                <label className="inline-flex items-center gap-2 text-sm cursor-pointer">
                  <input type="checkbox" className="accent-cyan-500" checked={!!sub?.auto_renew} disabled={subBusy}
                    onChange={(e) => setSubAksi(e.target.checked ? 'auto-on' : 'auto-off')} />
                  {t('sel.subAuto')}
                </label>
                <span className={`text-xs ${sub?.auto_renew ? 'text-cyan-300' : 'text-mute'}`}>
                  {sub?.auto_renew ? t('sel.subAutoOn') : t('sel.subAutoOff')}
                </span>
              </div>
              <p className="text-xs text-mute">{t('sel.subAutoHint', { price: rupiah(hargaVerified), balance: rupiah(saldoToko) })}</p>
              <Button size="sm" variant="outline" disabled={subBusy} onClick={() => setSubAksi('renew')}>
                {t('sel.subRenewBtn', { price: rupiah(hargaVerified) })}
              </Button>
            </div>
          )}

          <form onSubmit={pindahSaldo} className="mt-5 border-t border-border pt-5 space-y-3">
            <h3 className="label normal-case tracking-normal text-sm text-text">{t('sel.transferH')}</h3>
            <dl className="flex flex-wrap items-baseline gap-x-2">
              <dt className="text-sm text-mute">{t('sel.transferable')}</dt>
              <dd className="text-lg font-semibold tabular-nums text-cyan-300">{rupiah(saldoToko)}</dd>
            </dl>
            <div className="flex flex-wrap items-end gap-3">
              <div className="space-y-2 min-w-40 grow sm:grow-0">
                <label htmlFor="nominal-pindah" className="label">{t('sel.transferAmount')}</label>
                <Input id="nominal-pindah" type="text" inputMode="numeric" value={nominal ? angka(nominal) : ''}
                  aria-invalid={nominalSalah} aria-describedby="pindah-bantuan"
                  onChange={(e) => setNominal(e.target.value.replace(/\D/g, ''))} placeholder="0" />
              </div>
              <Button type="submit" disabled={!nominalSah || pindahBusy || toko.loading}>
                {pindahBusy ? t('sel.processing') : t('sel.transferBtn')}
              </Button>
            </div>
            <p id="pindah-bantuan" role={nominalSalah ? 'alert' : undefined}
              className={`text-xs ${nominalSalah ? 'text-danger' : 'text-mute'}`}>
              {nominalSalah ? t('sel.transferOver', { max: rupiah(saldoToko) }) : t('sel.transferHint', { max: rupiah(saldoToko) })}
            </p>
            <p className="text-xs text-mute leading-relaxed">{t('sel.escrowNote')}</p>
          </form>
        </section>
      ) : null}

      {ada && tab === 'ulasan' && (
        <section aria-label={t('sel.storeReviewsAria')} className="mb-8">
          <div className="flex flex-wrap items-center gap-3 mb-4">
            <h2 className="text-lg font-semibold tracking-tight">{t('sel.reviews')}</h2>
            {ulasan.loading || !ulasanRingkas?.count ? null : (
              <>
                <Badge tone={rata >= 4 ? 'ok' : rata >= 3 ? 'pending' : 'danger'}>{angka(ulasanRingkas?.average)} / 5</Badge>
                <span className="text-xs text-mute tabular-nums">{t('mkt.reviews', { n: angka(ulasanRingkas?.count) })}</span>
              </>
            )}
          </div>

          {ulasan.err ? (
            <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{ulasan.err}</p>
          ) : ulasan.loading ? (
            <div className="space-y-3">
              {[0, 1].map((i) => <div key={i} className="card p-4"><div className="skel h-4 w-40 mb-2" /><div className="skel h-3 w-64" /></div>)}
            </div>
          ) : daftarUlasan.length === 0 ? (
            <div className="card p-8 text-center">
              <p className="font-medium">{t('sel.noReviews')}</p>
              <p className="text-sm text-mute mt-1">{t('sel.noReviewsP')}</p>
            </div>
          ) : (
            <>
            <ul className="space-y-3">
              {halamanUlasan.items.map((r) => (
                <li key={r.id} className="card p-4">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="flex items-center gap-3 min-w-0">
                      <Bintang n={Number(r.rating) || 0} />
                      <span className="text-sm font-medium truncate">{r.buyer_name || t('sel.buyer')}</span>
                    </div>
                    <span className="text-xs text-mute tabular-nums shrink-0">{tanggal(r.created_at)}</span>
                  </div>
                  {r.comment && <p className="text-sm text-mute mt-2 leading-relaxed break-words">{r.comment}</p>}
                </li>
              ))}
            </ul>
            {daftarUlasan.length > 10 && <Pagination page={halamanUlasan.page} hasMore={halamanUlasan.hasMore} onPage={halamanUlasan.setPage} />}
            </>
          )}
        </section>
      )}

      {/* Form toko: tampil di tab "toko", dan SELALU kalau belum punya toko
          (tanpa toko, membuka toko adalah satu-satunya aksi yang berguna). */}
      {(!ada || tab === 'toko') && (
      <form onSubmit={simpanToko} className="card p-5 space-y-4 mb-10">
        <h2 className="label normal-case tracking-normal text-sm text-text">{ada ? t('sel.editStore') : t('sel.openStore')}</h2>
        <div className="space-y-2">
          <label htmlFor="nama-toko" className="label">{t('sel.storeName')}</label>
          <Input id="nama-toko" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })}
            placeholder={info?.name || t('sel.storeNamePh')} minLength={3} maxLength={80} required />
        </div>
        <div className="space-y-2">
          <label htmlFor="desk-toko" className="label">{t('sel.description')}</label>
          <Textarea id="desk-toko" value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })}
            maxLength={500} placeholder={t('sel.storeDescPh')} />
        </div>
        <ImageField label={t('sel.storeLogo')} value={form.logo}
          onChange={(url) => setForm({ ...form, logo: url })} />
        <Button type="submit" disabled={bukaBusy || form.name.trim().length < 3}>
          {bukaBusy ? t('sel.saving') : ada ? t('sel.saveChanges') : t('sel.openStoreBtn')}
        </Button>
      </form>
      )}

      {ada && tab === 'produk' && (
      <section aria-label={t('sel.myProductsAria')}>
        <div id="boost-packages" className="card mb-5 p-4 scroll-mt-24">
         <div className="flex flex-wrap items-center justify-between gap-3"><div><p className="label">{t('sel.boostKicker')}</p><p className="mt-1 text-sm text-mute">{t('sel.boostInfo')}</p></div><Badge tone={sisaBoost > 0 ? 'ok' : 'danger'}>{t('sel.boostRemaining', { count: sisaBoost })}</Badge></div>
         <div className="mt-4 flex flex-wrap gap-2"><Button size="sm" variant="outline" onClick={() => bukaPembayaran('boost_pack','5',5000,t('sel.boostPackage', { count: 5 }))}>x5 · Rp5.000</Button><Button size="sm" variant="outline" onClick={() => bukaPembayaran('boost_pack','15',15000,t('sel.boostPackage', { count: 15 }))}>x15 · Rp15.000</Button><Button size="sm" variant="outline" onClick={() => bukaPembayaran('boost_pack','50',30000,t('sel.boostPackage', { count: 50 }))}>x50 · Rp30.000</Button></div>
          <p className="mt-3 text-xs text-amber-200">{t('sel.boostPriceNote')}</p>
        </div>
        <div className="flex items-center justify-between gap-4 mb-4">
          <h2 className="text-lg font-semibold tracking-tight">{t('sel.myProducts')}</h2>
          <Button size="sm" disabled={!ada} onClick={() => { setItem(KOSONG); setDialog({ open: true }); }}>
            <Plus className="w-4 h-4" aria-hidden="true" /> {t('sel.add')}
          </Button>
        </div>

        {produk.err ? (
          <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{produk.err}</p>
        ) : produk.loading ? (
          <div className="space-y-3">
            {[0, 1].map((i) => <div key={i} className="card p-4 h-20"><div className="skel h-4 w-48 mb-2" /><div className="skel h-3 w-32" /></div>)}
          </div>
        ) : daftar.length === 0 ? (
          <div className="card p-10 text-center">
            <StoreIcon className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />
            <p className="font-medium">{t('sel.noProducts')}</p>
            <p className="text-sm text-mute mt-1">{ada ? t('sel.noProductsP') : t('sel.noStoreFirst')}</p>
          </div>
        ) : (
          <>
          <ul className="tbl-wrap zebra">
            {halamanProduk.items.map((p) => (
              <li key={p.id} className="p-4 flex flex-wrap items-center justify-between gap-4">
                <div className="min-w-0">
                  <div className="flex items-center gap-3 flex-wrap">
                    <p className="font-medium truncate">{p.title}</p>
                    <span className={`badge ${p.is_active === false ? 'badge-off' : 'badge-on'}`}>{p.is_active === false ? t('sel.inactive') : t('sel.active')}</span>
                    {p.is_instant && <Badge tone="ok">{t('sel.instant')}</Badge>}
                    {p.allow_affiliate && <Badge tone="pending">{t('sel.affBadge', { n: angka(p.affiliate_percent) })}</Badge>}
                    {p.stock <= STOK_RENDAH && <Badge tone="warn">{t('sel.lowStock', { n: angka(p.stock) })}</Badge>}
                    {/* Sisa masa boost: badge + tanggal berakhir supaya seller tahu
                        kapan harus memperpanjang. */}
                    {p.is_boosted && p.boosted_until && (
                      <Badge tone="ok" className="flex items-center gap-1">
                        <Rocket className="h-3 w-3" aria-hidden="true" />
                        {t('sel.boostedTill', { date: tanggal(p.boosted_until), left: sisaHari(p.boosted_until) })}
                      </Badge>
                    )}
                  </div>
                  <p className="text-xs text-mute mt-1 tabular-nums">
                    {rupiah(p.price)} &middot; {p.is_instant ? t('sel.instantStock') : t('sel.stock')} {p.stock}
                  </p>
                </div>
                <div className="act-group">
                  <Button size="sm" variant="ghost" disabled={busy === `boost:${p.id}`} onClick={() => bukaBoost(p)}>
                    <Rocket className="w-4 h-4" aria-hidden="true" /> {busy === `boost:${p.id}` ? '…' : sisaBoost > 0 ? t('sel.boost') : t('sel.buyBoost')}
                  </Button>
                  <Button size="sm" variant="outline"
                    onClick={() => {
                      // Produk instan: isian TIDAK bisa dibaca kembali dari server
                      // (sengaja — isinya kredensial) dan PATCH juga tidak
                      // mengubahnya, jadi textarea isian di dialog sunting memang
                      // kosong dan tidak menggantikan apa pun. Lihat catatan di
                      // bawah textarea.
                      setItem({ ...KOSONG, title: p.title, description: p.description || '', category_id: p.category_id || '', price: String(p.price), stock: String(p.stock), is_instant: !!p.is_instant, allow_affiliate: !!p.allow_affiliate, affiliate_percent: p.affiliate_percent ? String(p.affiliate_percent) : '' });
                      setNamaBerkas('');
                      setDialog({ open: true, id: p.id });
                    }}>
                    {t('sel.edit')}
                  </Button>
                  {hapusId === p.id ? (
                    <>
                      <span className="text-xs text-danger hidden sm:block">{t('sel.deleteConfirm')}</span>
                      <Button size="sm" variant="destructive" disabled={busy === p.id} onClick={() => hapus(p.id)}>
                        {busy === p.id ? t('sel.deleting') : t('sel.yesDelete')}
                      </Button>
                      <Button size="sm" variant="ghost" onClick={() => setHapusId(null)}>{t('sel.cancel')}</Button>
                    </>
                  ) : (
                    <button onClick={() => setHapusId(p.id)} aria-label={t('sel.deleteAria', { title: p.title })}
                      className="p-2 text-mute hover:text-danger hover:bg-danger/10 rounded-lg transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-danger/60">
                      <Trash2 className="w-4 h-4" aria-hidden="true" />
                    </button>
                  )}
                </div>
              </li>
            ))}
          </ul>
          {daftar.length > 10 && <Pagination page={halamanProduk.page} hasMore={halamanProduk.hasMore} onPage={halamanProduk.setPage} />}
          </>
        )}
      </section>
      )}

      {ada && tab === 'pesanan' && (
        <section aria-label={t('sel.incomingOrdersAria')} className="mt-10">
          <div className="flex items-center justify-between gap-4 mb-4">
            <h2 className="text-lg font-semibold tracking-tight">{t('sel.incomingOrders')}</h2>
            <Link href="/toko/disputes" className="text-sm text-cyan-400 hover:underline">{t('dispute.kicker')}</Link>
          </div>
          {pesananMasuk.err ? (
            <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{pesananMasuk.err}</p>
          ) : pesananMasuk.loading ? (
            <div className="space-y-3">
              {[0, 1].map((i) => <div key={i} className="card p-4 h-20"><div className="skel h-4 w-56 mb-2" /><div className="skel h-3 w-40" /></div>)}
            </div>
          ) : daftarPesanan.length === 0 ? (
            <div className="card p-8 text-center">
              <p className="font-medium">{t('sel.noIncoming')}</p>
              <p className="text-sm text-mute mt-1">{t('sel.noIncomingP')}</p>
            </div>
          ) : (
            <>
              <ul className="tbl-wrap zebra">
                {daftarPesanan.map((o) => (
                  <li key={o.id} className="p-4 flex flex-wrap items-center justify-between gap-4">
                    <div className="min-w-0">
                      <p className="font-medium truncate">{o.product_title}</p>
                      <p className="text-xs text-mute mt-1 tabular-nums">
                        {t('sel.unit', { n: o.quantity, date: tanggal(o.created_at, true) })}
                      </p>
                    </div>
                    <div className="act-group">
                      <div className="text-right">
                        <p className="text-sm tabular-nums text-cyan-300">{rupiah(o.seller_payout)}</p>
                        <p className="text-xs text-mute tabular-nums">{t('sel.fromAmount', { amount: rupiah(o.subtotal) })}</p>
                      </div>
                      {!o.is_instant && (o.status === 'PAID' || o.status === 'PROCESSING') && (
                        <Button size="sm" variant="outline" disabled={busy === `delivered:${o.id}`} onClick={() => tandaiTerkirim(o.id)}>
                          {busy === `delivered:${o.id}` ? t('sel.processing') : t('sel.markDelivered')}
                        </Button>
                      )}
                      {!o.is_instant && (o.status === 'PENDING' || o.status === 'PAID' || o.status === 'PROCESSING') && (
                        <Link href={`/pesanan/${o.id}/chat`} passHref>
                          <Button size="sm" variant="outline" aria-label={t('sel.chatAria', { title: o.product_title })}>
                            <MessageCircle className="w-4 h-4" aria-hidden="true" /> {t('sel.chat')}
                          </Button>
                        </Link>
                      )}
                      <Badge>{statusLabel(o.status, t)}</Badge>
                    </div>
                  </li>
                ))}
              </ul>
              <Pagination page={poPage} hasMore={pesananMasuk.data?.has_more} onPage={setPoPage} />
            </>
          )}
        </section>
      )}

      <Dialog open={dialog.open} title={dialog.id ? t('sel.editProduct') : t('sel.addProduct')} onClose={() => setDialog({ open: false })}>
        <div className="space-y-5">
          <div className="space-y-2">
            <label htmlFor="p-judul" className="label">{t('sel.productTitle')}</label>
            <Input id="p-judul" value={item.title} onChange={(e) => setItem((prev) => ({ ...prev, title: e.target.value }))} required />
          </div>
          <div className="space-y-2">
            <label htmlFor="p-kategori" className="label">{t('sel.category')}</label>
            <select id="p-kategori" value={item.category_id}
              onChange={(e) => setItem((prev) => ({ ...prev, category_id: e.target.value }))}
              required className="w-full rounded-lg border border-border bg-surface-hi px-3 py-2.5 text-sm text-text focus:outline-none focus:ring-2 focus:ring-cyan-400/60">
              <option value="">{t('sel.chooseCategory')}</option>
              {daftarKategori.map((k) => <option key={k.id} value={k.id}>{labelKategori(k)}</option>)}
            </select>
            {kategori.err && <p role="alert" className="text-xs text-danger">{t('sel.categoryFail')}</p>}
          </div>
          <div className="space-y-2">
            <div className="flex items-center justify-between gap-3">
              <label htmlFor="p-desk" className="label">{t('sel.description')}</label>
              <span className={`text-xs tabular-nums ${deskripsiPanjang < MIN_DESKRIPSI ? 'text-danger' : 'text-mute'}`}>
                {deskripsiPanjang}/{MAX_DESKRIPSI}
              </span>
            </div>
            <Textarea id="p-desk" value={item.description} minLength={MIN_DESKRIPSI} maxLength={MAX_DESKRIPSI} required
              onChange={(e) => setItem((prev) => ({ ...prev, description: e.target.value }))} />
            <p className="text-xs text-mute">{t('sel.descHint', { min: MIN_DESKRIPSI, max: MAX_DESKRIPSI })}</p>
          </div>
          <fieldset className="space-y-2">
            <legend className="label">{t('sel.deliveryType')}</legend>
            <div className="flex flex-wrap items-center gap-5 pt-1">
              <label htmlFor="p-jenis-instan" className="flex items-center gap-2 text-sm cursor-pointer">
                <input id="p-jenis-instan" type="radio" name="p-jenis" className="accent-cyan-400 w-4 h-4"
                  checked={item.is_instant} onChange={() => setItem((prev) => ({ ...prev, is_instant: true }))} />
                {t('sel.instant')}
              </label>
              <label htmlFor="p-jenis-biasa" className="flex items-center gap-2 text-sm cursor-pointer">
                <input id="p-jenis-biasa" type="radio" name="p-jenis" className="accent-cyan-400 w-4 h-4"
                  checked={!item.is_instant} onChange={() => setItem((prev) => ({ ...prev, is_instant: false }))} />
                {t('sel.no')}
              </label>
            </div>
            <p className="text-xs text-mute">
              {item.is_instant
                ? t('sel.instantHint')
                : t('sel.manualHint')}
            </p>
          </fieldset>
          <fieldset className="card p-4 space-y-3">
            <label htmlFor="p-afiliasi" className="flex items-start gap-3 cursor-pointer">
              <input id="p-afiliasi" type="checkbox" className="mt-0.5 h-4 w-4 accent-cyan-400"
                checked={item.allow_affiliate}
                onChange={(e) => setItem((prev) => ({ ...prev, allow_affiliate: e.target.checked, affiliate_percent: e.target.checked ? prev.affiliate_percent : '' }))} />
              <span>
                <span className="block text-sm font-medium">{t('sel.enableAffiliate')}</span>
                <span className="block text-xs text-mute mt-1">{t('sel.enableAffiliateP')}</span>
              </span>
            </label>
            {item.allow_affiliate && (
              <div className="space-y-2 pl-7">
                <label htmlFor="p-komisi" className="label">{t('sel.affiliatePercent')}</label>
                <Input id="p-komisi" type="number" inputMode="decimal" min={1} max={100} step="0.01"
                  value={item.affiliate_percent}
                  onChange={(e) => setItem((prev) => ({ ...prev, affiliate_percent: e.target.value }))} required />
                <p className="text-xs text-mute">{t('sel.affiliatePercentP')}</p>
              </div>
            )}
          </fieldset>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label htmlFor="p-harga" className="label">{t('sel.priceRp')}</label>
              <Input id="p-harga" type="text" inputMode="numeric" value={item.price ? angka(item.price) : ''}
                onChange={(e) => setItem((prev) => ({ ...prev, price: e.target.value.replace(/\D/g, '') }))} required />
            </div>
            {item.is_instant ? (
              <div className="space-y-2">
                <span className="label">{t('sel.stock')}</span>
                <p className="text-sm tabular-nums pt-2">
                  {angka(jumlahInstan)} <span className="text-mute">{t('sel.fromItems')}</span>
                </p>
              </div>
            ) : (
              <div className="space-y-2">
                <label htmlFor="p-stok" className="label">{t('sel.stock')}</label>
                <Input id="p-stok" type="text" inputMode="numeric" value={item.stock ? angka(item.stock) : ''}
                  onChange={(e) => setItem((prev) => ({ ...prev, stock: e.target.value.replace(/\D/g, '') }))} required />
              </div>
            )}
          </div>
          {item.is_instant && (
            <div className="space-y-2">
              <label htmlFor="p-isian" className="label">{t('sel.itemsLabel')}</label>
              {dialog.id && (
                <p className="banner banner-warn mt-0">
                  {t('sel.instantEditNote')}
                </p>
              )}
              <input id="p-berkas" ref={berkasRef} type="file" accept=".txt,text/plain" className="block w-full text-xs text-mute"
                onChange={(e) => bacaBerkas(e.target.files?.[0])} />
              <Textarea id="p-isian" value={item.instant_items} rows={6}
                onChange={(e) => setItem((prev) => ({ ...prev, instant_items: e.target.value }))}
                placeholder={'kode-aktivasi-1\nkode-aktivasi-2\nakun@mail.com|password'} />
              <p className="text-xs text-mute tabular-nums">
                {t('sel.itemsReady', { n: angka(jumlahInstan) })}{namaBerkas ? t('sel.readFrom', { name: namaBerkas }) : ''}
                {jumlahInstan > MAX_INSTAN ? t('sel.maxItems', { n: angka(MAX_INSTAN) }) : ''}
              </p>
            </div>
          )}
          <ImageField label={t('sel.productImage')} value={item.image_url}
            onChange={(url) => setItem((prev) => ({ ...prev, image_url: url }))} />
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setDialog({ open: false })}>{t('sel.cancel')}</Button>
            <Button onClick={simpanProduk} disabled={!sah || simpanBusy}>{simpanBusy ? t('sel.saving') : t('sel.save')}</Button>
          </div>
        </div>
      </Dialog>
      <Dialog open={confirmVerify} title={t('sel.buyVerified')} onClose={() => setConfirmVerify(false)}>
        <div className="space-y-4">
          {/* Harga dari server (verified_monthly_price), bukan hardcode —
              kalau admin mengubah tarif, popup ikut berubah. */}
          <p className="text-sm text-mute">
            {t('sel.verifyQ')} <strong className="text-text">{rupiah(hargaVerified)}</strong>{t('sel.verifyP')}
          </p>
          <p className="text-sm text-mute tabular-nums">
            {t('sel.verifyWallet', { balance: rupiah(info?.wallet_balance) })}
          </p>
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setConfirmVerify(false)} disabled={verifyBusy}>{t('sel.cancel')}</Button>
            <Button onClick={beliVerified} disabled={verifyBusy}>{verifyBusy ? t('sel.processing') : t('sel.confirmBuy')}</Button>
          </div>
        </div>
      </Dialog>

      {/* Konfirmasi boost kredit: count × 6 jam, dibatasi saldo yang tersedia. */}
      <Dialog open={boostModal} title={t('sel.boostConfirmTitle')} onClose={() => setBoostModal(false)}>
        <div className="space-y-4">
          <p className="text-sm text-mute">{t('sel.boostConfirmIntro')}</p>
          <p className="text-sm font-medium break-words">{boostTarget?.title}</p>

          <div className="space-y-2">
            <label htmlFor="boost-count" className="label">{t('sel.boostCount')}</label>
            <Input id="boost-count" type="number" inputMode="numeric" min={1} max={Math.min(50, sisaBoost)} value={boostCount}
              onChange={(e) => setBoostCount(Math.max(1, Math.min(50, sisaBoost, Number(e.target.value) || 1)))} />
            <p className="text-xs text-mute tabular-nums">{t('sel.boostSummary', { count: boostCount, hours: boostCount * 6 })}</p>
          </div>

          <dl className="rounded-lg border border-border bg-surface-hi p-4 space-y-2 text-sm">
            <div className="flex justify-between gap-4">
              <dt className="text-mute">{t('sel.boostUsed')}</dt>
              <dd className="font-semibold tabular-nums text-cyan-300">{boostCount}</dd>
            </div>
            <div className="flex justify-between gap-4">
              <dt className="text-mute">{t('sel.boostLeftNow')}</dt>
              <dd className="tabular-nums">{sisaBoost}</dd>
            </div>
            <div className="flex justify-between gap-4 border-t border-border pt-2">
              <dt className="text-mute">{t('sel.boostAfter')}</dt>
              <dd className={`tabular-nums font-medium ${boostCount > sisaBoost ? 'text-danger' : ''}`}>{Math.max(0, sisaBoost - boostCount)}</dd>
            </div>
          </dl>

          <div className="flex flex-wrap justify-end gap-2">
            <Button variant="ghost" onClick={() => setBoostModal(false)} disabled={!!busy}>{t('sel.cancel')}</Button>
            <Button onClick={jalankanBoost} disabled={!!busy || boostCount < 1 || boostCount > sisaBoost}>
              {busy?.startsWith('boost:') ? t('sel.processing') : t('sel.useBoost', { count: boostCount })}
            </Button>
          </div>
        </div>
      </Dialog>

      {/* Konfirmasi langganan: menyalakan auto-renew berarti setuju dipotong
          otomatis tiap periode — wajib dikonfirmasi, bukan efek samping klik. */}
      <Dialog open={subAksi !== null} title={t(subAksi === 'renew' ? 'sel.subRenewTitle' : subAksi === 'auto-on' ? 'sel.subAutoOnTitle' : 'sel.subAutoOffTitle')}
        onClose={() => setSubAksi(null)}>
        <div className="space-y-4">
          {subAksi === 'auto-on' && (
            <>
              <p className="text-sm text-mute">{t('sel.subAutoOnBody', { price: rupiah(hargaVerified), days: sub?.renew_days || 30 })}</p>
              <dl className="rounded-lg border border-border bg-surface-hi p-4 space-y-2 text-sm">
                <div className="flex justify-between gap-4"><dt className="text-mute">{t('sel.subPerPeriod')}</dt><dd className="font-semibold tabular-nums text-cyan-300">{rupiah(hargaVerified)}</dd></div>
                <div className="flex justify-between gap-4"><dt className="text-mute">{t('sel.subBalanceNow')}</dt><dd className="tabular-nums">{rupiah(saldoToko)}</dd></div>
              </dl>
              <p className="text-xs text-amber-200 bg-amber-500/10 border border-amber-400/30 rounded-lg px-3 py-2">{t('sel.subAutoOnWarn', { balance: rupiah(saldoToko) })}</p>
            </>
          )}
          {subAksi === 'auto-off' && <p className="text-sm text-mute">{t('sel.subAutoOffBody', { date: info?.verified_until ? tanggal(info.verified_until) : '—' })}</p>}
          {subAksi === 'renew' && (
            <>
              <p className="text-sm text-mute">{t('sel.subRenewBody', { price: rupiah(hargaVerified), days: sub?.renew_days || 30 })}</p>
              <dl className="rounded-lg border border-border bg-surface-hi p-4 space-y-2 text-sm">
                <div className="flex justify-between gap-4"><dt className="text-mute">{t('sel.boostCost')}</dt><dd className="font-semibold tabular-nums text-cyan-300">{rupiah(hargaVerified)}</dd></div>
                <div className="flex justify-between gap-4"><dt className="text-mute">{t('sel.subBalanceNow')}</dt><dd className="tabular-nums">{rupiah(saldoToko)}</dd></div>
                <div className="flex justify-between gap-4 border-t border-border pt-2">
                  <dt className="text-mute">{t('sel.boostAfter')}</dt>
                  <dd className={`tabular-nums font-medium ${hargaVerified > saldoToko ? 'text-danger' : ''}`}>
                    {hargaVerified > saldoToko ? t('sel.boostInsufficient') : rupiah(saldoToko - hargaVerified)}
                  </dd>
                </div>
              </dl>
            </>
          )}
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setSubAksi(null)} disabled={subBusy}>{t('sel.cancel')}</Button>
            {subAksi === 'auto-on' && <Button onClick={() => ubahAutoRenew(true)} disabled={subBusy}>{subBusy ? t('sel.processing') : t('sel.subAutoOnBtn')}</Button>}
            {subAksi === 'auto-off' && <Button onClick={() => ubahAutoRenew(false)} disabled={subBusy}>{subBusy ? t('sel.processing') : t('sel.subAutoOffBtn')}</Button>}
            {subAksi === 'renew' && <Button onClick={perpanjangSekarang} disabled={subBusy || hargaVerified > saldoToko}>{subBusy ? t('sel.processing') : t('sel.subRenewConfirm')}</Button>}
          </div>
        </div>
      </Dialog>
    </div>
  );
}
