'use client';
import PublicHeader from '@/components/PublicHeader';
import PublicFooter from '@/components/PublicFooter';
import { DocsSection } from '@/components/sections/DocsSection';

// Halaman publik penuh (chrome landing); isi dipakai juga oleh /bantuan.
export default function Page() {
  return (
    <div className="min-h-dvh"><PublicHeader /><main className="p-6 pt-24 md:p-10 md:pt-28 max-w-4xl mx-auto"><DocsSection /></main><PublicFooter /></div>
  );
}
