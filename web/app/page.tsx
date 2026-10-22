// Public catalog, package prices, and featured products are fetched on the server so
// the first response contains real values without waiting for client JavaScript.
import HomeClient from '@/components/landing/HomeClient';
import PricingCards from '@/components/landing/PricingCards';
import { getPublicHomeData } from '@/components/landing/public-home-data';

export default async function RootPage() {
  const data = await getPublicHomeData();
  return <HomeClient initialModels={data.models} initialFeatured={data.featured} initialBestSellers={data.bestSellers} initialStatus={data.status} pricingCards={<PricingCards key="home-pricing" data={data.packages} compact />} />;
}
