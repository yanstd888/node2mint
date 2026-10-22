import { notFound } from 'next/navigation';
import ArticleDetailClient from './ArticleDetailClient';

type Artikel = { slug?: string; title: string; content?: string; excerpt?: string; image?: string; cover_url?: string; published_at?: string; author?: string; is_admin_post?: boolean };
type ArticleResponse = { article?: Artikel };

export default async function ArtikelDetailPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug: rawSlug } = await params;
  const slug = decodeURIComponent(rawSlug);
  const base = process.env.NEXT_PUBLIC_API_URL || 'http://gateway:8080';
  const response = await fetch(`${base}/api/articles/${encodeURIComponent(slug)}`, { cache: 'no-store' });
  if (!response.ok) notFound();
  const data = await response.json() as ArticleResponse;
  if (!data.article) notFound();
  return <ArticleDetailClient slug={slug} initial={data} />;
}
