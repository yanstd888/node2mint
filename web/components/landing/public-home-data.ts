// Public gateway data used in the first HTML response. These endpoints are anonymous
// catalog reads; authenticated/session-specific data stays client-side.
const API = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export type PublicHomeData = {
  models: unknown | null;
  packages: unknown | null;
  featured: unknown | null;
  bestSellers: unknown | null;
  newest: unknown | null;
  categories: unknown | null;
  status: unknown | null;
};

async function read(path: string): Promise<unknown | null> {
  try {
    const response = await fetch(`${API}${path}`, {
      cache: 'no-store',
      signal: AbortSignal.timeout(3000),
    });
    return response.ok ? await response.json() : null;
  } catch {
    return null;
  }
}

export async function getPublicHomeData(includeMarketplace = true): Promise<PublicHomeData> {
  const [models, packages, featured, status] = await Promise.all([
    read('/api/models'),
    read('/api/packages'),
    read('/api/market/featured?per_page=8'),
    read('/api/status'),
  ]);
  const [bestSellers, newest, categories] = includeMarketplace
    ? await Promise.all([
      read('/api/market/products?sort=best_selling&available=true&per_page=8'),
      read('/api/market/products?sort=newest&available=true&per_page=8'),
      read('/api/market/categories'),
    ])
    : [null, null, null];
  return { models, packages, featured, bestSellers, newest, categories, status };
}
