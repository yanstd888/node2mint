'use client';
// NotificationBell: lonceng di header AppShell, samping CartMenu (desktop & mobile).
// Hanya tampil saat terautentikasi. Polling 30s sekali hanya saat panel terbuka.
//
// Lifecycle notifikasi:
//  - Pagination: beban pertama page=1 (default 20); tombol "Load more" fetch page
//    berikutnya pakai ?page=&per_page=. has_more dari server mengatur tombol.
//  - Automatic bell read: saat panel dibuka, semua notifikasi yang belum dibaca
//    ditandai dibaca lewat POST /notifications/read-all (satu UPDATE di server).
//  - Chat read only on opening: notifikasi ditandai dibaca singleton HANYA saat
//    pengguna mengeklik notifikasi bertipe order untuk membuka chat.
import { useCallback, useEffect, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Bell, Check, ShoppingCart, Tag as TagIcon, MessageCircle, BellRing, HandCoins } from 'lucide-react';
import { api } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import { useT } from '@/lib/i18n';

export type Notification = {
  id: string;
  type: 'PURCHASE' | 'SALE' | 'ORDER_CHAT' | 'SYSTEM' | 'UPDATE' | 'AFFILIATE_COMMISSION';
  title: string;
  message: string;
  is_read: boolean;
  created_at: string;
  order_id?: string;
};

const TYPE_ICON: Record<Notification['type'], React.ElementType> = {
  PURCHASE: ShoppingCart,
  SALE: TagIcon,
  ORDER_CHAT: MessageCircle,
  SYSTEM: BellRing,
  UPDATE: BellRing,
  AFFILIATE_COMMISSION: HandCoins,
};

const PER_PAGE = 20;

type NotifPage = {
  notifications: Notification[];
  has_more: boolean;
};

