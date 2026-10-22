'use client';

import { useState } from 'react';
import { Heart, LoaderCircle } from 'lucide-react';
import { api } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import { useFetch } from '@/lib/useApi';
import { useT } from '@/lib/i18n';
import { useToast } from '@/components/ui/toast';

type WishItem = { id: string };

type Props = {
  productId: string;
  className?: string;
};

export default function WishlistButton({ productId, className = '' }: Props) {
  const { user } = useAuth();
  const { t } = useT();
  const toast = useToast();
  const [busy, setBusy] = useState(false);
  const { data, loading, setData } = useFetch<{ items?: WishItem[] }>(
    user && productId ? '/market/wishlist' : '', !user || !productId
  );
  const items = data?.items ?? [];
  const saved = items.some((item) => item.id === productId);

  const toggle = async () => {
    if (!user) {
      location.assign(`/masuk?next=${encodeURIComponent(location.pathname)}`);
      return;
    }
    if (busy) return;
    setBusy(true);
    try {
      if (saved) {
        await api.del(`/market/wishlist/${encodeURIComponent(productId)}`);
        setData({ items: items.filter((item) => item.id !== productId) });
        toast.ok(t('mkt.wishlistRemoved'));
      } else {
        await api.post(`/market/wishlist/${encodeURIComponent(productId)}`, {});
        setData({ items: [...items, { id: productId }] });
        toast.ok(t('mkt.wishlistSaved'));
      }
    } catch {
      toast.err(t('mkt.wishlistFail'));
    } finally {
      setBusy(false);
    }
  };

  const disabled = busy || loading;
  const label = saved ? t('mkt.wishlistRemove') : t('mkt.wishlistAdd');

  return (
    <button type="button" onClick={toggle} disabled={disabled} aria-pressed={saved} aria-label={label}
      title={label}
      className={`inline-flex items-center justify-center rounded-lg border border-border bg-surface-hi p-2 text-mute hover:text-cyan-300 disabled:cursor-wait disabled:opacity-60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60 ${className}`}>
      {disabled ? <LoaderCircle className="h-5 w-5 animate-spin" aria-hidden="true" /> : <Heart className={`h-5 w-5 ${saved ? 'fill-current text-cyan-300' : ''}`} aria-hidden="true" />}
    </button>
  );
}
