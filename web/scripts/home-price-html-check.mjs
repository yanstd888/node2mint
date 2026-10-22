import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..');
const page = readFileSync(resolve(root, 'app/page.tsx'), 'utf8');
const client = readFileSync(resolve(root, 'components/landing/HomeClient.tsx'), 'utf8');
const cards = readFileSync(resolve(root, 'components/landing/PricingCards.tsx'), 'utf8');
const pricing = readFileSync(resolve(root, 'app/pricing/page.tsx'), 'utf8');
const pricingFrame = readFileSync(resolve(root, 'components/landing/PricingPageFrame.tsx'), 'utf8');

assert.match(page, /getPublicHomeData\(\)/);
assert.match(page, /<PricingCards data=\{data\.packages\} compact \/>/);
assert.match(client, /pricingCards: ReactNode/);
assert.match(client, /\{pricingCards\}/);
assert.doesNotMatch(cards, /^['"]use client['"]/m);
assert.match(cards, /\{money\(p\.price\)\}/);
assert.match(pricing, /<PricingPageFrame>/);
assert.doesNotMatch(pricing, /PublicHeader|PublicFooter/);
assert.match(pricingFrame, /const \{ user \} = useAuth\(\)/);
assert.match(pricingFrame, /\{!user && <PublicHeader \/>\}/);
assert.match(pricingFrame, /user \? 'p-6 md:p-10'/);
assert.match(pricingFrame, /\{!user && <PublicFooter \/>\}/);

console.log('PASS: homepage keeps server-rendered package prices; pricing switches to dashboard spacing without public chrome after login.');
