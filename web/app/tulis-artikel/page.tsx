'use client';
// Tulis artikel: kirim ke POST /api/articles. Setelah sukses arahkan ke daftar artikel.
import { useEffect, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { post, pesanError } from '@/lib/http';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { RichTextEditor } from '@/components/ui/RichTextEditor';
import { ImageField } from '@/components/ui/ImageField';
import { useT } from '@/lib/i18n';

export default function TulisArtikelPage() {
  const { t } = useT();
  const router = useRouter();
  const [form, setForm] = useState({ title: '', slug: '', excerpt: '', content: '', image: '' });
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState('');
  const [ok, setOk] = useState('');
  const redirectTimer = useRef<number | null>(null);

  useEffect(() => () => {
    if (redirectTimer.current !== null) window.clearTimeout(redirectTimer.current);
  }, []);

  const kirim = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr('');
    setOk('');
    try {
      await post('/articles', {
        title: form.title.trim(),
        slug: form.slug.trim() || undefined,
        excerpt: form.excerpt.trim(),
        content: form.content,
        image: form.image || undefined,
      });
      setOk(t('crt.ok'));
      setForm({ title: '', slug: '', excerpt: '', content: '', image: '' });
      redirectTimer.current = window.setTimeout(() => router.push('/artikel'), 1200);
    } catch (e) {
      setErr(pesanError(e, t('crt.fail')));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="p-6 md:p-10 max-w-3xl mx-auto">
      <header className="mb-8">
        <p className="label mb-1">{t('art.kicker')}</p>
        <h1 className="text-2xl font-semibold tracking-tight">{t('art.writeLong')}</h1>
      </header>

      {err && <p role="alert" className="mb-6 text-sm text-danger bg-danger/10 border border-danger/30 rounded-lg px-4 py-3">{err}</p>}
      {ok && <p role="status" className="mb-6 text-sm text-cyan-200 bg-cyan-500/10 border border-cyan-400/30 rounded-lg px-4 py-3">{ok}</p>}

      <form onSubmit={kirim} className="space-y-5">
        <div className="space-y-2">
          <label htmlFor="judul" className="label">{t('crt.title')}</label>
          <Input id="judul" value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })}
            maxLength={200} required />
        </div>
        <div className="space-y-2">
          <label htmlFor="slug" className="label">{t('crt.slug')}</label>
          <Input id="slug" value={form.slug} onChange={(e) => setForm({ ...form, slug: e.target.value })}
            placeholder="judul-artikel" pattern="[a-z0-9\\-]*" />
          <p className="text-xs text-mute">{t('crt.slugP')}</p>
        </div>
        <div className="space-y-2">
          <label htmlFor="ringkas" className="label">{t('crt.excerpt')}</label>
          <Textarea id="ringkas" value={form.excerpt} onChange={(e) => setForm({ ...form, excerpt: e.target.value })}
            maxLength={300} className="min-h-[60px]" />
        </div>
        <div className="space-y-2">
          <label htmlFor="isi" className="label">{t('crt.content')}</label>
          <RichTextEditor id="isi" value={form.content} onChange={(content) => setForm({ ...form, content })} required />
        </div>
        <ImageField label={t('crt.image')} value={form.image}
          onChange={(url) => setForm({ ...form, image: url })} />
        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={() => router.push('/artikel')}>{t('mkt.cancel')}</Button>
          <Button type="submit" disabled={busy || !form.title.trim() || !form.content.replace(/<[^>]*>/g, '').trim()}>
            {busy ? t('crt.sending') : t('crt.submit')}
          </Button>
        </div>
      </form>
    </div>
  );
}