export default function NotificationBell() {
  const { user } = useAuth();
  const { t, lang } = useT();
  const router = useRouter();
  const ref = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  // Semua notifikasi yang sudah dimuat (paginated).
  const [all, setAll] = useState<Notification[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [page, setPage] = useState(0);
  const errRef = useRef('');
  // Badge dari /notifications/unread (COUNT murni) — panel hanya memuat daftar
  // saat dibuka, jadi tanpa ini notifikasi baru (mis. komisi afiliasi) tidak
  // pernah memunculkan badge sampai pengguna membuka panel.
  const [unread, setUnread] = useState(0);

  const loadPage = async (nextPage: number, replace: boolean) => {
    if (replace) {
      setLoading(true);
    } else {
      setLoadingMore(true);
    }
    try {
      const res = await api.get<NotifPage>(`/notifications?page=${nextPage}&per_page=${PER_PAGE}`);
      const page = res.data;
      setAll(replace ? page.notifications : [...all, ...page.notifications]);
      setHasMore(page.has_more);
      setPage(nextPage);
      errRef.current = '';
    } catch {
      errRef.current = t('notif.fail');
    } finally {
      setLoading(false);
      setLoadingMore(false);
    }
  };

  // Badge count dari endpoint ringan; tidak menyentuh daftar.
  const loadUnread = useCallback(async () => {
    try {
      const { data } = await api.get<{ unread: number }>('/notifications/unread');
      setUnread(data.unread || 0);
    } catch {
      // swallow — badge cukup menampilkan angka terakhir yang diketahui
    }
  }, []);

  // Badge: muat sekali saat login, lalu polling 60s (termasuk saat panel tertutup).
  useEffect(() => {
    if (!user) return;
    void loadUnread();
    const timer = setInterval(() => void loadUnread(), 60000);
    return () => clearInterval(timer);
  }, [user, loadUnread]);

  // Buka panel: reset ke page 1 + automatic bell read.
  useEffect(() => {
    if (!open || !user) return;
    void loadPage(1, true);
    // Automatic bell read: satu panggilan batch ke server.
    void api.post<{ ok: boolean }>('/notifications/read-all', {}).then(() => setUnread(0));
  }, [open, user]); // eslint-disable-line react-hooks/exhaustive-deps

  // Polling 30s hanya saat panel terbuka.
  useEffect(() => {
    if (!open) return;
    const t = setInterval(() => {
      if (page >= 1) void loadPage(page, true);
    }, 30000);
    return () => clearInterval(t);
  }, [open, page]);

  // Close on outside click (mobile + desktop).
  useEffect(() => {
    if (!open) return;
    const close = (e: PointerEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('pointerdown', close);
    return () => document.removeEventListener('pointerdown', close);
  }, [open]);

  // Mark single as read (optimistic).
  const markRead = async (id: string) => {
    try {
      await api.post<{ ok: boolean }>(`/notifications/${id}/read`, {});
      setAll((prev) => prev.map((n) => (n.id === id ? { ...n, is_read: true } : n)));
      setUnread((n) => Math.max(0, n - 1));
    } catch {
      // swallow — UI tetap responsif
    }
  };

  // Mark all as read: satu batch call ke server, lalu optimis ke lokal.
  const markAllRead = async () => {
    if (!all.length) return;
    try {
      await api.post<{ ok: boolean }>('/notifications/read-all', {});
      setAll((prev) => prev.map((n) => ({ ...n, is_read: true })));
      setUnread(0);
    } catch {
      // swallow — UI tetap responsif
    }
  };

  const loadMore = () => {
    void loadPage(page + 1, false);
  };

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => { setOpen((v) => !v); if (!open) void loadPage(1, true); }}
        aria-label={t('notif.title')}
        aria-expanded={open}
        aria-haspopup="dialog"
        className={`relative grid h-10 w-10 place-items-center rounded-lg transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60 ${
          open ? 'bg-white/10 text-text' : 'text-mute hover:bg-white/5 hover:text-text'
        }`}
      >
        <Bell className="h-5 w-5" aria-hidden="true" />
        {unread > 0 && (
          <span className="absolute right-0.5 top-0.5 min-w-4 rounded-full bg-cyan-300 px-1 text-center text-[10px] font-semibold leading-4 text-[#071015]">
            {unread > 99 ? '99+' : unread}
          </span>
        )}
      </button>

      {open && (
        <section
          role="dialog"
          aria-label={t('notif.title')}
          className="fixed left-4 right-4 top-20 z-50 max-h-[calc(100dvh-6rem)] overflow-y-auto rounded-xl border border-border bg-bg p-2 shadow-2xl sm:absolute sm:left-auto sm:right-0 sm:top-12 sm:w-80"
        >
          <div className="mb-2 flex items-center justify-between px-3 py-2">
            <h2 className="font-semibold text-sm">{t('notif.title')}</h2>
            {all.length > 0 && unread > 0 && (
              <button
                onClick={markAllRead}
                className="text-xs text-cyan-300 hover:text-cyan-200"
              >
                {t('notif.markAll')}
              </button>
            )}
          </div>

          {errRef.current ? (
            <p role="alert" className="px-3 py-2 text-sm text-danger">{errRef.current}</p>
          ) : loading ? (
            <div className="space-y-2 p-2" aria-label={t('notif.loading')}>
              {[0, 1, 2].map((i) => <div key={i} className="skel h-14 rounded-lg" />)}
            </div>
          ) : all.length === 0 ? (
            <div className="py-8 text-center">
              <Bell className="mx-auto mb-3 h-8 w-8 text-mute" aria-hidden="true" />
              <p className="text-sm text-mute">{t('notif.empty')}</p>
            </div>
          ) : (
            <ul className="space-y-1">
              {all.map((n) => {
                const Icon = TYPE_ICON[n.type] ?? Bell;
                const isOrder = !!n.order_id;
                return (
                  <li
                    key={n.id}
                    onClick={(e) => {
                      if (!isOrder) return;
                      e.stopPropagation();
                      void markRead(n.id);
                      setOpen(false);
                      router.push(`/pesanan/${n.order_id}/chat`);
                    }}
                    role={isOrder ? 'button' : undefined}
                    aria-label={isOrder ? t('notif.openChat', { title: n.title }) : undefined}
                    className={`rounded-lg border p-3 text-sm transition-colors cursor-${isOrder ? 'pointer' : 'default'} ${
                      n.is_read
                        ? 'border-border bg-transparent'
                        : 'border-cyan-400/30 bg-cyan-500/5'
                    }`}
                  >
                    <div className="flex items-start gap-3">
                      <Icon className={`h-4 w-4 shrink-0 mt-0.5 ${
                        n.is_read ? 'text-mute' : 'text-cyan-300'
                      }`} aria-hidden="true" />
                      <div className="min-w-0 flex-1">
                        <p className={`font-medium ${n.is_read ? 'text-mute' : 'text-text'}`}>
                          {n.title}
                        </p>
                        <p className={`mt-0.5 text-xs ${n.is_read ? 'text-mute' : 'text-mute'}`}>
                          {n.message}
                        </p>
                        <span className="text-[10px] text-mute/50">
                          {new Date(n.created_at).toLocaleDateString(lang === 'id' ? 'id-ID' : 'en-US', {
                            day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit',
                          })}
                        </span>
                      </div>
                      {!n.is_read && (
                        <button
                          onClick={(e) => { e.stopPropagation(); void markRead(n.id); }}
                          aria-label={t('notif.markOne')}
                          className="p-1 text-mute hover:text-text rounded focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-cyan-400/60"
                        >
                          <Check className="h-3 w-3" />
                        </button>
                      )}
                    </div>
                  </li>
                );
              })}
            </ul>
          )}

          {hasMore && !errRef.current && (
            <button
              onClick={loadMore}
              disabled={loadingMore}
              className="flex w-full items-center justify-center gap-1 rounded-lg border border-border py-2 text-xs text-mute hover:bg-surface-hi hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60"
            >
              {loadingMore ? t('notif.loading') : t('notif.more')}
            </button>
          )}
        </section>
      )}
    </div>
  );
}
