import ModelCards from '@/components/landing/ModelCards';
import PricingCards, { BillingExplanation } from '@/components/landing/PricingCards';
import PricingPageFrame from '@/components/landing/PricingPageFrame';
import { PricingPageExample, PricingPageIntro } from '@/components/PublicPageContent';
import { getPublicHomeData } from '@/components/landing/public-home-data';

export default async function PricingPage() {
  const data = await getPublicHomeData(false);
  return (
    <PricingPageFrame>
      <PricingPageIntro />
      <PricingCards data={data.packages} />
      <BillingExplanation models={data.models} />
      <PricingPageExample />
      <ModelCards initialModels={data.models} />
    </PricingPageFrame>
  );
}