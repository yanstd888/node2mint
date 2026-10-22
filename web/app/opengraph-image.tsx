import { ImageResponse } from 'next/og';
import { cookies } from 'next/headers';
import { en, id } from '@/lib/dict/public-pages';

export const size = { width: 1200, height: 630 };
export const contentType = 'image/png';
export const alt = 'Node2Mint — API AI Multi-Model dan Pasar Digital';

export default async function OpenGraphImage() {
  const copy = (await cookies()).get('locale')?.value === 'id' ? id : en;
  return new ImageResponse(
    <div style={{ width: '100%', height: '100%', display: 'flex', flexDirection: 'column', justifyContent: 'space-between', padding: 72, color: '#f8fafc', background: 'linear-gradient(135deg, #071015 0%, #0d1720 55%, #102a35 100%)' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 20, fontSize: 34, fontWeight: 700 }}>
        <div style={{ display: 'flex', width: 58, height: 58, alignItems: 'center', justifyContent: 'center', borderRadius: 14, background: '#67e8f9', color: '#071015', fontSize: 34, fontWeight: 900 }}>N</div>
        Node2Mint
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 22 }}>
        <div style={{ display: 'flex', flexDirection: 'column', fontSize: 70, lineHeight: 1.04, letterSpacing: -3, fontWeight: 800 }}>{copy['og.title']}</div>
        <div style={{ fontSize: 28, color: '#a5f3fc' }}>{copy['og.sub']}</div>
      </div>
      <div style={{ display: 'flex', fontSize: 24, color: '#94a3b8' }}>node2mint.com</div>
    </div>,
    size,
  );
}
