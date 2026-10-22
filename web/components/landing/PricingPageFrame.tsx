'use client';

import type { ReactNode } from 'react';
import { useAuth } from '@/lib/auth';
import PublicFooter from '@/components/PublicFooter';
import PublicHeader from '@/components/PublicHeader';

export default function PricingPageFrame({ children }: { children: ReactNode }) {
  const { user } = useAuth();

  return (
    <div className="min-h-dvh">
      {!user && <PublicHeader />}
      <main className={`mx-auto max-w-6xl ${user ? 'p-6 md:p-10' : 'px-4 pb-16 pt-28 sm:px-6 md:pt-32'}`}>
        {children}
      </main>
      {!user && <PublicFooter />}
    </div>
  );
}
