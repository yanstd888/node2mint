'use client';
// Chat pesanan: buyer & seller order ini saling kirim pesan.
// Akses di-enforce di backend (hanya buyer + seller order).
// Termasuk pembukaan sengketa buyer (POST /market/orders/{id}/dispute).
import { useParams, useRouter } from 'next/navigation';
import { useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { post, pesanError } from '@/lib/http';
import { useFetch } from '@/lib/useApi';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Dialog } from '@/components/ui/dialog';
import { useToast } from '@/components/ui/toast';
import { rupiah, tanggal } from '@/lib/utils';
import { useT } from '@/lib/i18n';
import { ArrowLeft, Send, Package, AlertCircle } from 'lucide-react';

type Msg = {
  id: string;
  sender_id: string;
  message: string;
  created_at: string;
  is_me: boolean;
};

type OrderInfo = {
  id: string;
  status: string;
  subtotal?: number;
  quantity?: number;
  is_instant?: boolean;
  created_at?: string;
  created_at_iso?: string;
  product?: { title?: string };
  store?: { name?: string };
};

export default function OrderChatPage() {
  const { t } = useT();
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const orderId = params?.id ?? '';
  const toast = useToast();

  // Endpoint chat hanya mengembalikan messages; detail order datang dari daftar
  // buyer yang memang sudah memuat title/store/status untuk header.
  const ordersList = useFetch<{ orders?: OrderInfo[] }>(`/market/orders?page=1`, !orderId);
  const order = ordersList.data?.orders?.find((o) => o.id === orderId);

  const [messages, setMessages] = useState<Msg[]>([]);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const endRef = useRef<HTMLLIElement>(null);
	const [disputeOpen, setDisputeOpen] = useState(false);
	const [alasan, setAlasan] = useState('');
	const [bukti, setBukti] = useState('');
	const [disputeBusy, setDisputeBusy] = useState(false);
	const disputeFetch = useFetch<{ dispute?: { status: string; resolution?: string; seller_response?: string } }>(
		orderId ? `/market/orders/${orderId}/dispute` : '', !orderId || order?.status !== 'DISPUTED'
	);

  const { data: chatData, loading, err, reload } = useFetch<{ messages?: Msg[] }>(
    orderId ? `/market/orders/${orderId}/chat` : '', !orderId
  );

  const dispute = disputeFetch.data?.dispute;

  useEffect(() => {
    if (chatData?.messages) {
      setMessages(chatData.messages);
    }
  }, [chatData]);

  useEffect(() => {
    endRef.current?.scrollIntoView({ block: 'end' });
  }, [messages]);

  if (!orderId) {
    return <p role="alert" className="p-6">{t('chat.noId')}</p>;
  }

  if (err) {
    return (
      <div className="p-6">
        <p role="alert" className="text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">
          {err}
        </p>
        <Button variant="outline" onClick={() => router.back()} className="mt-4">
          <ArrowLeft className="w-4 h-4" /> {t('chat.back')}
        </Button>
      </div>
    );
  }

  const kirim = async () => {
    const isi = input.trim();
    if (!isi) return;
    setSending(true);
    try {
      await post(`/market/orders/${orderId}/chat`, { message: isi });
      setInput('');
      await reload();
    } catch (e) {
      toast.err(pesanError(e, t('chat.sendFail')));
    } finally {
      setSending(false);
    }
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      if (!sending && input.trim()) kirim();
    }
  };

  // Buyer may open a dispute on PAID/PROCESSING/COMPLETED (≤7d). After opening,
  // the order status flips to DISPUTED (backend-enforced; UI mirrors it).
  const bisaBuka = order && !['DISPUTED', 'CANCELLED', 'REFUNDED'].includes(order.status);
  const bukaSengketa = async () => {
    if (alasan.trim().length < 10) {
      toast.err(t('dispute.min'));
      return;
    }
    setDisputeBusy(true);
    try {
      await post(`/market/orders/${orderId}/dispute`, { reason: alasan, evidence_url: bukti.trim() || undefined });
      toast.ok(t('dispute.ok'));
      setDisputeOpen(false);
      setAlasan('');
      setBukti('');
      await reload();
    } catch (e) {
      toast.err(pesanError(e, t('dispute.fail')));
    } finally {
      setDisputeBusy(false);
    }
  };

  return (
    <div className="flex min-h-[calc(100dvh-3.5rem)] flex-col p-4 sm:p-6 max-w-3xl mx-auto">
      <header className="mb-4 pb-3 border-b border-border">
        <Link href="/pesanan" className="inline-flex items-center gap-2 text-sm text-mute hover:text-text mb-2">
          <ArrowLeft className="w-4 h-4" aria-hidden="true" /> {t('pd.back')}
        </Link>
        <div className="flex items-center gap-3">
          <div className="flex-1 min-w-0">
            <p className="label mb-1">{order?.product?.title || t('chat.order')}</p>
            <h1 className="text-xl font-semibold tracking-tight truncate">{t('chat.h')}</h1>
            <p className="text-xs text-mute mt-1">
              {order?.store?.name || '-'} · {tanggal(order?.created_at || order?.created_at_iso)} · {rupiah(order?.subtotal)}
            </p>
          </div>
          <Package className="w-8 h-8 text-mute/40" aria-hidden="true" />
          {order?.status === 'DISPUTED' && (
            <Badge tone="warn" className="ml-2"><AlertCircle className="w-3 h-3 mr-1" aria-hidden="true" />{t('dispute.open')}</Badge>
          )}
          {order?.status === 'DISPUTED' && dispute && (
            <div className="mt-3 rounded-xl border border-border bg-surface-hi px-4 py-3 text-sm">
              <p className="font-medium mb-1">{t('dispute.open')}</p>
              {dispute.seller_response && (
                <p className="text-xs text-mute mt-1"><span className="font-medium">{t('dispute.sellerResponse')}:</span> {dispute.seller_response}</p>
              )}
              {dispute.resolution && (
                <p className="text-xs text-mute mt-1"><span className="font-medium">{t('dispute.resolved')}:</span> {dispute.resolution}</p>
              )}
              {!dispute.resolution && !dispute.seller_response && (
                <p className="text-xs text-mute mt-1">{t('dispute.noOutcome')}</p>
              )}
            </div>
          )}
        </div>
        {bisaBuka && order?.status !== 'DISPUTED' && (
          <Button size="sm" variant="outline" className="mt-3" onClick={() => setDisputeOpen(true)}>
            <AlertCircle className="w-4 h-4" aria-hidden="true" /> {t('dispute.h')}
          </Button>
        )}
      </header>

      <section aria-label="Riwayat pesan" className="flex-1 overflow-y-auto">
        {loading ? (
          <div className="space-y-3">
            {[0, 1, 2].map((i) => (
              <div key={i} className="skel h-12 w-3/4 rounded-lg ml-auto" />
            ))}
          </div>
        ) : messages.length === 0 ? (
          <div className="h-full flex items-center justify-center text-center py-12">
            <p className="text-sm text-mute">{t('chat.empty')}</p>
          </div>
        ) : (
          <ul className="space-y-3 py-2">
            {messages.map((m) => (
              <li key={m.id} className={`flex ${m.is_me ? 'justify-end' : 'justify-start'}`}>
                <div className={`max-w-[80%] rounded-xl px-4 py-2 text-sm leading-relaxed ${
                  m.is_me
                    ? 'bg-cyan-500/20 text-text rounded-br-md'
                    : 'bg-surface-hi text-text rounded-bl-md'
                }`}>
                  <p className="whitespace-pre-wrap break-words">{m.message}</p>
                  <p className="text-[10px] text-mute mt-1 opacity-60">
                    {tanggal(m.created_at, true)}
                  </p>
                </div>
              </li>
            ))}
            <li ref={endRef} aria-hidden="true" />
          </ul>
        )}
      </section>

      <form
        className="mt-4"
        onSubmit={(e) => {
          e.preventDefault();
          if (!sending && input.trim()) kirim();
        }}
      >
        <div className="relative">
          <textarea
            id="pesan"
            rows={1}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={onKeyDown}
            placeholder={t('chat.ph')}
            aria-label={t('chat.sendAria')}
            disabled={sending}
            className="block w-full resize-none rounded-xl border border-border bg-surface-hi text-sm leading-relaxed py-3 pr-12 pl-4 text-text outline-none placeholder:text-mute disabled:opacity-50 focus-within:border-cyan-400/50"
          />
          <button
            type="submit"

            disabled={sending || !input.trim()}
            aria-label={t('chat.send')}
            className="absolute bottom-2.5 right-2.5 grid h-8 w-8 place-items-center rounded-full bg-cyan-400 text-bg transition hover:scale-105 disabled:cursor-not-allowed disabled:opacity-50"
          >
            <Send className="h-4 w-4" aria-hidden="true" />
          </button>
        </div>
      </form>

      <Dialog open={disputeOpen} title={t('dispute.h')} onClose={() => setDisputeOpen(false)}>
        <p className="text-sm text-mute mb-3">{t('dispute.reason')}</p>
        <textarea
          value={alasan}
          onChange={(e) => setAlasan(e.target.value)}
          placeholder={t('dispute.reasonPh')}
          disabled={disputeBusy}
          rows={4}
          className="block w-full resize-none rounded-xl border border-border bg-surface-hi text-sm py-3 px-4 text-text outline-none placeholder:text-mute disabled:opacity-50 focus-within:border-cyan-400/50"
        />
        <input
          type="url"
          value={bukti}
          onChange={(e) => setBukti(e.target.value)}
          placeholder={t('dispute.evidencePh')}
          disabled={disputeBusy}
          className="block w-full rounded-xl border border-border bg-surface-hi text-sm py-2 px-4 text-text outline-none placeholder:text-mute disabled:opacity-50 focus-within:border-cyan-400/50 mt-3"
        />
        <p className="text-xs text-mute mt-2">{t('dispute.evidence')}</p>
        <div className="mt-4 flex gap-2 justify-end">
          <Button size="sm" variant="ghost" disabled={disputeBusy} onClick={() => setDisputeOpen(false)}>{t('ad.common.cancel')}</Button>
          <Button size="sm" variant="destructive" disabled={disputeBusy} onClick={bukaSengketa}>
            {disputeBusy ? t('dispute.submitting') : t('dispute.submit')}
          </Button>
        </div>
      </Dialog>
    </div>
  );
}
