'use client';
// Upload gambar sekali pakai di mana-mana (produk, artikel): pilih berkas →
// POST /api/media (multipart "file") → {url:"/media/<id>"} → onChange(url).
// Preview + tombol hapus (hapus = set value '', berkas tetap tersimpan di gateway).
import { useRef, useState } from 'react';
import { post, pesanError } from '@/lib/http';

const MAX_IMAGE_BYTES = 1024 * 1024;
const MAX_IMAGE_EDGE = 1600;

async function optimizeImage(file: File): Promise<File> {
  if (file.type === 'image/gif' || file.size <= 256 * 1024) return file;
  const bitmap = await createImageBitmap(file);
  const scale = Math.min(1, MAX_IMAGE_EDGE / Math.max(bitmap.width, bitmap.height));
  const canvas = document.createElement('canvas');
  canvas.width = Math.max(1, Math.round(bitmap.width * scale));
  canvas.height = Math.max(1, Math.round(bitmap.height * scale));
  canvas.getContext('2d')!.drawImage(bitmap, 0, 0, canvas.width, canvas.height);
  bitmap.close();
  const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/webp', 0.82));
  if (!blob || blob.size >= file.size) return file;
  return new File([blob], `${file.name.replace(/\.[^.]+$/, '')}.webp`, { type: 'image/webp', lastModified: file.lastModified });
}
import { Button } from '@/components/ui/button';
import { ImagePlus, Trash2 } from 'lucide-react';
import { useT } from '@/lib/i18n';

export function ImageField({ label, value, onChange }: { label: string; value: string; onChange: (url: string) => void }) {
  const { t } = useT();
  const ref = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState('');

  const upload = async (f: File) => {
    if (!f.type.startsWith('image/')) return;
    setBusy(true);
    setErr('');
    try {
      const optimized = await optimizeImage(f);
      if (optimized.size > MAX_IMAGE_BYTES) {
        setErr(t('img.tooBig'));
        return;
      }
      const fd = new FormData();
      fd.append('file', optimized);
      // Relative URL: lewat rewrite Next, jadi cookie sesi (HttpOnly) ikut
      // terkirim tanpa skrip ini harus menyentuh token apa pun.
      const { data } = await post<{ url: string }>('/api/media', fd);
      onChange(data?.url || '');
    } catch (e) {
      setErr(pesanError(e, t('img.uploadFail')));
    } finally {
      setBusy(false);
      if (ref.current) ref.current.value = '';
    }
  };

  return (
    <div className="space-y-2">
      <span className="label">{label}</span>
      {value ? (
        <div className="flex items-center gap-3">
          <img src={value} alt="Pratinjau gambar" className="w-32 h-24 object-cover rounded-lg border border-border" />
          <Button type="button" size="sm" variant="outline" onClick={() => onChange('')}>
            <Trash2 className="w-4 h-4" aria-hidden="true" /> Hapus
          </Button>
        </div>
      ) : (
        <label className="inline-flex items-center gap-2 text-sm text-mute border border-dashed border-border rounded-lg px-4 py-3 cursor-pointer hover:text-text hover:border-cyan-400/40 transition-colors">
          <ImagePlus className="w-4 h-4" aria-hidden="true" />
          {busy ? t('img.uploading') : t('img.pick')}
          <input ref={ref} type="file" className="sr-only" accept="image/png,image/jpeg,image/webp,image/gif"
            onChange={(e) => { const f = e.target.files?.[0]; if (f) upload(f); }} />
        </label>
      )}
      {err && <p role="alert" className="text-xs text-danger">{err}</p>}
    </div>
  );
}
