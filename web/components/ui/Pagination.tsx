'use client';
// Pagination: Sebelumnya/Berikutnya + "Halaman N". Gateway membalas has_more & page.
import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import { useT } from '@/lib/i18n';

export function useClientPagination<T>(all: T[], perPage = 10) {
  const [page, setPage] = useState(1);
  const pages = Math.max(1, Math.ceil(all.length / perPage));
  useEffect(() => setPage((current) => Math.min(current, pages)), [pages]);
  return {
    page,
    setPage,
    items: all.slice((page - 1) * perPage, page * perPage),
    hasMore: page < pages,
  };
}

export function Pagination({ page, hasMore, onPage }: { page: number; hasMore?: boolean; onPage: (p: number) => void }) {
  const { t } = useT();
  return (
    <nav aria-label={t('pg.nav')} className="flex items-center justify-center gap-3 mt-8">
      <Button size="sm" variant="outline" disabled={page <= 1} onClick={() => onPage(page - 1)}>
        <ChevronLeft className="w-4 h-4" aria-hidden="true" /> {t('pg.prev')}
      </Button>
      <span className="text-sm text-mute tabular-nums" aria-live="polite">{t('pg.page', { n: page })}</span>
      <Button size="sm" variant="outline" disabled={!hasMore} onClick={() => onPage(page + 1)}>
        {t('pg.next')} <ChevronRight className="w-4 h-4" aria-hidden="true" />
      </Button>
    </nav>
  );
}
