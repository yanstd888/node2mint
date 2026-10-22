'use client';

import Link from 'next/link';
import { useState } from 'react';
import { Heart, Package } from 'lucide-react';
import { api } from '@/lib/api';
import { useFetch } from '@/lib/useApi';
import { useT } from '@/lib/i18n';
import { useToast } from '@/components/ui/toast';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { productUrl, rupiah } from '@/lib/utils';

type WishlistItem = {
  id: string;
  title: string;
  price: number;
  stock: number;
  image_url?: string;
  store_name?: string;
  store_id?: string;
  is_instant?: boolean;
  sales_count?: number;
  review_count?: number;
};

export default function WishlistPage() {
  const { t } = useT();
  const toast = useToast();
  const [busy, setBusy] = useState<string | null>(null);
  const { data, loading, err, reload } = useFetch<{ items?: WishlistItem[] }>('/market/wishlist');
  const items = data?.items ?? [];

  const remove = async (id: string) => {
    setBusy(id);
    try {
      await api.del(`/market/wishlist/${encodeURIComponent(id)}`);
      toast.ok(t('mkt.wishlistRemoved'));
      await reload();
    } catch {
      toast.err(t('mkt.wishlistFail'));
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="p-6 md:p-10 max-w-6xl mx-auto">
      <header className="mb-8">
        <p className="label mb-1">{t('sb.wishlist')}</p>
        <h1 className="text-2xl font-semibold tracking-tight">{t('wish.h')}</h1>
        <p className="text-sm text-mute mt-2">{t('wish.sub')}</p>
      </header>

      {err ? (
        <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{err}</p>
      ) : loading ? (
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4" aria-busy="true">
          {[0, 1, 2, 3].map((i) => <div key={i} className="card p-3"><div className="skel aspect-square mb-3" /><div className="skel h-4 w-3/4 mb-2" /><div className="skel h-4 w-1/2" /></div>)}
        </div>
      ) : items.length === 0 ? (
        <div className="card p-12 text-center">
          <Heart className="w-10 h-10 mx-auto mb-4 text-mute" aria-hidden="true" />
          <p className="font-medium">{t('wish.empty')}</p>
          <p className="text-sm text-mute mt-1 mb-6">{t('wish.emptyP')}</p>
          <Link href="/market"><Button>{t('cart.browse')}</Button></Link>
        </div>
      ) : (
        <ul className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
          {items.map((item) => (
            <li key={item.id} className="card p-3 flex flex-col min-w-0">
              <Link href={productUrl(item.store_name, item.title, item.id)} className="block">
                {item.image_url ? (
                  <img src={item.image_url} alt={item.title} className="w-full aspect-square object-cover rounded-lg mb-3 border border-border" />
                ) : (
                  <div className="w-full aspect-square rounded-lg mb-3 border border-border bg-gradient-to-br from-cyan-500/10 to-surface-hi flex items-center justify-center"><Package className="w-7 h-7 text-mute/70" /></div>
                )}
                <h2 className="font-medium text-sm line-clamp-2 min-h-10">{item.title}</h2>
                {item.store_name && <p className="text-xs text-mute mt-1 truncate">@{item.store_name}</p>}
                <p className="text-cyan-300 font-semibold text-sm tabular-nums mt-2">{rupiah(item.price)}</p>
              </Link>
              <div className="flex flex-wrap items-center gap-2 mt-3">
                {item.is_instant && <Badge tone="ok">{t('show.instant')}</Badge>}
                <span className="text-xs text-mute">{item.stock > 0 ? t('mkt.inStock', { n: item.stock }) : t('mkt.out')}</span>
              </div>
              <Button size="sm" variant="ghost" className="mt-3 w-full" disabled={busy === item.id} onClick={() => remove(item.id)}>
                <Heart className="w-4 h-4 fill-current text-cyan-300" aria-hidden="true" /> {t('mkt.wishlistRemove')}
              </Button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
