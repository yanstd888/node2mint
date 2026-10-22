'use client';
import PublicHeader from '@/components/PublicHeader';
import PublicFooter from '@/components/PublicFooter';
import { ArtikelSection } from '@/components/sections/ArtikelSection';

// Halaman publik penuh (chrome landing); isi dipakai juga oleh /bantuan.
export default function Page() {
  return (
    <div className="min-h-dvh"><PublicHeader /><main className="p-6 pt-24 md:p-10 md:pt-28 max-w-5xl mx-auto"><ArtikelSection /></main><PublicFooter /></div>
  );
}
