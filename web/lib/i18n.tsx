'use client';

// ponytail: flat dictionary + {name} slots. Add ICU/plurals when a string needs them.

import { createContext, useCallback, useContext, useEffect, useState, ReactNode } from 'react';
import { usePathname } from 'next/navigation';

import { appEn, appId } from './dict';
import { patch } from './http';
import generatedLocaleOverrides from './dict/locale-overrides.generated.json';
import uiLocaleOverrides from './dict/locale-ui-overrides.json';

type AdminEnDict = typeof import('./dict/admin')['en'];
type AdminIdDict = typeof import('./dict/admin')['id'];



export type Lang = 'en' | 'id' | 'cn' | 'ru' | 'jp';

const KEY = 'locale';
const COOKIE = 'locale';

// Tidak ada penebakan locale di klien. Server (layout.tsx) memutuskan locale dari
// cookie/x-locale dan sudah merender teksnya; menebak dari navigator/timezone
// setelah hidrasi mengubah bahasa dan memicu React #418. Pengguna tetap bisa
// memilih bahasa lewat tombol (setLang) atau URL ber-prefix.

// URL locale (/en /id /zh /ru /ja) → internal dictionary code. Middleware maps
// the same prefixes onto the unprefixed routes; this only reads them back.
const URL_LANGS = ['en', 'id', 'zh', 'ru', 'ja'] as const;
const URL_LANG: Record<string, Lang> = { en: 'en', id: 'id', zh: 'cn', ru: 'ru', ja: 'jp' };

export const resolveLang = (pathname: string): Lang | null => {
  const seg = pathname.split('/')[1];
  return URL_LANGS.includes(seg as (typeof URL_LANGS)[number]) ? URL_LANG[seg] : null;
};

// Internal dictionary code → URL segment (cn→zh, jp→ja).
const LANG_URL: Record<Lang, string> = { en: 'en', id: 'id', cn: 'zh', ru: 'ru', jp: 'ja' };
const HTML_LANG: Record<Lang, string> = { en: 'en', id: 'id', cn: 'zh', ru: 'ru', jp: 'ja' };

// usePathname() tetap memuat prefix locale (URL kanonik). Perbandingan path
// (nav aktif, daftar halaman publik) membuang prefix itu.
const INTERNAL_ROUTE: Record<string, string> = { articles: 'artikel', 'sign-in': 'masuk', help: 'bantuan', 'forgot-password': 'lupa-password', cart: 'keranjang', orders: 'pesanan', settings: 'pengaturan', store: 'toko', affiliate: 'afiliasi', 'write-article': 'tulis-artikel' };
export const stripLocale = (pathname: string) => {
  const parts = (pathname.replace(/^\/(en|id|zh|ru|ja)(?=\/|$)/, '') || '/').split('/');
  if (INTERNAL_ROUTE[parts[1]]) parts[1] = INTERNAL_ROUTE[parts[1]];
  return parts.join('/');
};

// Prefix locale dari konteks (disemai server, diperbarui saat navigasi).
export const useUrlPrefix = () => useContext(Ctx).prefix;



const base = {

  en: {

    'nav.market': 'Marketplace', 'nav.about': 'About', 'nav.models': 'Models', 'nav.pricing': 'Pricing',

    'nav.blog': 'Blogs/News', 'nav.docs': 'Docs', 'nav.status': 'Service Status', 'nav.faq': 'FAQ',

    'nav.menu': 'Menu', 'nav.home': 'Node2Mint, home', 'nav.dashboard': 'Dashboard', 'nav.signin': 'Start for free',

    'nav.lang': 'Language',

    'about.kicker': 'About Node2Mint', 'about.h': 'One account for AI and digital commerce.', 'about.sub': 'Node2Mint brings multi-model AI access, token usage, and a digital marketplace together in one clear account experience.', 'about.pillarsAria': 'What Node2Mint provides', 'about.apiH': 'Multi-model AI API', 'about.apiP': 'Use one OpenAI-compatible endpoint to work with the available models while keeping token usage visible in your dashboard.', 'about.marketH': 'Digital marketplace', 'about.marketP': 'Discover and sell useful digital products through seller storefronts, reviews, verified signals, and clear order flows.', 'about.accountH': 'One account, clear ledgers', 'about.accountP': 'Wallet funds, API token quota, referral rewards, and affiliate earnings stay separate so every balance has one meaning.', 'about.trustKicker': 'Our approach', 'about.trustH': 'Useful tools should stay understandable.', 'about.trustP': 'We prefer explicit pricing, server-verified actions, honest empty states, and product flows that show what happens before money or access changes.', 'about.marketCta': 'Browse marketplace', 'about.contactCta': 'Contact support',

    'foot.market': 'Browse Marketplace', 'foot.blog': 'Blogs/News', 'foot.docs': 'Documentation',
    'foot.status': 'Service Status', 'foot.nav': 'Footer links',
    'foot.product': 'Product', 'foot.learn': 'Learn', 'foot.community': 'Community', 'foot.company': 'Company',
    'foot.build': 'Developers', 'foot.earn': 'Earn', 'foot.support': 'Support',
    'foot.communityDiscord': 'Discord', 'foot.communityTelegram': 'Telegram', 'foot.communityX': 'X', 'foot.communityYoutube': 'YouTube',
    'foot.guides': 'Guides', 'foot.faq': 'FAQ', 'foot.changelog': 'Changelog',
    'foot.terms': 'Terms', 'foot.privacy': 'Privacy', 'foot.refund': 'Refund policy',
    'foot.models': 'Model catalog', 'foot.packs': 'Token packages', 'foot.pricing': 'Pricing',
    'foot.keys': 'API keys', 'foot.playground': 'Playground',
    'foot.sell': 'Sell on Node2Mint', 'foot.affiliate': 'Affiliate', 'foot.referral': 'Referral',
    'foot.help': 'Help center', 'foot.articles': 'Articles',
    'foot.about': 'One account for AI models, token quota, a digital marketplace, affiliate commissions, and referral token rewards in one account.',
        'privacy.cookieTitle': 'Your privacy matters', 'privacy.cookieBody': 'We use essential cookies to keep the site working and remember your preferences.', 'privacy.cookieAccept': 'Accept',
    'foot.copyright': '© 2026 Node2Mint', 'foot.signature': 'AI API and digital marketplace', 'foot.credit': '· Built with', 'foot.creditAria': 'Built with love by YanSTD',
    'foot.openDocs': 'Open documentation', 'foot.openStatus': 'Open service status', 'foot.openMarket': 'Open marketplace',

    'hero.kicker': 'Digital marketplace + AI API tokens', 'hero.h1': 'Digital Marketplace & AI API Tokens, Together',

    'hero.sub': 'Buy and sell digital products, access multiple AI models, and manage wallet and token usage from one account.',

    'hero.cta': 'Create free account', 'hero.ctaIn': 'Open dashboard', 'hero.explore': 'Browse AI models', 'hero.seeAll': 'See all {n} models', 'hero.marketCta': 'Open marketplace', 'hero.apiCta': 'Use AI API',

    'hero.card': 'Live model catalog', 'hero.live': 'Connected', 'hero.statModels': 'models available', 'hero.statBase': 'OpenAI-compatible endpoint', 'hero.marketCard': 'Digital marketplace', 'hero.apiCard': 'AI API tokens', 'hero.walletCode': 'Marketplace wallet + /v1/chat/completions', 'market.banner': 'Marketplace banner', 'market.bannerNum': 'Marketplace banner {n}', 'market.prev': 'Previous banner', 'market.next': 'Next banner', 'market.open': 'Open banner {n}',

    'home.ledgerKicker': 'Clear billing', 'home.ledgerH': 'Money and API quota stay separate.',

    'home.ledgerP': 'Your wallet holds real money. Token quota pays for API calls. The dashboard shows both ledgers without mixing them.',

    'home.walletH': 'Wallet', 'home.walletP': 'Top up, buy marketplace products, receive affiliate earnings, and withdraw available funds.',

    'home.tokensH': 'API tokens', 'home.tokensP': 'Buy a token package, use it across available models, and track consumption per API key.',

    'home.cacheH': 'Cached input costs less', 'home.cacheP': 'Repeated context is detected from provider usage and charged at a discounted token rate.',

    'home.earnKicker': 'Partnerships', 'home.earnH': 'Share a product or invite a new user. The reward is different.',

    'home.earnP': 'Affiliate and referral are separate programs, with separate reward ledgers.',

    'home.affH': 'Affiliate', 'home.affP': 'Create a link for a marketplace product. Purchases attributed to your link generate earnings in your wallet.',

    'home.affReward': 'Reward: real-money wallet balance', 'home.affCta': 'Open affiliate tools',

    'home.refH': 'Referral', 'home.refP': 'Invite someone to create an account. When their first top-up settles, both accounts receive an API token package.',

    'home.refReward': 'Reward: API token package', 'home.refCta': 'Open referral program',

    'home.finalH': 'Your first request starts with one account.',

    'home.finalP': 'Create an account, choose a token package, and call your first model through one endpoint.',

    'home.finalCta': 'Create free account', 'home.finalIn': 'Open dashboard', 'home.finalDocs': 'Read the API docs',

    'models.kicker': 'Model catalog', 'models.h': 'Choose the model. Keep the same API.',

    'models.sub': 'Model names, context windows, and health come from the live catalog.', 'models.base': 'Base URL',

    'models.copy': 'Copy base URL', 'models.copied': 'Copied.', 'models.available': '{n} models available', 'models.context': '{n} context',

    'models.healthy': 'Operational', 'models.unknown': 'No health data', 'models.all': 'View API documentation',

    'models.other': 'Other', 'models.freeTag': 'Free', 'models.paidTag': 'Paid', 'models.copyId': 'Copy model ID', 'models.joinToUnlock': 'JOIN TO UNLOCK',
    'models.freeTerms': 'Sign-in required · {n} API tokens/day · excess uses your token-package balance.',
    'models.paidTerms': 'Consumes API tokens at ×{n}; an active token package is required.',

    'models.usage': 'Token usage · multiplier', 'models.docs': 'Open API documentation',

    'models.showAll': 'Show all {n} models', 'models.showLess': 'Show fewer',

    'pack.kicker': 'Token packages', 'pack.h': 'Pay for usage, not another subscription.',

    'pack.sub': 'One token balance works across available models. Cached input is charged at a lower rate.',

    'pack.popular': 'Balanced option', 'pack.active': 'Active {n} days', 'pack.allModels': 'Use on all available models', 'pack.cta': 'Compare packages',

    'faq.kicker': 'FAQ', 'faq.h': 'Questions, answered.',

    'faq.sub': 'Clear answers about API access, token usage, wallet funds, and the marketplace.',

    'faq.docs': 'Read the docs', 'faq.status': 'Service status',

    'faq.q1': 'Are wallet balance and API tokens the same?',

    'faq.a1': 'No. Wallet balance is money used for top-ups, marketplace purchases, seller earnings, and withdrawals. API tokens are the quota consumed by model requests.',

    'faq.q2': 'Is the API compatible with OpenAI?',

    'faq.a2': 'Yes. Keep the OpenAI SDK and chat completions format, then change the base URL, API key, and public model name.',

    'faq.q3': 'How is model usage charged?',

    'faq.a3': 'Each request deducts API tokens from effective input and output. Provider-reported cached input receives the platform cache discount; the usage surcharge and selected model multiplier apply before settlement.',

    'faq.q4': 'What is the difference between affiliate and referral?',

    'faq.a4': 'Affiliate commissions come from marketplace product purchases and are paid to the wallet. Referral rewards add API tokens after a referred user completes the qualifying first top-up.',

    'faq.q5': 'What can I buy or sell in the marketplace?',

    'faq.a5': 'The marketplace supports digital products such as prompts, datasets, tools, templates, and digital services. Buyers pay from their wallet, while sellers manage stock and delivery from their store.',

    'faq.q6': 'What happens when my API tokens run out?',

    'faq.a6': 'New API requests are rejected until another token package is activated or a valid voucher is redeemed. Your wallet balance is not used automatically for API requests.',

    'faqPage.h': 'Frequently asked questions', 'faqPage.q1': 'What is the difference between wallet and tokens?', 'faqPage.a1': 'Wallet stores money for transactions. Tokens are the quota used for API requests.',
    'faqPage.q2': 'When does a seller receive funds?', 'faqPage.a2': 'When the buyer confirms the order, or automatically 48 hours after the seller marks it delivered, unless there is an open dispute.',
    'faqPage.q3': 'How does referral work?', 'faqPage.a3': 'Referrals that pass local IP checks receive 2.5 million tokens and one active day for both the referrer and new user.',
    'faqPage.q4': 'How do I top up my wallet and buy a token package?', 'faqPage.a4': 'Open the Wallet page and pick a payment method. Once the payment settles, the balance appears in your wallet; buying a token package adds tokens and its active period to your account.',
    'faqPage.q5': 'What can I buy or sell in the marketplace?', 'faqPage.a5': 'Digital products such as prompts, datasets, tools, templates, and digital services. Buyers pay from the wallet; sellers control stock, delivery, and disputes from their store.',
    'faqPage.q6': 'What happens when my API tokens run out?', 'faqPage.a6': 'New API requests are rejected until another token package is activated or a valid redeem code is used. Your wallet balance is never charged automatically for API requests.',
    'faqPage.q7': 'How do I keep my account safe?', 'faqPage.a7': 'Turn on two-factor authentication in the Security tab, keep your API keys secret (a key is shown only once and stored hashed), and end any session you do not recognise from that same tab.',
    'faqPage.q8': 'What is the difference between affiliate and referral?', 'faqPage.a8': 'Affiliate pays a real-money wallet commission from marketplace product sales attributed to your link. Referral adds API tokens after the person you invited completes the qualifying first top-up.',
    'faqPage.q9': 'How do I open a dispute, and when can I get a refund?', 'faqPage.a9': 'Open a dispute from the order while it is PAID or PROCESSING, or within 7 days after the order is marked complete. A refund is paid from the order escrow while the funds are still held; if the seller funds were already released, the case moves to manual review.',
    'faqPage.q10': 'Can I get a refund for a token package?', 'faqPage.a10': 'Consumed tokens are never refunded. If you bought a package that is still unused, you can ask the help center to review that transaction; an activated package keeps working until its active period ends.',
    'faqPage.q11': 'Does Node2Mint charge a fee on withdrawals?', 'faqPage.a11': 'A 5% platform fee applies to wallet withdrawals. The fee and the amount you receive are shown before you confirm the request.',
    'faqPage.q12': 'What content is not allowed in the marketplace?', 'faqPage.a12': 'Products and articles that are illegal, harmful, or violate the rights of others. The platform can hide or remove listings and restrict accounts that break these rules.',
    'faqPage.help': 'Did not find your answer?', 'faqPage.helpLink': 'Open the help center',
    'faqPage.support': 'You can also reach us through the contact page.',

    // F-06: jalur bantuan yang benar-benar bisa dipakai. Tanpa ini Help/Refund/
    // lupa-password hanya menyuruh "hubungi admin" tanpa tujuan.
    'sup.kicker': 'Support', 'sup.h': 'Contact support',
    'sup.sub': 'One place for pending top-ups, withdrawals, refunds, marketplace disputes, and locked accounts.',
    'sup.emailH': 'Email support', 'sup.subject': 'Node2Mint support request',
    'sup.emailP': 'Write to {email} from your registered email address so we can match the account.',
    'sup.emailCta': 'Open email draft',
    'sup.copyEmail': 'Copy email address', 'sup.copied': 'Email address copied.',
    'sup.hoursH': 'Response window',
    'sup.hoursP': 'First reply within 3–5 business days (Mon–Fri, WIB). Include the order ID or top-up reference to speed up the review.',
    'sup.needH': 'Include this',
    'sup.need1': 'Registered account email',
    'sup.need2': 'Order ID, withdrawal ID, or top-up reference',
    'sup.need3': 'Amount, date, and destination (bank/e-wallet/network)',
    'sup.need4': 'Screenshots or transaction hash when relevant',
    'sup.escalateH': 'Money movement',
    'sup.escalateP': 'Top-up and withdrawal requests wait for admin verification; marketplace funds are held in escrow. Escalate to the same address if a money request has no reply within 3–5 business days (Mon–Fri, WIB) and include the reference.',
    'sup.resetH': 'Locked out of your account?',
    'sup.resetP': 'Request a self-service password-reset link from the email attached to your account.',
    'sup.resetBtn': 'Request a password reset',
    'sup.faq': 'Read the FAQ', 'sup.docs': 'Open documentation', 'sup.status': 'Service status',

    'docs.examplePrompt': 'Hello!', 'price.signupGiven': 'Already included with sign-up',

    'show.kicker': 'Digital marketplace', 'show.h': 'Sell useful AI assets, not shelfware.',

    'show.sub': 'List prompts, datasets, tools, templates, or digital services. Buyers pay from their wallet and receive digital delivery.',

    'show.all': 'Browse the catalog', 'show.out': 'Out of stock', 'show.new': 'New', 'show.instant': 'Digital delivery', 'show.stock': 'Stock {n}', 'show.buy': 'View product', 'show.featured': 'Featured marketplace', 'show.best': 'Best sellers', 'show.newest': 'New arrivals', 'show.sales': '{n} sold', 'show.reviews': '{n} reviews',

    'articles.kicker': 'Guides and updates', 'articles.h': 'Learn the product before you commit.', 'articles.all': 'Read all articles',

    'articles.empty': 'No published articles yet. Check back soon.', 'articles.official': 'Official', 'articles.community': 'Community',

    'ticker': 'Available model providers',

    'err.load': 'Could not load this. Check the connection and reload.',

    'price.kicker': 'Token packages', 'price.h': 'One token balance for every model',

    'price.sub': 'Choose the token package that fits your workload. One active balance works across every available model.', 'price.cacheH': 'Cached context costs less', 'price.cacheTip': 'When a provider reports repeated context as cached input, it uses fewer tokens than regular input.',
    'price.currencyBasis': 'Indonesian prices use IDR; other locales use USD at a fixed 1 USD = Rp{n}. The settlement ledger remains IDR.',
    'price.billingFormula': 'Each request uses token balance based on the model input and output. Repeated context can use fewer tokens when the provider reports it as cached input; each model card shows its usage multiplier.',
    'price.signupTerms': 'Included once at sign-up. The token amount and active period shown above apply.',

    'price.currency': 'Choose currency', 'price.rate': 'Rate: 1 USD = Rp{n}',

    'price.empty': 'No packs available yet.', 'price.popular': 'Most popular', 'price.tokens': '{n} tokens',
    'price.tokensBonus': '{base} tokens + {bonus} bonus ({total} total)',
    'price.freeLimits': 'Free registration/referral access: 20 requests/minute and 250,000 tokens/minute.',
    'price.paidLimits': 'Active Mini through Pro Max pack: 55 requests/minute and 1,000,000 tokens/minute.',

    'price.days': 'Active {n} days', 'price.allModels': 'All models', 'price.buy': 'Buy pack', 'price.buying': 'Processing…',
    'price.daily': 'Daily', 'price.monthly': 'Monthly',

    'price.bought': 'Pack "{name}" is active. Tokens have been added.', 'price.buyFail': 'Could not buy the pack. Try again.',

    'price.models': 'Model catalog', 'price.modelsSub': 'Every model uses the same endpoint. Usage is charged per token, with the multiplier shown on each card.',

    'price.noModels': 'No models registered yet.',

    'th.model': 'Model', 'th.tier': 'Tier', 'th.provider': 'Provider', 'th.in': 'Input / 1k', 'th.out': 'Output / 1k',

    'th.ctx': 'Context', 'th.type': 'Type', 'th.ctxTokens': '{n} tokens', 'fam.other': 'Other',

    'price.faq': 'Common questions',

    'pf.q1': 'How do I buy a token package?', 'pf.a1': 'Choose a package, continue to payment, and complete the selected method. The package activates after payment is confirmed.',

    'pf.q2': 'What is the difference between wallet balance and API tokens?', 'pf.a2': 'Wallet balance is money for platform transactions. API tokens are the quota used by model requests and come from an active token package or a valid redeem code.',

    'pf.q3': 'What are the API access limits?', 'pf.a3': 'Free registration or referral access has 20 requests per minute and 250,000 tokens per minute. An active Mini through Pro Max package has 55 requests per minute and 1,000,000 tokens per minute.',

    'pf.q4': 'Does one package work for every model?', 'pf.a4': 'Yes. One active token balance is shared across every available model. Usage differs by the multiplier shown on each model card.',

    'pf.q5': 'What happens when tokens or the active period run out?', 'pf.a5': 'New API requests are rejected until another package is activated or a valid redeem code is used. Wallet balance is not charged automatically for API usage.',

    'art.kicker': 'Content', 'art.h': 'Articles', 'art.write': 'Write', 'art.writeLong': 'Write an article',

    'art.empty': 'No articles yet', 'art.emptyP': 'Published articles will show up here.', 'art.team': 'Team', 'art.community': 'Community',

    'art.more': 'Read more', 'art.back': 'Back to articles', 'art.fail': 'This article could not be loaded', 'art.emptyBody': 'This article is empty.',

    'art.all': 'All articles',

    'chat.start': 'Where should we start?', 'chat.placeholder': 'Type a message…',

    'chat.model': 'Model',

    'chat.loadingModels': 'Loading models…', 'chat.noModels': 'No models available',

    'chat.user': 'You', 'chat.assistant': 'Assistant',

    'chat.cost': 'Cost {n}', 'chat.writing': 'Writing a response…',

    'mkt.kicker': 'Shop', 'mkt.h': 'Marketplace', 'mkt.sellCta': 'Start selling', 'mkt.empty': 'No products yet', 'mkt.emptyP': 'New products will show up here once they are listed.', 'mkt.emptyNoDemoP': 'There are no active listings yet. We do not show fictional sample products; products appear after a seller publishes them.',

    'mkt.noDesc': 'No description.', 'mkt.inStock': '{n} in stock', 'mkt.soldOut': 'Sold out', 'mkt.demo': 'Demo · not for sale', 'mkt.demoNote': 'This demo listing is for browsing only and cannot be bought or added to cart.', 'mkt.reviews': '{n} reviews', 'mkt.reviewsTitle': 'Buyer reviews', 'mkt.noReviews': 'No reviews yet.', 'mkt.buyer': 'Buyer',

    'mkt.cart': 'Cart', 'mkt.carting': 'Adding…', 'mkt.review': 'Review', 'mkt.buy': 'Buy now', 'mkt.buying': 'Processing…', 'mkt.out': 'Out of stock',

    'mkt.bought': 'Purchase complete. Your balance was charged automatically.', 'mkt.noFunds': 'Not enough balance. Top up from the dashboard first.',

    'mkt.buyFail': 'Purchase failed. Try again.', 'mkt.added': 'Added to cart.', 'mkt.noCart': 'This product cannot go in the cart.',

    'mkt.cartFail': 'Could not add to cart. Try again.', 'mkt.reviewed': 'Review sent. Thank you!', 'mkt.reviewFail': 'Could not send the review. Try again.',
    'mkt.wishlistAdd': 'Save to wishlist', 'mkt.wishlistRemove': 'Remove from wishlist', 'mkt.wishlistSaved': 'Saved to your wishlist.', 'mkt.wishlistRemoved': 'Removed from your wishlist.', 'mkt.wishlistFail': 'Could not update your wishlist. Try again.',

    'mkt.products': 'products', 'mkt.orders': 'orders', 'pd.qty': 'Qty', 'pd.inc': 'Increase quantity', 'pd.dec': 'Decrease quantity',

    'mkt.loadFail': 'Could not load products. Check the connection and reload.',

    'mkt.reviewNote': 'Reviews can only be sent for products you have bought.', 'mkt.rating': 'Rating', 'mkt.stars': '{n} stars',

    'mkt.comment': 'Comment (optional)', 'mkt.commentPh': 'What did you think of this product?', 'mkt.cancel': 'Cancel',

    'mkt.send': 'Send review', 'mkt.sending': 'Sending…', 'mkt.reviewOf': 'Review {name}',

    'mkt.search': 'Search products', 'mkt.searchPh': 'Search by title or description', 'mkt.cats': 'Categories', 'mkt.catPremiumsApp': 'Premium apps', 'mkt.catSoulSkill': 'Soul & skill', 'mkt.catToolsSoftware': 'Tools & software', 'mkt.catEbook': 'eBook', 'mkt.catAccounts': 'Accounts', 'mkt.catOthers': 'Others',

    'mkt.moreFromStore': 'Other products from this store', 'mkt.moreFromAll': 'Other products from all stores',

    'mkt.all': 'All', 'mkt.forYou': 'For you', 'mkt.verified': 'Verified', 'mkt.reset': 'Reset filters', 'mkt.filter': 'Filter & sort', 'mkt.sort': 'Sort by', 'mkt.sortNewest': 'Newest', 'mkt.sortBest': 'Best sellers', 'mkt.sortLow': 'Price: low to high', 'mkt.sortHigh': 'Price: high to low', 'mkt.priceMin': 'Minimum price', 'mkt.priceMax': 'Maximum price', 'mkt.instantOnly': 'Instant delivery', 'mkt.availableOnly': 'In stock', 'mkt.apply': 'Apply', 'mkt.close': 'Close', 'mkt.noMatch': 'No products found', 'mkt.noMatchP': 'Try another search or category.',
    'mkt.infoTitle': 'Marketplace information', 'mkt.infoVerifiedH': 'Store verification and moderation', 'mkt.infoVerifiedP': 'The Verified badge means the store has passed the platform verification step. It is not a guarantee that every product is suitable or high quality. Listings can be hidden or removed when they violate marketplace rules.', 'mkt.infoFeeH': 'Marketplace fees', 'mkt.infoFeeP': 'The order detail separates the product price, platform fee, and seller amount. The applicable fee is taken during order settlement, so buyers can see the product price before completing payment.', 'mkt.infoPayoutH': 'Seller balance and withdrawals', 'mkt.infoPayoutP': 'Seller proceeds remain in the store balance until the order is completed. Once funds are available, the seller can request a withdrawal; the applicable fee and net amount are shown before confirmation.',

    'pd.missing': 'Product not found', 'pd.left': '{n} left', 'pd.out': 'Sold out',

    'pd.listed': 'Listed {date}', 'pd.noFunds': 'Not enough balance. Top up at /wallet first.', 'pd.needLogin': 'Sign in to buy.',

    'pg.prev': 'Previous', 'pg.next': 'Next', 'pg.page': 'Page {n}', 'pg.nav': 'Page navigation',

    'docs.kicker': 'Developer Docs', 'docs.h': 'One API key for every model',

    'docs.sub': 'An OpenAI-compatible API. Keep the SDK you already know — just change the base URL.',

    'docs.makeKey': 'Create API key', 'docs.seePrice': 'See pricing', 'docs.step': 'Step {n}',

    'docs.s1': 'Create an API key', 'docs.s1p': 'Sign in, open API Keys, and create a key. It is shown only once — store it safely.', 'docs.s1c': 'Create key',

    'docs.s2': 'Point your SDK', 'docs.s2p': 'Use any OpenAI SDK. Change base_url to the endpoint below and use your key.',

    'docs.s3': 'Send a request', 'docs.s3p': 'Call /chat/completions with the model you want. Token rates follow the pricing page.', 'docs.s3c': 'See pricing',

    'docs.code': 'Code samples', 'docs.lang': 'Choose a sample language', 'docs.sampleCurl': 'cURL', 'docs.samplePython': 'Python', 'docs.sampleJavaScript': 'JavaScript', 'docs.sampleCurlStream': 'cURL stream',

    'docs.keyHint': 'Replace this API key with your own. Base URL:', 'docs.endpoints': 'Endpoints', 'docs.models': 'Available models',
    'docs.copyCode': 'Copy code sample', 'docs.copyFail': 'Could not copy. Select the code and copy it manually.',

    'docs.modelsFail': 'The model list failed to load. Reload the page.', 'docs.noModels': 'No models yet.',

    'docs.e1': 'Send a conversation and get a reply (OpenAI compatible).', 'docs.e2': 'Anthropic-style messages for Claude models.',

    'docs.e3': 'Every available model, plus the rate per 1k tokens.',
    'docs.localLabel': 'Local router (dev only)', 'docs.localWarn': 'This endpoint works ONLY on the machine running the local router, not from a browser or another device.',
    'docs.localHost': 'https://www.node2mint.com/v1', 'docs.localKeyEnv': 'HERMES_CUSTOM_YANSTD_V1_API_KEY',
    'docs.localCurl': 'curl https://www.node2mint.com/v1/models \\\n  -H "Authorization: Bearer $NODE2MINT_KEY"',
    'docs.codex': 'Codex CLI', 'docs.opencode': 'OpenCode', 'docs.claudeCode': 'Claude Code', 'docs.hermes': 'Hermes Agent', 'docs.cursor': 'Cursor',
    'docs.apiKeyAuth': 'Auth: send `Authorization: Bearer *** or `x-api-key: *** to the /v1/* endpoints. Wallet balance is never charged automatically for API calls; each key has its own token quota (credit_limit / credit_used).',
    'docs.quotaHint': 'Check remaining quota with `GET /v1/key` using the key itself as Bearer.',

    'docs.intKick': 'Integration guides', 'docs.intSub': 'Point any LLM client at Node2Mint. The base URL is the public /v1 endpoint; no local router or extra proxy is needed.',
    'docs.intLocal': 'Public API endpoint', 'docs.intLocalWarn': 'Use this canonical public endpoint from any compatible client.',
    'docs.intLocalKey': 'Use a Node2Mint API key from the NODE2MINT_KEY environment variable.',
    'docs.intProd': 'Production endpoint', 'docs.intProdHint': 'Use this canonical URL from any client.',
    'docs.intCodex': 'Codex CLI config (~/.codex/config.toml)',
    'docs.intCodexFile': 'Config file: ~/.codex/config.toml',
    'docs.intOpenCode': 'OpenCode config (opencode.json)',
    'docs.intOpenCodeFile': 'Config file: ~/.opencode.json',
    'docs.intClaude': 'Claude Code env vars', 'docs.intClaudeFile': 'Env vars: ANTHROPIC_BASE_URL + ANTHROPIC_API_KEY',
    'docs.intHermes': 'Hermes Agent', 'docs.intHermesFile': 'Config file: ~/.hermes/config.yaml',
    'docs.intCursor': 'Cursor settings', 'docs.intCursorFile': 'Settings: .cursor/settings.json',
    'docs.troubleshooting': 'Troubleshooting', 'docs.trLocal': 'Connection refused', 'docs.trLocalMsg': 'Confirm the base URL is exactly https://www.node2mint.com/v1 and that your network allows outbound HTTPS on port 443.',
    'docs.tr401': '401 from /v1/*', 'docs.tr401Msg': 'Wrong key, expired key, or the key has no `chat` scope / quota exhausted. Check quota with GET /v1/key.',
    'docs.tr429': '429 Too Many Requests', 'docs.tr429Msg': 'Key-level rate limit hit. Wait and retry; use GET /v1/key to see rate_limit_per_minute.',
    'docs.trModel': 'Model not found', 'docs.trModelMsg': 'Use the exact model id returned by GET /v1/models. Provider-prefixed names (vendor/model) only work at the router that serves them.',
    'docs.copied': 'Copied.', 'docs.copyButton': 'Copy code sample', 'docs.chooseCode': 'Choose a code sample',
    'docs.replaceModel': 'Replace {model} with an id from GET {endpoint}.', 'docs.capabilities': 'Capabilities and limits', 'docs.errors': 'Errors and rate limits',
    'docs.status': 'Status', 'docs.type': 'Type', 'docs.meaning': 'Meaning',
    'docs.endpointChat': 'OpenAI-compatible text chat; supports stream: true.', 'docs.endpointMessages': 'Anthropic-style, text-only messages; streaming is not supported.', 'docs.endpointModels': 'The authenticated model catalog; use an exact returned id.', 'docs.endpointKey': 'The current key’s rate limit, token limit, and credit usage.',
    'docs.err400': 'Malformed request, unavailable model, or unsupported Anthropic streaming.', 'docs.err401': 'Missing or invalid API key.', 'docs.err402': 'Insufficient API token credit or key credit limit reached.', 'docs.err403': 'The key lacks chat scope or the requested model is not allowed.', 'docs.err429': 'Retry after the interval in Retry-After.', 'docs.err5xx': 'An upstream provider could not complete the request.',
    'docs.streaming': 'Streaming', 'docs.streamingP': 'Chat Completions supports server-sent events. Each event is', 'docs.streamingEnd': '; the stream ends with',
    'docs.tools': 'Function calling and tools', 'docs.toolsP': 'Not supported. Do not send', 'docs.toolsEnd': ', or expect tool-call output.',
    'docs.image': 'Image input', 'docs.imageP': 'Not supported. Message content must be text, not multimodal content blocks.',
    'docs.agents': 'Codex and Claude Code', 'docs.agentsP': 'Direct configuration is not documented: their agent workflows require unsupported tool and/or multimodal behavior. Use a text-only OpenAI client instead.',
    'docs.errorsP': 'Programmatic errors use', 'docs.errors429': 'On a 429 response, read', 'docs.errorsRetry': '; retry only after the indicated delay.',

    'st.kicker': 'Public', 'st.h': 'Service Status', 'st.reload': 'Reload', 'st.summary': 'Status summary',

    'st.checking': 'Checking…', 'st.allOk': 'All systems normal', 'st.degraded': 'Partial disruption', 'st.unknown': 'Status unavailable',

    'st.numbers': 'Key numbers', 'st.avg': 'Average availability (24h)', 'st.watched': 'Models watched',

    'st.board': 'Model status', 'st.noBoard': 'No active models in the catalog.', 'st.avail': 'Availability', 'st.avgL': 'Average',

    'st.checks': '{n} checks', 'st.noChecks': 'No checks yet', 'st.last': 'last {date}', 'st.monitors': 'Service monitors',

    'st.noMonitors': 'No monitors reported yet.', 'st.service': 'Service', 'st.unk': 'UNKNOWN', 'st.catalog': 'Available models',

    'st.none': 'No active models.', 'st.ok': 'Normal', 'st.slow': 'Slow', 'st.error': 'Error', 'st.down': 'Down', 'st.nodata': 'No data yet',
    'st.bannerAria': 'Service status notice', 'st.maint': 'Under maintenance', 'st.outage': 'Service outage', 'st.bannerMore': 'View status',
    'tip.avail': 'Share of the last 24 hours of checks that were not down. Dash = never checked.',
    'tip.p90': '90% of the checks in the last 24 hours were faster than this value.',
    'tip.base': 'Public API endpoint used by every model. Keep your SDK, change only this URL.',
    'tip.mult': 'Token multiplier applied to effective API-token usage when a request is settled.',
    'nf.code': '404 · Not found', 'nf.h': 'Page not found', 'nf.p': 'The page you are looking for does not exist or has moved.', 'nf.subject': 'Node2Mint: page not found',
    'nf.home': 'Back to home', 'nf.help': 'Open help center',
    'e5.code': '500 · Server error', 'e5.h': 'Something went wrong', 'e5.p': 'This page could not be loaded. Try again, or go back home. Nothing was charged.',
    'e5.retry': 'Try again', 'e5.home': 'Back to home',

    'auth.kickerIn': '// Account access', 'auth.kickerUp': '// New account',

    'auth.hIn': 'Sign in', 'auth.hUp': 'Create an account', 'auth.noAcc': 'No account yet? ', 'auth.hasAcc': 'Already have an account? ',

    'auth.toUp': 'Sign up', 'auth.toIn': 'Sign in', 'auth.email': 'Email', 'auth.emailInvalid': 'Enter a valid email address.', 'auth.turnstileRequired': 'Complete the security check first.', 'auth.fullName': 'Full name', 'auth.fullNamePh': 'Your full name', 'auth.fullNameParts': 'Enter your full name (at least 2 characters).', 'auth.pass': 'Password',
    'auth.showPass': 'Show password', 'auth.hidePass': 'Hide password', 'auth.passReq': 'Password must contain 8–72 characters.',

    'auth.forgot': 'Forgot password?', 'auth.passIn': 'Your password', 'auth.passUp': '8-72 characters', 'auth.registered': 'Account request processed. Sign in to continue.', 'auth.accountCreated': 'Account created successfully.', 'auth.loginSuccess': 'Login successful.', 'auth.emailRegistered': 'Email is already registered.',
    'forgot.h': 'Reset your password', 'forgot.sub': 'Enter the email attached to your account. If it matches an account, we will send a reset link.', 'forgot.sent': 'If that address is registered, a reset link has been sent. Check your inbox and spam folder.', 'forgot.email': 'Account email', 'forgot.placeholder': 'you@example.com', 'forgot.send': 'Send reset link', 'forgot.sending': 'Sending…', 'forgot.signIn': 'Back to sign in', 'forgot.help': 'Need help instead?', 'forgot.fail': 'Could not request a reset link. Try again later.',
    'reset.title': 'Set a new password', 'reset.sub': 'Use the single-use link from your email. It expires in 15 minutes.', 'reset.new': 'New password', 'reset.newPh': '8–72 characters', 'reset.repeat': 'Confirm new password', 'reset.mismatch': 'The passwords do not match.', 'reset.saving': 'Saving…', 'reset.save': 'Save new password', 'reset.saved': 'Your password was changed. All previous sessions have been signed out.', 'reset.toSignIn': 'Sign in', 'reset.missing': 'This reset link is invalid or incomplete.', 'reset.request': 'Request another reset link', 'reset.invalid': 'This reset link is invalid or has expired.',

    'auth.passConfirm': 'Confirm password', 'auth.passMismatch': 'Password confirmation does not match.', 'auth.refOptional': 'Referral code (optional)',

    'auth.aff': 'Invite code applied: ', 'auth.busy': 'Working…', 'auth.submitIn': 'Sign in', 'auth.submitUp': 'Sign up & continue', 'auth.backHome': 'Back to home',

    'auth.fail': 'Something went wrong. Try again.', 'auth.badCreds': 'Incorrect email or password.', 'auth.or': 'or continue with',
    // 2FA di halaman masuk: tanpa ini akun ber-2FA tidak punya cara login sama sekali.
    'auth.totpLabel': 'Authenticator code', 'auth.totpNeeded': 'Enter the 6-digit code from your authenticator app.',
    'auth.totpWrong': 'Wrong code. Check your authenticator app and try again.',
    'auth.totpHint': 'Open your authenticator app and enter the current 6-digit code.',

    'auth.with': 'Continue with {name}', 'auth.unavailable': 'Not configured', 'auth.cbFail': 'Sign-in did not finish. The link had no token.', 'auth.cbBack': 'Back to sign in',

    'auth.brandTitle': 'Sell and use AI products with one balance.', 'auth.brandSub': 'Digital-product marketplace plus API gateway: atomic balance, hashed API keys, and automatic provider failover.', 'auth.brandFoot': 'Marketplace + API gateway',

    'dash.hello': 'Hi, {name}', 'dash.label': 'Dashboard', 'dash.acct': 'Account summary',

    'dash.loadErr': 'Failed to load account summary. Check the connection and reload the page.', 'dash.noWallet': 'Wallet summary unavailable.',

    'dash.tokens': 'API Tokens Left', 'dash.tokenUnit': 'tokens',

    'dash.tokenExp': 'Deducted on every API call · nearest expiry {date}', 'dash.tokenNone': 'Deducted on every API call · no active pack — buy one in Packs & Pricing.',

    'dash.stale': 'Active period ended — buy a pack to re-activate the tokens you still hold.',

    'dash.tokenOutTitle': 'Token quota empty — requests will be rejected.',

    'dash.usage': 'API Usage', 'dash.reqUnit': 'requests', 'dash.req24': '{n} calls in the last 24 hours',

    'dash.detail': 'Token breakdown', 'dash.in': 'Input', 'dash.cached': 'Cached', 'dash.out': 'Output', 'dash.manageKeys': 'Manage API Keys →',

    'dash.recentModel': 'Recent requests · Model', 'dash.recentTokens': 'In / Out', 'dash.recentWhen': 'When', 'dash.noRecent': 'No requests yet.',

    'dash.wallet': 'Wallet', 'dash.walletP': 'Real money — marketplace spend & token packs.', 'dash.manageWallet': 'Manage wallet →',

    'dash.store': 'My Store', 'dash.products': 'products · {n} orders', 'dash.manageStore': 'Manage store →', 'dash.noStore': 'No store yet.', 'dash.openStore': 'Open a store →',

    'dash.aff': 'Affiliate', 'dash.affStat': '{c} clicks · {v} conversions', 'dash.manageAff': 'Manage affiliate →',

    'dash.orders': 'Recent Orders', 'dash.allOrders': 'All orders →', 'dash.noOrders': 'No orders yet.',

    'adm.mode': 'Admin Mode', 'adm.h': 'Admin Dashboard', 'adm.overview': 'Platform overview', 'adm.loadErr': 'Failed to load the platform overview.',

    'adm.users': 'Users', 'adm.orders': 'Orders', 'adm.products': 'Products', 'adm.revenue': 'Revenue',

    'an.title': 'Analytics', 'an.data': 'Data', 'an.range': 'Time range', 'an.scope': 'Data scope',

    'an.sendiri': 'Personal', 'an.toko': 'Store', 'an.platform': 'Platform',

    'an.noStore': 'Store not registered yet — open a store on the My Store page first.',

    'an.reqs': 'Requests', 'an.tokens': 'Tokens', 'an.tokensIn': 'Input tokens', 'an.tokensOut': 'Output tokens', 'an.aiCost': 'AI cost', 'an.topup': 'Top-up',

    'an.reqsPerDay': 'Requests per day', 'an.topModels': 'Top Models', 'an.noModelUse': 'No model usage yet.',

    'an.revenue': 'Revenue', 'an.views': 'Views', 'an.inCarts': 'In carts', 'an.sold': 'Sold', 'an.conv': 'Conversion', 'an.topProducts': 'Top Products',

    'an.noProducts': 'No products in this store yet.', 'an.units': '{n} units', 'an.viewedSold': '{v} viewed · {s} sold', 'an.cartCount': '{n} in carts',

    'an.signups': 'New signups', 'an.activeUsers': 'Active users', 'an.topupRevenue': 'Top-up revenue', 'an.topupTx': 'Top-up transactions',

    'an.orderGmv': 'Order GMV', 'an.ordersCard': 'Orders', 'an.signupsPerDay': 'Signups per day', 'an.funnel': 'Funnel',

    'an.noData': 'No data.', 'an.chartDaily': 'Daily chart', 'an.modelsAria': 'Top models',

    'an.fDaftar': 'Sign up', 'an.fTopup': 'Top-up', 'an.fPakaiAPI': 'Used API',

  },

  id: {

    'nav.market': 'Pasar Digital', 'nav.about': 'Tentang', 'nav.models': 'Model', 'nav.pricing': 'Harga',

    'nav.blog': 'Berita', 'nav.docs': 'Dokumentasi', 'nav.status': 'Status Layanan', 'nav.faq': 'FAQ',

    'nav.menu': 'Menu', 'nav.home': 'Node2Mint, beranda', 'nav.dashboard': 'Dasbor', 'nav.signin': 'Mulai Gratis',

    'nav.lang': 'Bahasa',

        'about.kicker': 'Tentang Node2Mint', 'about.h': 'Satu akun untuk AI dan perdagangan digital.', 'about.sub': 'Node2Mint menyatukan akses AI multi-model, pemakaian token, dan pasar digital dalam satu pengalaman akun yang jelas.', 'about.pillarsAria': 'Yang disediakan Node2Mint', 'about.apiH': 'API AI multi-model', 'about.apiP': 'Gunakan satu endpoint yang kompatibel dengan OpenAI untuk memakai model yang tersedia sambil memantau pemakaian token dari dasbor.', 'about.marketH': 'Pasar digital', 'about.marketP': 'Temukan dan jual produk digital melalui toko penjual, ulasan, tanda verifikasi, dan alur pesanan yang jelas.', 'about.accountH': 'Satu akun, pembukuan yang jelas', 'about.accountP': 'Dana dompet, kuota token API, hadiah rujukan, dan pendapatan afiliasi tetap terpisah agar setiap saldo memiliki arti yang jelas.', 'about.trustKicker': 'Cara kami bekerja', 'about.trustH': 'Alat yang berguna harus tetap mudah dipahami.', 'about.trustP': 'Kami memilih harga yang eksplisit, aksi yang diverifikasi server, keadaan kosong yang jujur, dan alur produk yang menjelaskan dampak sebelum uang atau akses berubah.', 'about.marketCta': 'Jelajahi pasar digital', 'about.contactCta': 'Hubungi dukungan',

    'foot.market': 'Jelajahi Pasar Digital', 'foot.blog': 'Blog/Berita', 'foot.docs': 'Dokumentasi',
    'foot.status': 'Status Layanan', 'foot.nav': 'Tautan bagian bawah',
    'foot.product': 'Produk', 'foot.learn': 'Pelajari', 'foot.community': 'Komunitas', 'foot.company': 'Perusahaan',
    'foot.build': 'Pengembang', 'foot.earn': 'Penghasilan', 'foot.support': 'Bantuan',
    'foot.communityDiscord': 'Discord', 'foot.communityTelegram': 'Telegram', 'foot.communityX': 'X', 'foot.communityYoutube': 'YouTube',
    'foot.guides': 'Panduan', 'foot.faq': 'FAQ', 'foot.changelog': 'Catatan perubahan',
    'foot.terms': 'Syarat penggunaan', 'foot.privacy': 'Privasi', 'foot.refund': 'Kebijakan pengembalian dana',
    'foot.models': 'Katalog model', 'foot.packs': 'Paket token', 'foot.pricing': 'Harga',
    'foot.keys': 'Kunci API', 'foot.playground': 'Ruang uji',
    'foot.sell': 'Berjualan di Node2Mint', 'foot.affiliate': 'Afiliasi', 'foot.referral': 'Rujukan',
    'foot.help': 'Pusat bantuan', 'foot.articles': 'Artikel',
    'foot.about': 'Satu akun untuk model AI, pemakaian token API, pasar digital, komisi afiliasi, dan hadiah token rujukan.',
    'privacy.cookieTitle': 'Privasi Anda penting', 'privacy.cookieBody': 'Kami memakai cookie esensial agar situs berfungsi dan mengingat preferensi Anda.', 'privacy.cookieAccept': 'Setuju',
    'foot.copyright': '© 2026 Node2Mint', 'foot.signature': 'API AI dan pasar digital', 'foot.credit': '· Dibuat dengan', 'foot.creditAria': 'Dibuat dengan cinta oleh YanSTD',
    'foot.openDocs': 'Buka dokumentasi', 'foot.openStatus': 'Buka status layanan', 'foot.openMarket': 'Buka pasar digital',

    'hero.kicker': 'Pasar digital + token API AI', 'hero.h1': 'Pasar Digital & Token API AI, Bersama',

    'hero.sub': 'Jual-beli produk digital, akses banyak model AI, serta kelola dompet dan pemakaian token dari satu akun.',

    'hero.cta': 'Buat akun gratis', 'hero.ctaIn': 'Buka dasbor', 'hero.explore': 'Lihat model AI', 'hero.seeAll': 'Lihat semua {n} model', 'hero.marketCta': 'Buka pasar digital', 'hero.apiCta': 'Gunakan API AI',

    'hero.card': 'Katalog model langsung', 'hero.live': 'Terhubung', 'hero.statModels': 'model tersedia', 'hero.statBase': 'Endpoint kompatibel OpenAI', 'hero.marketCard': 'Pasar digital', 'hero.apiCard': 'Token API AI', 'hero.walletCode': 'Dompet pasar digital + /v1/chat/completions', 'market.banner': 'Spanduk pasar digital', 'market.bannerNum': 'Spanduk pasar digital {n}', 'market.prev': 'Spanduk sebelumnya', 'market.next': 'Spanduk berikutnya', 'market.open': 'Buka spanduk {n}',

    'home.ledgerKicker': 'Penagihan yang jelas', 'home.ledgerH': 'Uang dan kuota API tetap terpisah.',

    'home.ledgerP': 'Dompet menyimpan uang nyata. Kuota token membayar panggilan API. Dasbor menampilkan keduanya tanpa mencampur pembukuan.',

    'home.walletH': 'Dompet', 'home.walletP': 'Isi saldo, beli produk pasar digital, terima pendapatan afiliasi, dan tarik dana yang tersedia.',

    'home.tokensH': 'Token API', 'home.tokensP': 'Beli paket token, pakai di semua model tersedia, dan pantau pemakaian per kunci API.',

    'home.cacheH': 'Input cache lebih murah', 'home.cacheP': 'Konteks berulang dibaca dari pemakaian penyedia dan ditagih dengan tarif token cache yang lebih rendah.',

    'home.earnKicker': 'Kemitraan', 'home.earnH': 'Bagikan produk atau undang pengguna baru. Hadiahnya berbeda.',

    'home.earnP': 'Afiliasi dan rujukan adalah program terpisah dengan pembukuan hadiah yang berbeda.',

    'home.affH': 'Afiliasi', 'home.affP': 'Buat tautan untuk produk pasar digital. Pembelian yang berasal dari tautan Anda menghasilkan pendapatan di dompet.',

    'home.affReward': 'Hadiah: saldo uang nyata di dompet', 'home.affCta': 'Buka alat afiliasi',

    'home.refH': 'Rujukan', 'home.refP': 'Undang seseorang untuk membuat akun. Saat isi saldo pertamanya selesai, kedua akun menerima paket token API.',

    'home.refReward': 'Hadiah: paket token API', 'home.refCta': 'Buka program rujukan',

    'home.finalH': 'Permintaan pertama Anda dimulai dari satu akun.',

    'home.finalP': 'Buat akun, pilih paket token, lalu panggil model pertama lewat satu endpoint.',

    'home.finalCta': 'Buat akun gratis', 'home.finalIn': 'Buka dasbor', 'home.finalDocs': 'Baca dokumentasi API',

    'models.kicker': 'Katalog model', 'models.h': 'Pilih modelnya. API-nya tetap sama.',

    'models.sub': 'Nama model, jendela konteks, dan status kesehatan berasal dari katalog langsung.', 'models.base': 'Base URL',

    'models.copy': 'Salin base URL', 'models.copied': 'Tersalin.', 'models.available': '{n} model tersedia', 'models.context': 'Konteks {n}',

    'models.healthy': 'Normal', 'models.unknown': 'Belum ada data status', 'models.all': 'Lihat dokumentasi API',

    'models.other': 'Lainnya', 'models.freeTag': 'Gratis', 'models.paidTag': 'Berbayar', 'models.copyId': 'Salin ID model', 'models.joinToUnlock': 'JOIN TO UNLOCK',
    'models.freeTerms': 'Perlu masuk · {n} token API/hari · selebihnya memakai saldo paket token.',
    'models.paidTerms': 'Memakai token API ×{n}; perlu paket token yang aktif.',

    'models.usage': 'Penggunaan token · pengali', 'models.docs': 'Buka dokumentasi API',

    'models.showAll': 'Lihat semua {n} model', 'models.showLess': 'Tampilkan lebih sedikit',

    'pack.kicker': 'Paket token', 'pack.h': 'Bayar pemakaian, bukan langganan baru.',

    'pack.sub': 'Satu saldo token berlaku di semua model tersedia. Input cache ditagih dengan tarif lebih rendah.',

    'pack.popular': 'Pilihan seimbang', 'pack.active': 'Aktif {n} hari', 'pack.allModels': 'Berlaku untuk semua model tersedia', 'pack.cta': 'Bandingkan paket',

    'faq.kicker': 'FAQ', 'faq.h': 'Pertanyaan, terjawab.',

    'faq.sub': 'Jawaban ringkas tentang akses API, pemakaian token, saldo dompet, dan pasar digital.',

    'faq.docs': 'Baca dokumentasi', 'faq.status': 'Status layanan',

    'faq.q1': 'Apakah saldo dompet dan token API itu sama?',

    'faq.a1': 'Tidak. Saldo dompet adalah uang untuk isi saldo, belanja pasar digital, pendapatan penjual, dan penarikan. Token API adalah kuota yang dipakai oleh permintaan model.',

    'faq.q2': 'Apakah API kompatibel dengan OpenAI?',

    'faq.a2': 'Ya. Tetap gunakan SDK OpenAI dan format chat completions, lalu ganti base URL, kunci API, dan nama model publik.',

    'faq.q3': 'Bagaimana pemakaian model ditagih?',

    'faq.a3': 'Setiap permintaan memotong token API dari input efektif dan output. Input cache yang dilaporkan penyedia mendapat diskon cache platform; biaya tambahan pemakaian dan pengali model diterapkan sebelum penyelesaian.',

    'faq.q4': 'Apa bedanya afiliasi dan rujukan?',

    'faq.a4': 'Komisi afiliasi berasal dari pembelian produk pasar digital dan dibayarkan ke dompet. Hadiah rujukan menambahkan token API setelah pengguna yang diundang menyelesaikan isi saldo pertama yang memenuhi syarat.',

    'faq.q5': 'Apa yang bisa dibeli atau dijual di pasar digital?',

    'faq.a5': 'Pasar digital mendukung produk digital seperti prompt, dataset, alat, template, dan layanan digital. Pembeli membayar dari dompet, sedangkan penjual mengatur stok dan pengiriman dari tokonya.',

    'faq.q6': 'Apa yang terjadi ketika token API habis?',

    'faq.a6': 'Permintaan API baru akan ditolak sampai paket token lain diaktifkan atau voucher yang valid ditukar. Saldo dompet tidak dipotong otomatis untuk permintaan API.',

    'faqPage.h': 'Pertanyaan umum', 'faqPage.q1': 'Apa bedanya dompet dan token?', 'faqPage.a1': 'Dompet menyimpan saldo rupiah untuk transaksi. Token adalah kuota untuk panggilan API.',
    'faqPage.q2': 'Kapan penjual menerima dana?', 'faqPage.a2': 'Saat pembeli mengonfirmasi pesanan, atau otomatis 2×24 jam setelah penjual menandai pesanan terkirim, kecuali ada sengketa terbuka.',
    'faqPage.q3': 'Bagaimana rujukan bekerja?', 'faqPage.a3': 'Rujukan yang lolos pemeriksaan IP lokal mendapat 2,5 juta token dan satu hari masa aktif untuk pemberi dan pengguna baru.',
    'faqPage.q4': 'Bagaimana cara mengisi dompet dan membeli paket token?', 'faqPage.a4': 'Buka halaman Dompet dan pilih metode pembayaran. Setelah pembayaran selesai, saldo masuk ke dompet; membeli paket token menambahkan token dan masa aktifnya ke akun.',
    'faqPage.q5': 'Apa yang bisa dibeli atau dijual di pasar digital?', 'faqPage.a5': 'Produk digital seperti prompt, dataset, alat, template, dan layanan digital. Pembeli membayar dari dompet; penjual mengatur stok, pengiriman, dan sengketa dari tokonya.',
    'faqPage.q6': 'Apa yang terjadi saat token API habis?', 'faqPage.a6': 'Permintaan API baru ditolak sampai paket token lain diaktifkan atau kode penukaran yang valid dipakai. Saldo dompet tidak pernah dipotong otomatis untuk permintaan API.',
    'faqPage.q7': 'Cara menjaga akun tetap aman?', 'faqPage.a7': 'Aktifkan autentikasi dua faktor (2FA) di tab Keamanan, jaga kunci API tetap rahasia (kunci hanya ditampilkan sekali dan disimpan sebagai ringkasan kriptografis), serta akhiri sesi yang tidak dikenali dari tab yang sama.',
    'faqPage.q8': 'Apa bedanya afiliasi dan rujukan?', 'faqPage.a8': 'Afiliasi memberi komisi uang nyata ke dompet dari penjualan produk pasar digital yang berasal dari tautan Anda. Rujukan menambahkan token API setelah orang yang Anda undang menyelesaikan isi saldo pertama yang memenuhi syarat.',
    'faqPage.q9': 'Cara membuka sengketa dan kapan bisa mendapat pengembalian dana?', 'faqPage.a9': 'Buka sengketa dari pesanan saat status DIBAYAR atau DIPROSES, atau paling lambat 7 hari setelah pesanan ditandai selesai. Pengembalian dana dibayar dari rekening penampungan pesanan saat dana masih tertahan; jika dana penjual sudah dilepas, kasus masuk peninjauan manual.',
    'faqPage.q10': 'Bisa mendapat pengembalian dana untuk paket token?', 'faqPage.a10': 'Token yang sudah terpakai tidak pernah dikembalikan. Jika paket yang dibeli masih tidak terpakai, ajukan peninjauan melalui pusat bantuan; paket yang sudah diaktifkan tetap berlaku sampai masa aktifnya berakhir.',
    'faqPage.q11': 'Node2Mint mengenakan biaya penarikan?', 'faqPage.a11': 'Biaya layanan 5% dikenakan pada penarikan dompet. Besar biaya dan jumlah yang Anda terima ditampilkan sebelum konfirmasi.',
    'faqPage.q12': 'Apa konten yang dilarang di pasar digital?', 'faqPage.a12': 'Produk atau artikel yang ilegal, berbahaya, atau melanggar hak pihak lain. Platform dapat menyembunyikan atau menghapus penawaran dan membatasi akun yang melanggar.',
    'faqPage.help': 'Belum menemukan jawaban?', 'faqPage.helpLink': 'Buka pusat bantuan',
    'faqPage.support': 'Anda juga bisa menghubungi kami lewat halaman kontak.',

    'sup.kicker': 'Bantuan', 'sup.h': 'Hubungi bantuan',
    'sup.sub': 'Satu tempat untuk isi saldo tertunda, penarikan, pengembalian dana, sengketa pasar digital, dan akun yang terkunci.',
    'sup.emailH': 'Email bantuan', 'sup.subject': 'Permintaan dukungan Node2Mint',
    'sup.emailP': 'Kirim dari email akun yang terdaftar ke {email} supaya akunmu bisa dicocokkan.',
    'sup.emailCta': 'Buka draf email',
    'sup.copyEmail': 'Salin alamat email', 'sup.copied': 'Alamat email disalin.',
    'sup.hoursH': 'Waktu respons',
    'sup.hoursP': 'Balasan pertama dalam 3–5 hari kerja (Sen–Jum, WIB). Sertakan ID pesanan atau referensi isi saldo agar peninjauan lebih cepat.',
    'sup.needH': 'Sertakan ini',
    'sup.need1': 'Email akun yang terdaftar',
    'sup.need2': 'ID pesanan, ID penarikan, atau referensi isi saldo',
    'sup.need3': 'Nominal, tanggal, dan tujuan (bank/dompet elektronik/jaringan)',
    'sup.need4': 'Tangkapan layar atau ringkasan kriptografis transaksi bila relevan',
    'sup.escalateH': 'Perpindahan dana',
    'sup.escalateP': 'Isi saldo dan penarikan menunggu verifikasi admin; dana pasar digital ditahan di rekening penampungan. Naikkan ke alamat yang sama bila permintaan dana belum dibalas dalam 3–5 hari kerja (Sen–Jum, WIB) dan sertakan referensinya.',
    'sup.resetH': 'Tidak bisa masuk akun?',
    'sup.resetP': 'Minta tautan pengaturan ulang mandiri dari email yang terhubung ke akunmu.',
    'sup.resetBtn': 'Minta pengaturan ulang kata sandi',
    'sup.faq': 'Baca FAQ', 'sup.docs': 'Buka dokumentasi', 'sup.status': 'Status layanan',

    'docs.examplePrompt': 'Halo!', 'price.signupGiven': 'Sudah diberikan saat pendaftaran',

    'show.kicker': 'Pasar digital', 'show.h': 'Jual aset AI yang benar-benar berguna.',

    'show.sub': 'Pasang prompt, dataset, alat, template, atau layanan digital. Pembeli membayar dari dompet dan menerima pengiriman digital.',

    'show.all': 'Lihat katalog', 'show.out': 'Stok habis', 'show.new': 'Terbaru', 'show.instant': 'Pengiriman digital', 'show.stock': 'Stok {n}', 'show.buy': 'Lihat produk', 'show.featured': 'Pilihan pasar digital', 'show.best': 'Produk terlaris', 'show.newest': 'Produk terbaru', 'show.sales': '{n} terjual', 'show.reviews': '{n} ulasan',

    'articles.kicker': 'Panduan dan pembaruan', 'articles.h': 'Pahami produk sebelum mulai.', 'articles.all': 'Baca semua artikel',

    'articles.empty': 'Belum ada artikel yang terbit. Kunjungi lagi nanti.', 'articles.official': 'Resmi', 'articles.community': 'Komunitas',

    'ticker': 'Penyedia model tersedia',

    'err.load': 'Gagal memuat data. Periksa koneksi lalu muat ulang halaman.',

    'price.kicker': 'Paket token', 'price.h': 'Satu saldo token untuk semua model',

    'price.sub': 'Pilih paket token sesuai kebutuhan. Satu saldo aktif dapat dipakai untuk semua model yang tersedia.', 'price.cacheH': 'Konteks cache lebih hemat', 'price.cacheTip': 'Jika penyedia melaporkan konteks berulang sebagai input cache, pemakaiannya lebih kecil daripada input biasa.',
    'price.currencyBasis': 'Harga Indonesia memakai IDR; bahasa lain memakai USD dengan kurs tetap 1 USD = Rp{n}. Pembukuan penyelesaian tetap IDR.',
    'price.billingFormula': 'Setiap permintaan memakai saldo token berdasarkan input dan output model. Konteks berulang dapat memakai token lebih sedikit jika penyedia melaporkannya sebagai cache; pengali penggunaan tiap model terlihat pada kartunya.',
    'price.signupTerms': 'Diberikan sekali saat pendaftaran. Jumlah token dan masa aktif yang tertera di atas berlaku.',

    'price.currency': 'Pilih mata uang', 'price.rate': 'Kurs 1 USD = Rp{n}',

    'price.empty': 'Belum ada paket tersedia.', 'price.popular': 'Paling populer', 'price.tokens': '{n} token',
    'price.tokensBonus': '{base} token + bonus {bonus} (total {total})',
    'price.freeLimits': 'Akses gratis dari pendaftaran/rujukan: 20 permintaan/menit dan 250.000 token/menit.',
    'price.paidLimits': 'Paket aktif Mini sampai Pro Max: 55 permintaan/menit dan 1.000.000 token/menit.',

    'price.days': 'Aktif {n} hari', 'price.allModels': 'Semua model', 'price.buy': 'Beli paket', 'price.buying': 'Memproses…',
    'price.daily': 'Harian', 'price.monthly': 'Bulanan',

    'price.bought': 'Paket "{name}" aktif. Kuota token sudah ditambahkan.', 'price.buyFail': 'Pembelian paket gagal. Coba lagi.',

    'price.models': 'Katalog Model', 'price.modelsSub': 'Semua model memakai endpoint yang sama. Pemakaian ditagih per token, dengan pengali di tiap kartu.',

    'price.noModels': 'Belum ada model terdaftar.',

    'th.model': 'Model', 'th.tier': 'Tingkat', 'th.provider': 'Penyedia', 'th.in': 'Masukan / 1k', 'th.out': 'Keluaran / 1k',

    'th.ctx': 'Konteks', 'th.type': 'Tipe', 'th.ctxTokens': '{n} token', 'fam.other': 'Lainnya',

    'price.faq': 'Pertanyaan Umum',

    'pf.q1': 'Bagaimana cara membeli paket token?', 'pf.a1': 'Pilih paket, lanjutkan ke pembayaran, lalu selesaikan metode yang dipilih. Paket aktif setelah pembayaran dikonfirmasi.',

    'pf.q2': 'Apa bedanya saldo dompet dan token API?', 'pf.a2': 'Saldo dompet adalah uang untuk transaksi platform. Token API adalah kuota permintaan model yang berasal dari paket token aktif atau kode penukaran yang valid.',

    'pf.q3': 'Berapa batas akses API?', 'pf.a3': 'Akses gratis dari pendaftaran atau rujukan memiliki 20 permintaan per menit dan 250.000 token per menit. Paket Mini sampai Pro Max yang aktif memiliki 55 permintaan per menit dan 1.000.000 token per menit.',

    'pf.q4': 'Apakah satu paket berlaku untuk semua model?', 'pf.a4': 'Ya. Satu saldo token aktif dipakai bersama oleh semua model tersedia. Pemakaian tiap model dapat berbeda sesuai pengali yang tampil pada kartunya.',

    'pf.q5': 'Apa yang terjadi jika token atau masa aktif habis?', 'pf.a5': 'Permintaan API baru ditolak sampai paket lain diaktifkan atau kode penukaran yang valid digunakan. Saldo dompet tidak dipotong otomatis untuk pemakaian API.',

    'art.kicker': 'Konten', 'art.h': 'Artikel', 'art.write': 'Tulis', 'art.writeLong': 'Tulis Artikel',

    'art.empty': 'Belum ada artikel', 'art.emptyP': 'Artikel yang ditayangkan akan muncul di sini.', 'art.team': 'Tim', 'art.community': 'Komunitas',

    'art.more': 'Baca selengkapnya', 'art.back': 'Kembali ke daftar artikel', 'art.fail': 'Artikel tidak bisa dimuat', 'art.emptyBody': 'Isi artikel kosong.',

    'art.all': 'Semua artikel',

    'chat.start': 'Mau mulai dari mana?', 'chat.placeholder': 'Tulis pesan…',

    'chat.model': 'Model',

    'chat.loadingModels': 'Memuat model…', 'chat.noModels': 'Tidak ada model tersedia',

    'chat.user': 'Kamu', 'chat.assistant': 'Asisten',

    'chat.cost': 'Biaya {n}', 'chat.writing': 'Menulis balasan…',

    'mkt.kicker': 'Belanja', 'mkt.h': 'Pasar Digital', 'mkt.sellCta': 'Mulai berjualan', 'mkt.empty': 'Belum ada produk', 'mkt.emptyP': 'Produk baru akan muncul di sini begitu tersedia.', 'mkt.emptyNoDemoP': 'Belum ada penawaran aktif. Kami tidak menampilkan contoh produk fiktif; produk akan muncul setelah penjual menerbitkannya.',

    'mkt.noDesc': 'Tanpa deskripsi.', 'mkt.inStock': '{n} tersedia', 'mkt.soldOut': 'Habis', 'mkt.demo': 'Demo · tidak dijual', 'mkt.demoNote': 'Penawaran demo ini hanya untuk dilihat dan tidak dapat dibeli atau dimasukkan ke keranjang.', 'mkt.reviews': '{n} ulasan', 'mkt.reviewsTitle': 'Ulasan pembeli', 'mkt.noReviews': 'Belum ada ulasan.', 'mkt.buyer': 'Pembeli',

    'mkt.cart': 'Keranjang', 'mkt.carting': 'Menambahkan…', 'mkt.review': 'Ulas', 'mkt.buy': 'Beli Sekarang', 'mkt.buying': 'Memproses…', 'mkt.out': 'Stok Habis',

    'mkt.bought': 'Pembelian berhasil. Saldo kamu sudah dipotong otomatis.', 'mkt.noFunds': 'Saldo tidak cukup. Isi ulang dulu di dasbor.',

    'mkt.buyFail': 'Pembelian gagal. Coba lagi.', 'mkt.added': 'Ditambahkan ke keranjang.', 'mkt.noCart': 'Produk ini tidak bisa masuk keranjang.',

    'mkt.cartFail': 'Gagal menambah ke keranjang. Coba lagi.', 'mkt.reviewed': 'Ulasan terkirim. Terima kasih!', 'mkt.reviewFail': 'Gagal mengirim ulasan. Coba lagi.',
    'mkt.wishlistAdd': 'Simpan ke favorit', 'mkt.wishlistRemove': 'Hapus dari favorit', 'mkt.wishlistSaved': 'Disimpan ke favorit.', 'mkt.wishlistRemoved': 'Dihapus dari favorit.', 'mkt.wishlistFail': 'Gagal memperbarui favorit. Coba lagi.',

    'mkt.products': 'produk', 'mkt.orders': 'pesanan', 'pd.qty': 'Jml', 'pd.inc': 'Tambah jumlah', 'pd.dec': 'Kurangi jumlah',

    'mkt.loadFail': 'Gagal memuat produk. Periksa koneksi lalu muat ulang halaman.',

    'mkt.reviewNote': 'Ulasan hanya bisa dikirim untuk produk yang pernah kamu beli.', 'mkt.rating': 'Penilaian', 'mkt.stars': '{n} bintang',

    'mkt.comment': 'Komentar (opsional)', 'mkt.commentPh': 'Bagaimana produk ini menurutmu?', 'mkt.cancel': 'Batal',

    'mkt.send': 'Kirim Ulasan', 'mkt.sending': 'Mengirim…', 'mkt.reviewOf': 'Ulas {name}',

    'mkt.search': 'Cari produk', 'mkt.searchPh': 'Cari judul atau deskripsi', 'mkt.cats': 'Kategori', 'mkt.catPremiumsApp': 'Aplikasi premium', 'mkt.catSoulSkill': 'Jiwa & keahlian', 'mkt.catToolsSoftware': 'Alat & perangkat lunak', 'mkt.catEbook': 'Buku digital', 'mkt.catAccounts': 'Akun', 'mkt.catOthers': 'Lainnya',

    'mkt.moreFromStore': 'Produk lain dari toko ini', 'mkt.moreFromAll': 'Produk lain dari semua toko',

    'mkt.all': 'Semua', 'mkt.forYou': 'Untuk kamu', 'mkt.verified': 'Terverifikasi', 'mkt.reset': 'Atur ulang penyaring', 'mkt.filter': 'Saring & urutkan', 'mkt.sort': 'Urutkan berdasarkan', 'mkt.sortNewest': 'Terbaru', 'mkt.sortBest': 'Produk terlaris', 'mkt.sortLow': 'Harga: rendah ke tinggi', 'mkt.sortHigh': 'Harga: tinggi ke rendah', 'mkt.priceMin': 'Harga minimum', 'mkt.priceMax': 'Harga maksimum', 'mkt.instantOnly': 'Pengiriman instan', 'mkt.availableOnly': 'Masih tersedia', 'mkt.apply': 'Terapkan', 'mkt.close': 'Tutup', 'mkt.noMatch': 'Produk tidak ditemukan', 'mkt.noMatchP': 'Coba kata kunci atau kategori lain.',
    'mkt.infoTitle': 'Info pasar digital', 'mkt.infoVerifiedH': 'Verifikasi toko dan moderasi', 'mkt.infoVerifiedP': 'Lencana Terverifikasi berarti toko sudah melewati proses verifikasi platform. Lencana ini bukan jaminan bahwa semua produk cocok atau berkualitas. Penawaran dapat disembunyikan atau dihapus jika melanggar aturan pasar digital.', 'mkt.infoFeeH': 'Biaya pasar digital', 'mkt.infoFeeP': 'Rincian pesanan memisahkan harga produk, biaya platform, dan bagian penjual. Biaya yang berlaku dipotong saat penyelesaian pesanan, sehingga pembeli dapat melihat harga produk sebelum menyelesaikan pembayaran.', 'mkt.infoPayoutH': 'Saldo penjual dan penarikan', 'mkt.infoPayoutP': 'Pendapatan penjual tetap berada di saldo toko sampai pesanan selesai. Setelah dana tersedia, penjual dapat mengajukan penarikan; biaya yang berlaku dan jumlah bersih ditampilkan sebelum konfirmasi.',

    'pd.missing': 'Produk tidak ditemukan', 'pd.left': 'Stok tersisa {n}', 'pd.out': 'Habis',

    'pd.listed': 'Dipasang {date}', 'pd.noFunds': 'Saldo tidak cukup. Isi ulang dulu di halaman dompet.', 'pd.needLogin': 'Masuk dulu untuk membeli.',

    'pg.prev': 'Sebelumnya', 'pg.next': 'Berikutnya', 'pg.page': 'Halaman {n}', 'pg.nav': 'Navigasi halaman',

    'docs.kicker': 'Dokumentasi Pengembang', 'docs.h': 'Satu kunci API untuk semua model',

    'docs.sub': 'API kompatibel OpenAI. Pakai SDK yang sudah kamu kenal, cukup ganti base URL.',

    'docs.makeKey': 'Buat Kunci API', 'docs.seePrice': 'Lihat Harga', 'docs.step': 'Langkah {n}',

    'docs.s1': 'Buat Kunci API', 'docs.s1p': 'Daftar masuk, buka halaman Kunci API, buat kunci baru. Kunci hanya ditampilkan sekali, simpan baik-baik.', 'docs.s1c': 'Buat Kunci',

    'docs.s2': 'Arahkan SDK', 'docs.s2p': 'Pakai SDK OpenAI apa pun, cukup ganti base_url ke endpoint di bawah dan pakai kunci kamu.',

    'docs.s3': 'Kirim Permintaan', 'docs.s3p': 'Panggil /chat/completions dengan model pilihan. Tarif token mengikuti tabel di halaman harga.', 'docs.s3c': 'Lihat Harga',

    'docs.code': 'Contoh Kode', 'docs.lang': 'Pilih bahasa contoh', 'docs.sampleCurl': 'cURL', 'docs.samplePython': 'Python', 'docs.sampleJavaScript': 'JavaScript', 'docs.sampleCurlStream': 'cURL aliran data',

    'docs.keyHint': 'Ganti kunci API ini dengan milikmu. Base URL:', 'docs.endpoints': 'Endpoint', 'docs.models': 'Model Tersedia',
    'docs.copyCode': 'Salin contoh kode', 'docs.copyFail': 'Gagal menyalin. Pilih kodenya dan salin manual.',

    'docs.modelsFail': 'Daftar model gagal dimuat. Coba muat ulang halaman.', 'docs.noModels': 'Belum ada model.',

    'docs.e1': 'Kirim percakapan, dapat balasan (kompatibel OpenAI).', 'docs.e2': 'Format pesan gaya Anthropic untuk model claude.',

    'docs.e3': 'Daftar semua model yang tersedia + tarif per 1k token.',
    'docs.localLabel': 'Router lokal (hanya pengembangan)', 'docs.localWarn': 'Endpoint ini hanya bisa dipakai di komputer yang menjalankan router lokal, bukan dari browser atau perangkat lain.',
    'docs.localHost': 'https://www.node2mint.com/v1', 'docs.localKeyEnv': 'HERMES_CUSTOM_YANSTD_V1_API_KEY',
    'docs.localCurl': 'curl https://www.node2mint.com/v1/models \\\n  -H "Authorization: Bearer $NODE2MINT_KEY"',
    'docs.codex': 'Codex CLI', 'docs.opencode': 'OpenCode', 'docs.claudeCode': 'Claude Code', 'docs.hermes': 'Hermes Agent', 'docs.cursor': 'Cursor',
    'docs.apiKeyAuth': 'Autentikasi: kirim `Authorization: Bearer ***` atau `x-api-key: ***` ke endpoint /v1/*. Saldo dompet tidak dipotong otomatis untuk panggilan API; setiap kunci memiliki kuota token sendiri (credit_limit / credit_used).',
    'docs.quotaHint': 'Periksa sisa kuota dengan `GET /v1/key` menggunakan kunci itu sendiri sebagai Bearer.',
    'docs.intKick': 'Panduan Integrasi', 'docs.intSub': 'Arahkan klien LLM apa pun ke Node2Mint. Base URL adalah endpoint /v1 publik; tidak perlu router lokal atau proxy tambahan.',
    'docs.intLocal': 'Endpoint API publik', 'docs.intLocalWarn': 'Gunakan endpoint publik kanonis ini dari klien yang kompatibel.',
    'docs.intLocalKey': 'Gunakan kunci API Node2Mint dari variabel lingkungan NODE2MINT_KEY.',
    'docs.intProd': 'Endpoint produksi', 'docs.intProdHint': 'Pakai URL kanonis ini dari klien mana pun.',
    'docs.intCodex': 'Konfigurasi Codex CLI (~/.codex/config.toml)',
    'docs.intCodexFile': 'File konfig: ~/.codex/config.toml',
    'docs.intOpenCode': 'Konfigurasi OpenCode (opencode.json)',
    'docs.intOpenCodeFile': 'File konfig: ~/.opencode.json',
    'docs.intClaude': 'Variabel lingkungan Claude Code', 'docs.intClaudeFile': 'Variabel lingkungan: ANTHROPIC_BASE_URL + ANTHROPIC_API_KEY',
    'docs.intHermes': 'Hermes Agent', 'docs.intHermesFile': 'Berkas konfigurasi: ~/.hermes/config.yaml',
    'docs.intCursor': 'Pengaturan Cursor', 'docs.intCursorFile': 'Pengaturan: .cursor/settings.json',
    'docs.troubleshooting': 'Pemecahan Masalah', 'docs.trLocal': 'Koneksi ditolak', 'docs.trLocalMsg': 'Pastikan base URL tepat https://www.node2mint.com/v1 dan jaringanmu mengizinkan HTTPS keluar pada port 443.',
    'docs.tr401': '401 dari /v1/*', 'docs.tr401Msg': 'Kunci salah, kedaluwarsa, atau tidak punya scope `chat` / kuota habis. Cek kuota dengan GET /v1/key.',
    'docs.tr429': '429 Terlalu Banyak Permintaan', 'docs.tr429Msg': 'Batas laju kunci tercapai. Tunggu dan coba lagi; lihat rate_limit_per_minute di GET /v1/key.',
    'docs.trModel': 'Model tidak ditemukan', 'docs.trModelMsg': 'Pakai model id persis seperti di GET /v1/models. Nama bertprefiks (vendor/model) hanya bisa dipakai di router yang melayani.',
    'docs.copied': 'Tersalin.', 'docs.copyButton': 'Salin contoh kode', 'docs.chooseCode': 'Pilih contoh kode',
    'docs.replaceModel': 'Ganti {model} dengan ID dari GET {endpoint}.', 'docs.capabilities': 'Kemampuan dan batasan', 'docs.errors': 'Galat dan batas laju',
    'docs.status': 'Status', 'docs.type': 'Tipe', 'docs.meaning': 'Arti',
    'docs.endpointChat': 'Obrolan teks kompatibel OpenAI; mendukung stream: true.', 'docs.endpointMessages': 'Pesan teks bergaya Anthropic; aliran data tidak didukung.', 'docs.endpointModels': 'Katalog model terautentikasi; gunakan ID yang dikembalikan secara persis.', 'docs.endpointKey': 'Batas laju, batas token, dan pemakaian kredit kunci saat ini.',
    'docs.err400': 'Permintaan tidak valid, model tidak tersedia, atau aliran data Anthropic tidak didukung.', 'docs.err401': 'Kunci API tidak ada atau tidak valid.', 'docs.err402': 'Kredit token API tidak cukup atau batas kredit kunci tercapai.', 'docs.err403': 'Kunci tidak memiliki scope chat atau model yang diminta tidak diizinkan.', 'docs.err429': 'Coba lagi setelah jeda pada Retry-After.', 'docs.err5xx': 'Penyedia sumber tidak dapat menyelesaikan permintaan.',
    'docs.streaming': 'Aliran data', 'docs.streamingP': 'Chat Completions mendukung peristiwa yang dikirim server. Setiap peristiwa berbentuk', 'docs.streamingEnd': '; aliran berakhir dengan',
    'docs.tools': 'Pemanggilan fungsi dan alat', 'docs.toolsP': 'Tidak didukung. Jangan kirim', 'docs.toolsEnd': ', dan jangan mengharapkan output pemanggilan alat.',
    'docs.image': 'Masukan gambar', 'docs.imageP': 'Tidak didukung. Isi pesan harus berupa teks, bukan blok konten multimodal.',
    'docs.agents': 'Codex dan Claude Code', 'docs.agentsP': 'Konfigurasi langsung tidak didokumentasikan: alur agen tersebut membutuhkan alat dan/atau perilaku multimodal yang tidak didukung. Gunakan klien OpenAI teks saja.',
    'docs.errorsP': 'Galat terprogram memakai', 'docs.errors429': 'Pada respons 429, baca', 'docs.errorsRetry': '; coba lagi hanya setelah jeda yang ditunjukkan.',

    'st.kicker': 'Publik', 'st.h': 'Status Layanan', 'st.reload': 'Muat ulang', 'st.summary': 'Ringkasan status',

    'st.checking': 'Memeriksa…', 'st.allOk': 'Semua sistem normal', 'st.degraded': 'Ada gangguan sebagian', 'st.unknown': 'Status tidak tersedia',

    'st.numbers': 'Ringkasan angka', 'st.avg': 'Ketersediaan rata-rata (24 jam)', 'st.watched': 'Model dipantau',

    'st.board': 'Status Model', 'st.noBoard': 'Belum ada model aktif di katalog.', 'st.avail': 'Ketersediaan', 'st.avgL': 'Rata-rata',

    'st.checks': '{n} pemeriksaan', 'st.noChecks': 'Belum ada pemeriksaan', 'st.last': 'terakhir {date}', 'st.monitors': 'Pemantau layanan',

    'st.noMonitors': 'Belum ada monitor yang dilaporkan.', 'st.service': 'Layanan', 'st.unk': 'TIDAK DIKETAHUI', 'st.catalog': 'Model Tersedia',

    'st.none': 'Tidak ada model aktif.', 'st.ok': 'Normal', 'st.slow': 'Melambat', 'st.error': 'Galat', 'st.down': 'Gangguan', 'st.nodata': 'Belum ada data',
    'st.bannerAria': 'Pemberitahuan status layanan', 'st.maint': 'Sedang pemeliharaan', 'st.outage': 'Gangguan layanan', 'st.bannerMore': 'Lihat status',
    'tip.avail': 'Persentase pemeriksaan 24 jam terakhir yang tidak mengalami gangguan. Tanda “—” berarti belum pernah diperiksa.',
    'tip.p90': '90% pemeriksaan dalam 24 jam terakhir lebih cepat dari nilai ini.',
    'tip.base': 'Endpoint API publik yang dipakai semua model. SDK tetap, cukup ganti URL ini.',
    'tip.mult': 'Pengali token yang diterapkan pada pemakaian token API efektif saat permintaan diselesaikan.',
    'nf.code': '404 · Tidak ditemukan', 'nf.h': 'Halaman tidak ditemukan', 'nf.p': 'Halaman yang Anda cari tidak ada atau sudah dipindahkan.', 'nf.subject': 'Node2Mint: halaman tidak ditemukan',
    'nf.home': 'Kembali ke beranda', 'nf.help': 'Buka pusat bantuan',
    'e5.code': '500 · Kesalahan server', 'e5.h': 'Terjadi kesalahan', 'e5.p': 'Halaman ini gagal dimuat. Coba lagi atau kembali ke beranda. Tidak ada saldo yang terpotong.',
    'e5.retry': 'Coba lagi', 'e5.home': 'Kembali ke beranda',

    'auth.hIn': 'Masuk ke akun', 'auth.hUp': 'Buat akun baru', 'auth.noAcc': 'Belum punya akun? ', 'auth.hasAcc': 'Sudah punya akun? ',

    'auth.toUp': 'Daftar', 'auth.toIn': 'Masuk', 'auth.email': 'Email', 'auth.emailInvalid': 'Masukkan alamat email yang valid.', 'auth.turnstileRequired': 'Selesaikan verifikasi keamanan terlebih dahulu.', 'auth.fullName': 'Nama lengkap', 'auth.fullNamePh': 'Nama lengkap Anda', 'auth.fullNameParts': 'Masukkan nama lengkap (minimal 2 karakter).', 'auth.pass': 'Kata sandi',
    'auth.showPass': 'Tampilkan kata sandi', 'auth.hidePass': 'Sembunyikan kata sandi', 'auth.passReq': 'Kata sandi harus terdiri dari 8–72 karakter.',

    'auth.forgot': 'Lupa kata sandi?', 'auth.passIn': 'Kata sandi kamu', 'auth.passUp': '8–72 karakter', 'auth.registered': 'Pendaftaran diproses. Silakan masuk untuk melanjutkan.', 'auth.accountCreated': 'Akun berhasil dibuat.', 'auth.loginSuccess': 'Berhasil masuk.', 'auth.emailRegistered': 'Email sudah terdaftar.',
    'forgot.h': 'Atur ulang kata sandi', 'forgot.sub': 'Masukkan email yang terhubung ke akunmu. Jika cocok dengan akun, kami akan mengirim tautan pengaturan ulang.', 'forgot.sent': 'Jika alamat tersebut terdaftar, tautan pengaturan ulang sudah dikirim. Periksa inbox dan folder spam.', 'forgot.email': 'Email akun', 'forgot.placeholder': 'kamu@contoh.com', 'forgot.send': 'Kirim tautan pengaturan ulang', 'forgot.sending': 'Mengirim…', 'forgot.signIn': 'Kembali ke masuk', 'forgot.help': 'Butuh bantuan lain?', 'forgot.fail': 'Gagal meminta tautan pengaturan ulang. Coba lagi nanti.',
    'reset.title': 'Tetapkan kata sandi baru', 'reset.sub': 'Gunakan tautan sekali pakai dari email. Tautan kedaluwarsa dalam 15 menit.', 'reset.new': 'Kata sandi baru', 'reset.newPh': '8–72 karakter', 'reset.repeat': 'Ulangi kata sandi baru', 'reset.mismatch': 'Kata sandi tidak sama.', 'reset.saving': 'Menyimpan…', 'reset.save': 'Simpan kata sandi baru', 'reset.saved': 'Kata sandi berhasil diubah. Semua sesi sebelumnya sudah dikeluarkan.', 'reset.toSignIn': 'Masuk', 'reset.missing': 'Tautan pengaturan ulang tidak valid atau tidak lengkap.', 'reset.request': 'Minta tautan baru', 'reset.invalid': 'Tautan pengaturan ulang tidak valid atau sudah kedaluwarsa.',

    'auth.passConfirm': 'Verifikasi kata sandi', 'auth.passMismatch': 'Verifikasi kata sandi tidak sama.', 'auth.refOptional': 'Kode rujukan (opsional)',

    'auth.aff': 'Kode undangan terpasang: ', 'auth.busy': 'Memproses…', 'auth.submitIn': 'Masuk', 'auth.submitUp': 'Daftar & Masuk', 'auth.backHome': 'Kembali ke beranda',

    'auth.fail': 'Terjadi kesalahan. Coba lagi.', 'auth.badCreds': 'Kata sandi salah.', 'auth.or': 'atau lanjut dengan',
    'auth.totpLabel': 'Kode autentikator', 'auth.totpNeeded': 'Masukkan kode 6 angka dari aplikasi autentikator.',
    'auth.totpWrong': 'Kode salah. Periksa aplikasi autentikator lalu coba lagi.',
    'auth.totpHint': 'Buka aplikasi autentikator dan masukkan kode 6 angka yang sedang aktif.',

    'auth.with': 'Lanjut dengan {name}', 'auth.unavailable': 'Belum dikonfigurasi', 'auth.cbFail': 'Masuk tidak selesai. Tautan tidak membawa token.', 'auth.cbBack': 'Kembali ke halaman masuk',

    'auth.kickerIn': '// Akses akun', 'auth.kickerUp': '// Akun baru',

    'auth.brandTitle': 'Jual & pakai produk AI dengan satu saldo.', 'auth.brandSub': 'Pasar produk digital plus gerbang API: saldo atomik, kunci API disimpan sebagai ringkasan kriptografis, dan pengalihan penyedia otomatis.', 'auth.brandFoot': 'Pasar Digital + Gerbang API',

    'dash.hello': 'Halo, {name}', 'dash.label': 'Dasbor', 'dash.acct': 'Ringkasan akun',

    'dash.loadErr': 'Gagal memuat ringkasan akun. Periksa koneksi lalu muat ulang halaman.', 'dash.noWallet': 'Ringkasan saldo tidak tersedia.',

    'dash.tokens': 'Sisa Token API', 'dash.tokenUnit': 'token',

    'dash.tokenExp': 'Dipotong tiap panggilan API · kedaluwarsa terdekat {date}', 'dash.tokenNone': 'Dipotong tiap panggilan API · tanpa paket aktif — beli di Paket & Harga.',

    'dash.stale': 'Masa aktif habis — beli paket untuk mengaktifkan kembali token tersimpan.',

    'dash.tokenOutTitle': 'Kuota token habis — permintaan akan ditolak.',

    'dash.usage': 'Pemakaian API', 'dash.reqUnit': 'permintaan', 'dash.req24': '{n} panggilan dalam 24 jam terakhir',

    'dash.detail': 'Rincian token', 'dash.in': 'Masukan', 'dash.cached': 'Tersimpan', 'dash.out': 'Keluaran', 'dash.manageKeys': 'Kelola Kunci API →',

    'dash.recentModel': 'Permintaan terbaru · Model', 'dash.recentTokens': 'Masuk / Keluar', 'dash.recentWhen': 'Waktu', 'dash.noRecent': 'Belum ada permintaan.',

    'dash.wallet': 'Dompet', 'dash.walletP': 'Uang nyata — belanja pasar digital & beli paket token.', 'dash.manageWallet': 'Kelola dompet →',

    'dash.store': 'Toko Saya', 'dash.products': 'produk · {n} pesanan', 'dash.manageStore': 'Kelola toko →', 'dash.noStore': 'Belum punya toko.', 'dash.openStore': 'Buka toko →',

    'dash.aff': 'Afiliasi', 'dash.affStat': '{c} klik · {v} konversi', 'dash.manageAff': 'Kelola afiliasi →',

    'dash.orders': 'Pesanan Terakhir', 'dash.allOrders': 'Semua pesanan →', 'dash.noOrders': 'Belum ada pesanan.',

    'adm.mode': 'Mode Administrator', 'adm.h': 'Dasbor Administrator', 'adm.overview': 'Ikhtisar platform', 'adm.loadErr': 'Gagal memuat ikhtisar platform.',

    'adm.users': 'Pengguna', 'adm.orders': 'Pesanan', 'adm.products': 'Produk', 'adm.revenue': 'Pendapatan',

    'an.title': 'Analitik', 'an.data': 'Data', 'an.range': 'Rentang waktu', 'an.scope': 'Lingkup data',

    'an.sendiri': 'Sendiri', 'an.toko': 'Toko', 'an.platform': 'Platform',

    'an.noStore': 'Toko belum terdaftar - buka toko dulu di halaman Toko Saya.',

    'an.reqs': 'Permintaan', 'an.tokens': 'Token', 'an.tokensIn': 'Token masuk', 'an.tokensOut': 'Token keluar', 'an.aiCost': 'Biaya AI', 'an.topup': 'Isi saldo',

    'an.reqsPerDay': 'Permintaan per hari', 'an.topModels': 'Model Teratas', 'an.noModelUse': 'Belum ada pemakaian model.',

    'an.revenue': 'Pendapatan', 'an.views': 'Dilihat', 'an.inCarts': 'Di keranjang', 'an.sold': 'Terjual', 'an.conv': 'Konversi', 'an.topProducts': 'Produk Teratas',

    'an.noProducts': 'Belum ada produk di toko ini.', 'an.units': '{n} unit', 'an.viewedSold': '{v} dilihat · {s} terjual', 'an.cartCount': '{n} di keranjang',

    'an.signups': 'Pendaftar baru', 'an.activeUsers': 'Pengguna aktif', 'an.topupRevenue': 'Pendapatan isi saldo', 'an.topupTx': 'Transaksi isi saldo',

    'an.orderGmv': 'Omzet pesanan', 'an.ordersCard': 'Pesanan', 'an.signupsPerDay': 'Pendaftaran per hari', 'an.funnel': 'Corong',

    'an.noData': 'Tidak ada data.', 'an.chartDaily': 'Grafik per hari', 'an.modelsAria': 'Model teratas',

    'an.fDaftar': 'Daftar', 'an.fTopup': 'Isi saldo', 'an.fPakaiAPI': 'Pakai API',

  },

} as const;



// Gabungan literal (bukan Record<string,string>) supaya t() tetap punya union

// kunci yang sah: salah ketik kunci = error kompilasi, bukan teks kosong.

type EnDict = typeof base.en & typeof appEn & AdminEnDict;

type IdDict = typeof base.id & typeof appId & AdminIdDict;

type PublicEnDict = typeof base.en & typeof appEn;
type PublicIdDict = typeof base.id & typeof appId;

type LocaleOverrides = Partial<Record<keyof EnDict, string>>;

// ponytail: terjemahkan chrome paling sering terlihat; kunci lain fallback ke EN.
const manualLocaleOverrides: Record<'cn' | 'ru' | 'jp', LocaleOverrides> = {
  cn: {
    'nav.market': '市场', 'nav.models': '模型', 'nav.pricing': '价格', 'nav.blog': '博客/新闻',
    'nav.docs': '文档', 'nav.status': '服务状态', 'nav.faq': '常见问题', 'nav.menu': '菜单',
    'nav.dashboard': '控制面板', 'nav.signin': '免费开始', 'nav.lang': '语言', 'nav.home': 'Node2Mint，首页',
    'foot.market': '浏览市场', 'foot.docs': '文档', 'foot.status': '服务状态', 'foot.product': '产品',
    'foot.learn': '学习', 'foot.company': '公司', 'foot.support': '支持', 'foot.privacy': '隐私', 'foot.community': '社区', 'foot.guides': '指南', 'foot.faq': '常见问题', 'foot.changelog': '更新日志', 'foot.terms': '条款', 'foot.refund': '退款政策',
    'foot.models': '模型目录', 'foot.packs': '令牌套餐', 'foot.pricing': '价格', 'foot.keys': 'API 密钥', 'foot.playground': '试用台', 'foot.sell': '在 Node2Mint 销售', 'foot.affiliate': '联盟推广', 'foot.referral': '推荐计划', 'foot.help': '帮助中心', 'foot.articles': '文章',
    'foot.about': '一个账户即可使用 AI 模型、API 令牌额度、数字市场、联盟佣金和推荐奖励。', 'foot.copyright': '© 2026 Node2Mint', 'foot.signature': 'AI API 与数字市场', 'foot.credit': '· 用心打造', 'foot.creditAria': 'YanSTD 用心打造',
    'sb.dashboard': '控制面板', 'sb.pricing': '套餐与价格', 'sb.market': '市场', 'sb.orders': '订单',
    'sb.wallet': '钱包', 'sb.settings': '账户设置', 'sb.logout': '退出登录', 'sb.notifications': '通知',
    'sb.playground': '游乐场', 'sb.gTransaksi': '交易与财务', 'sb.gKemitraan': '合作与集成',
    'sb.store': '我的商店', 'sb.affiliate': '联盟', 'sb.referral': '推荐', 'sb.apikeys': 'API 密钥', 'sb.help': '帮助中心',
    // Sidebar admin (dict/admin.ts) — t() melempar cn/ru/jp ke EN tanpa ini.
    'sb.admin': 'Node2Mint 管理后台', 'sb.adminDash': '管理面板', 'sb.gOps': '运营与财务', 'sb.gAi': 'AI 与用户',
    'sb.gInfra': '基础设施与日志', 'sb.packages': '令牌套餐', 'sb.products': '产品', 'sb.users': '用户',
    'sb.providers': '服务商', 'sb.models': '模型', 'sb.uptime': '在线时长', 'sb.health': '健康状态',
    'sb.auditlog': '审计日志', 'sb.fraud': '欺诈事件', 'sb.articles': '文章管理', 'sb.platform': '平台设置',
    'sb.disputes': '争议', 'sb.redeem': '兑换码',
    'auth.backHome': '返回首页', 'auth.badCreds': '电子邮件或密码不正确。', 'auth.fail': '发生错误，请重试。',
    'auth.kickerIn': '// 账户访问', 'auth.kickerUp': '// 新账户', 'auth.hIn': '登录', 'auth.hUp': '创建账户', 'auth.noAcc': '还没有账户？', 'auth.hasAcc': '已有账户？',
    'auth.toUp': '注册', 'auth.toIn': '登录', 'auth.email': '电子邮件', 'auth.fullName': '姓名', 'auth.fullNamePh': '请输入您的全名', 'auth.fullNameParts': '请输入名字和姓氏。',
    'auth.pass': '密码', 'auth.showPass': '显示密码', 'auth.hidePass': '隐藏密码', 'auth.passReq': '密码必须为 8–72 个字符。', 'auth.forgot': '忘记密码？', 'auth.passIn': '您的密码', 'auth.passUp': '8–72 个字符',
    'auth.passConfirm': '确认密码', 'auth.passMismatch': '两次密码不一致。', 'auth.refOptional': '推荐码（可选）', 'auth.aff': '已应用邀请码：', 'auth.busy': '处理中…', 'auth.submitIn': '登录', 'auth.submitUp': '注册并继续',
    'auth.or': '或继续使用', 'auth.with': '使用 {name} 继续', 'auth.unavailable': '未配置', 'auth.cbFail': '登录未完成。链接中没有令牌。', 'auth.cbBack': '返回登录',
    'auth.brandTitle': '用一个余额销售和使用 AI 产品。', 'auth.brandSub': '数字产品市场与 API 网关：原子余额、哈希 API 密钥和自动提供商故障转移。', 'auth.brandFoot': '市场 + API 网关',
    'models.showAll': '显示全部 {n} 个模型', 'models.showLess': '收起',
    'mkt.demo': '演示商品 · 不出售', 'mkt.demoNote': '此演示商品仅供浏览，不能购买或加入购物车。',
    'models.freeTerms': '需要登录 · 每天 {n} 个 API 代币 · 超出部分使用你的代币包余额。', 'models.paidTerms': '按 ×{n} 消耗 API 代币；需要有效代币包。',
    'price.currencyBasis': '印尼语显示印尼盾；其他语言显示美元，固定汇率 1 USD = Rp{n}。结算账本仍为 IDR。',
    'price.billingFormula': '免费模型每天含 {free} 个 API 代币。计费 API 代币 = 有效输入（缓存输入按 {cache}% 计）+ 输出，再加 {surcharge}% 平台使用附加费和模型倍率。响应后按供应商报告的用量结算。',
    'price.signupTerms': '注册时一次性赠送。以上显示的代币数量和有效期适用。',
    'faqPage.h': '常见问题', 'faqPage.q1': '钱包和代币有什么区别？', 'faqPage.a1': '钱包保存交易资金；代币是 API 请求使用的额度。',
    'faqPage.q2': '卖家何时收到款项？', 'faqPage.a2': '买家确认订单后，或卖家标记已交付 48 小时后自动到账；如有未解决争议则例外。',
    'faqPage.q3': '推荐计划如何运作？', 'faqPage.a3': '通过本地 IP 检查的推荐，为推荐人和新用户各发放 250 万代币及一天有效期。',
    'faqPage.q4': '如何充值钱包并购买代币套餐？', 'faqPage.a4': '打开钱包页面并选择支付方式。支付完成后余额进入钱包；购买代币套餐会为账户添加代币及其有效期。',
    'faqPage.q5': '市场里可以买卖什么？', 'faqPage.a5': '提示词、数据集、工具、模板和数字服务等数字产品。买家从钱包支付，卖家在店铺管理库存、交付和争议。',
    'faqPage.q6': 'API 代币用完了会怎样？', 'faqPage.a6': '新的 API 请求会被拒绝，直到激活另一个代币套餐或使用有效的兑换码。钱包余额不会自动用于 API 请求。',
    'faqPage.help': '没有找到答案？', 'faqPage.helpLink': '打开帮助中心', 'docs.examplePrompt': '你好！', 'price.signupGiven': '注册时已包含',
    'sup.kicker': '支持', 'sup.h': '联系支持团队', 'sup.sub': '集中处理待处理充值、提现、退款、市场纠纷和账户锁定问题。', 'sup.emailH': '支持邮箱', 'sup.emailP': '请使用注册邮箱发送至 {email}，以便我们核对账户。', 'sup.emailCta': '打开邮件草稿', 'sup.copyEmail': '复制邮箱地址', 'sup.copied': '邮箱地址已复制。', 'sup.hoursH': '回复时间', 'sup.hoursP': '首次回复将在 3–5 个工作日内（周一至周五，WIB）。请附上订单 ID 或充值参考号以加快处理。', 'sup.needH': '请提供以下信息', 'sup.need1': '注册账户的邮箱', 'sup.need2': '订单 ID、提现 ID 或充值参考号', 'sup.need3': '金额、日期和收款方式（银行/电子钱包/网络）', 'sup.need4': '相关截图或交易哈希', 'sup.escalateH': '资金相关问题', 'sup.escalateP': '充值和提现需等待管理员核验；市场资金由托管保管。如资金请求在 3–5 个工作日内（周一至周五，WIB）未获回复，请向同一邮箱升级并附上参考号。', 'sup.resetH': '无法登录账户？', 'sup.resetP': '密码重置邮件尚未启用。请使用账户邮箱联系支持团队并申请重置链接。', 'sup.resetBtn': '申请重置密码', 'sup.faq': '阅读常见问题', 'sup.docs': '打开文档', 'sup.status': '服务状态',
    'st.bannerAria': '服务状态通知', 'st.maint': '维护中', 'st.outage': '服务中断', 'st.bannerMore': '查看状态',
    'tip.avail': '24 小时内未中断的检查占比。“—”表示尚未检查。', 'tip.p90': '24 小时内 90% 的检查比此值更快。',
    'tip.base': '所有模型共用的公共 API 端点，只需替换此 URL。', 'tip.mult': '结算时应用于有效 API 代币用量的代币倍率。',
    'nf.code': '404 · 未找到', 'nf.h': '页面未找到', 'nf.p': '你要找的页面不存在或已移动。', 'nf.home': '返回首页', 'nf.help': '打开帮助中心',
    'e5.code': '500 · 服务器错误', 'e5.h': '出现错误', 'e5.p': '页面无法加载。请重试或返回首页，未扣除任何余额。', 'e5.retry': '重试', 'e5.home': '返回首页',
    'docs.copyCode': '复制代码示例', 'docs.copyFail': '复制失败，请选中代码手动复制。',
    'docs.localLabel': '本地路由器（仅开发用)', 'docs.localWarn': '本端点仅可在运行本地路由器的机器上使用，不能从浏览器或其他设备访问。',
    'docs.localHost': 'https://www.node2mint.com/v1', 'docs.localKeyEnv': 'HERMES_CUSTOM_YANSTD_V1_API_KEY',
    'docs.localCurl': 'curl https://www.node2mint.com/v1/models \\\n  -H "Authorization: Bearer $NODE2MINT_KEY"',
    'docs.intLocal': '公开 API 端点', 'docs.intLocalWarn': '从任何兼容客户端使用此规范公开端点。',
    'docs.intLocalKey': '使用环境变量 NODE2MINT_KEY 中的 Node2Mint API 密钥。',
    'docs.intProd': '生产端点', 'docs.intProdHint': '从任何客户端使用此规范 URL。',
  },
  ru: {
    'nav.market': 'Маркетплейс', 'nav.models': 'Модели', 'nav.pricing': 'Цены', 'nav.blog': 'Блог/Новости',
    'nav.docs': 'Документы', 'nav.status': 'Статус сервиса', 'nav.faq': 'Частые вопросы', 'nav.menu': 'Меню',
    'nav.dashboard': 'Панель', 'nav.signin': 'Начать бесплатно', 'nav.lang': 'Язык', 'nav.home': 'Node2Mint, главная',
    'foot.market': 'Открыть маркетплейс', 'foot.docs': 'Документация', 'foot.status': 'Статус сервиса',
    'foot.product': 'Продукт', 'foot.learn': 'Обучение', 'foot.company': 'Компания', 'foot.support': 'Поддержка',
    'foot.privacy': 'Конфиденциальность', 'foot.community': 'Сообщество', 'foot.guides': 'Руководства', 'foot.faq': 'Частые вопросы', 'foot.changelog': 'Журнал изменений', 'foot.terms': 'Условия', 'foot.refund': 'Политика возврата',
    'foot.models': 'Каталог моделей', 'foot.packs': 'Пакеты токенов', 'foot.pricing': 'Цены', 'foot.keys': 'API-ключи', 'foot.playground': 'Песочница', 'foot.sell': 'Продавать на Node2Mint', 'foot.affiliate': 'Партнёрская программа', 'foot.referral': 'Реферальная программа', 'foot.help': 'Справочный центр', 'foot.articles': 'Статьи',
    'foot.about': 'Один аккаунт для AI-моделей, квоты API-токенов, цифрового маркетплейса, партнёрских комиссий и реферальных наград.', 'foot.copyright': '© 2026 Node2Mint', 'foot.signature': 'AI API и цифровой маркетплейс', 'foot.credit': '· Сделано с', 'foot.creditAria': 'Сделано с любовью YanSTD', 'sb.dashboard': 'Панель', 'sb.pricing': 'Пакеты и цены',
    'sb.market': 'Маркетплейс', 'sb.orders': 'Заказы', 'sb.wallet': 'Кошелёк', 'sb.settings': 'Настройки аккаунта',
    'sb.logout': 'Выйти', 'sb.notifications': 'Уведомления', 'sb.playground': 'Песочница',
    'sb.gTransaksi': 'ТРАНЗАКЦИИ И ФИНАНСЫ', 'sb.gKemitraan': 'ПАРТНЁРСТВА И ИНТЕГРАЦИЯ',
    'sb.store': 'Мой магазин', 'sb.affiliate': 'Партнёрская программа', 'sb.referral': 'Рефералы', 'sb.apikeys': 'API-ключи', 'sb.help': 'Справочный центр',
    // Sidebar админа (dict/admin.ts): без этого t() откатывается на EN для cn/ru/jp.
    'sb.admin': 'Администрирование Node2Mint', 'sb.adminDash': 'Панель администратора', 'sb.gOps': 'Операции и финансы', 'sb.gAi': 'AI и пользователи',
    'sb.gInfra': 'Инфраструктура и логи', 'sb.packages': 'Пакеты токенов', 'sb.products': 'Продукты', 'sb.users': 'Пользователи',
    'sb.providers': 'Провайдеры', 'sb.models': 'Модели', 'sb.uptime': 'Доступность', 'sb.health': 'Состояние',
    'sb.auditlog': 'Журнал аудита', 'sb.fraud': 'События фрода', 'sb.articles': 'Управление статьями', 'sb.platform': 'Настройки платформы',
    'sb.disputes': 'Споры', 'sb.redeem': 'Коды активации',
    'auth.backHome': 'На главную', 'auth.badCreds': 'Неверный email или пароль.', 'auth.fail': 'Что-то пошло не так. Повторите попытку.',
    'auth.kickerIn': '// ДОСТУП К АККАУНТУ', 'auth.kickerUp': '// НОВЫЙ АККАУНТ', 'auth.hIn': 'Войти', 'auth.hUp': 'Создать аккаунт', 'auth.noAcc': 'Нет аккаунта?', 'auth.hasAcc': 'Уже есть аккаунт?',
    'auth.toUp': 'Зарегистрироваться', 'auth.toIn': 'Войти', 'auth.email': 'Электронная почта', 'auth.fullName': 'Полное имя', 'auth.fullNamePh': 'Ваше полное имя', 'auth.fullNameParts': 'Введите имя и фамилию.',
    'auth.pass': 'Пароль', 'auth.showPass': 'Показать пароль', 'auth.hidePass': 'Скрыть пароль', 'auth.passReq': 'Пароль должен содержать от 8 до 72 символов.', 'auth.forgot': 'Забыли пароль?', 'auth.passIn': 'Ваш пароль', 'auth.passUp': '8–72 символа',
    'auth.passConfirm': 'Подтвердите пароль', 'auth.passMismatch': 'Пароли не совпадают.', 'auth.refOptional': 'Реферальный код (необязательно)', 'auth.aff': 'Код приглашения применён: ', 'auth.busy': 'Обработка…', 'auth.submitIn': 'Войти', 'auth.submitUp': 'Зарегистрироваться и продолжить',
    'auth.or': 'или продолжить через', 'auth.with': 'Продолжить через {name}', 'auth.unavailable': 'Не настроено', 'auth.cbFail': 'Вход не завершён. В ссылке нет токена.', 'auth.cbBack': 'Вернуться ко входу',
    'auth.brandTitle': 'Продавайте и используйте AI-продукты с одним балансом.', 'auth.brandSub': 'Маркетплейс цифровых продуктов и API-шлюз: атомарный баланс, хешированные API-ключи и автоматическое переключение провайдеров.', 'auth.brandFoot': 'МАРКЕТПЛЕЙС + API-ШЛЮЗ',
    'models.showAll': 'Показать все {n} моделей', 'models.showLess': 'Показать меньше',
    'mkt.demo': 'Демо · не продаётся', 'mkt.demoNote': 'Это демонстрационное объявление можно только просматривать: его нельзя купить или добавить в корзину.',
    'models.freeTerms': 'Требуется вход · {n} API-токенов в день · перерасход использует баланс пакета токенов.', 'models.paidTerms': 'Расходует API-токены с ×{n}; нужен активный пакет токенов.',
    'price.currencyBasis': 'Для индонезийского языка цены в IDR; для остальных — USD по фиксированному курсу 1 USD = Rp{n}. Расчётный реестр остаётся в IDR.',
    'price.billingFormula': 'Бесплатные модели включают {free} API-токенов в день. Списываемые API-токены = эффективный ввод (кэшированный ввод по {cache}%) + вывод, затем наценка платформы {surcharge}% и множитель модели. Использование от провайдера рассчитывается после ответа.',
    'price.signupTerms': 'Выдаётся один раз при регистрации. Применяются указанные выше количество токенов и срок действия.',
    'faqPage.h': 'Частые вопросы', 'faqPage.q1': 'В чём разница между кошельком и токенами?', 'faqPage.a1': 'Кошелёк хранит деньги для транзакций. Токены — это квота для API-запросов.',
    'faqPage.q2': 'Когда продавец получает средства?', 'faqPage.a2': 'После подтверждения заказа покупателем или автоматически через 48 часов после отметки доставки продавцом, если нет открытого спора.',
    'faqPage.q3': 'Как работает реферальная программа?', 'faqPage.a3': 'Рефералы, прошедшие локальную IP-проверку, получают по 2,5 млн токенов и один активный день для пригласившего и нового пользователя.',
    'faqPage.q4': 'Как пополнить кошелёк и купить пакет токенов?', 'faqPage.a4': 'Откройте страницу кошелька и выберите способ оплаты. После завершения платежа баланс поступает в кошелёк; покупка пакета токенов добавляет токены и срок действия на аккаунт.',
    'faqPage.q5': 'Что можно покупать и продавать на маркетплейсе?', 'faqPage.a5': 'Цифровые продукты: промпты, датасеты, инструменты, шаблоны и цифровые услуги. Покупатели платят из кошелька, продавцы управляют остатками, доставкой и спорами в своём магазине.',
    'faqPage.q6': 'Что произойдёт, когда токены API закончатся?', 'faqPage.a6': 'Новые API-запросы отклоняются, пока не активирован другой пакет токенов или не использован действительный код. Баланс кошелька никогда не списывается автоматически за API-запросы.',
    'faqPage.help': 'Не нашли ответ?', 'faqPage.helpLink': 'Открыть центр помощи', 'docs.examplePrompt': 'Привет!', 'price.signupGiven': 'Уже включено при регистрации',
    'sup.kicker': 'Поддержка', 'sup.h': 'Связаться с поддержкой', 'sup.sub': 'Единая точка для зависших пополнений, выводов, возвратов, споров на маркетплейсе и заблокированных аккаунтов.', 'sup.emailH': 'Почта поддержки', 'sup.emailP': 'Напишите на {email} с адреса, привязанного к аккаунту, чтобы мы могли его сопоставить.', 'sup.emailCta': 'Открыть черновик письма', 'sup.copyEmail': 'Скопировать адрес почты', 'sup.copied': 'Адрес почты скопирован.', 'sup.hoursH': 'Срок ответа', 'sup.hoursP': 'Первый ответ — в течение 3–5 рабочих дней (пн–пт, WIB). Укажите ID заказа или номер пополнения, чтобы ускорить проверку.', 'sup.needH': 'Что указать', 'sup.need1': 'Email зарегистрированного аккаунта', 'sup.need2': 'ID заказа, вывода или номер пополнения', 'sup.need3': 'Сумму, дату и способ получения (банк/электронный кошелёк/сеть)', 'sup.need4': 'Скриншоты или хеш транзакции, если уместно', 'sup.escalateH': 'Движение средств', 'sup.escalateP': 'Пополнения и выводы ждут проверки администратора; средства маркетплейса хранятся в эскроу. Если по денежному запросу нет ответа через 3–5 рабочих дней (пн–пт, WIB), напишите на тот же адрес и укажите номер обращения.', 'sup.resetH': 'Не удаётся войти?', 'sup.resetP': 'Письма для сброса пароля пока не включены. Напишите в поддержку с адреса аккаунта и запросите ссылку для сброса.', 'sup.resetBtn': 'Запросить сброс пароля', 'sup.faq': 'Читать FAQ', 'sup.docs': 'Открыть документацию', 'sup.status': 'Статус сервиса',
    'st.bannerAria': 'Уведомление о статусе сервиса', 'st.maint': 'Идут работы', 'st.outage': 'Сбой сервиса', 'st.bannerMore': 'Открыть статус',
    'tip.avail': 'Доля проверок за 24 часа без сбоев. «—» означает, что проверок ещё не было.', 'tip.p90': '90% проверок за 24 часа были быстрее этого значения.',
    'tip.base': 'Публичный endpoint API для всех моделей — меняйте только этот URL.', 'tip.mult': 'Множитель токенов, применяемый к эффективному расходу API-токенов при расчёте.',
    'nf.code': '404 · Не найдено', 'nf.h': 'Страница не найдена', 'nf.p': 'Страницы не существует или она перемещена.', 'nf.home': 'На главную', 'nf.help': 'Открыть центр помощи',
    'e5.code': '500 · Ошибка сервера', 'e5.h': 'Что-то пошло не так', 'e5.p': 'Страницу не удалось загрузить. Повторите попытку или вернитесь на главную. Списаний не было.', 'e5.retry': 'Повторить', 'e5.home': 'На главную',
    'docs.copyCode': 'Скопировать пример кода', 'docs.copyFail': 'Не удалось скопировать. Выделите код и скопируйте вручную.',
    'docs.localLabel': 'Локальный роутер (только dev)', 'docs.localWarn': 'Этот эндпоинт работает ТОЛЬКО на машине, где запущен локальный роутер, а не с браузера или другого устройства.',
    'docs.localHost': 'https://www.node2mint.com/v1', 'docs.localKeyEnv': 'HERMES_CUSTOM_YANSTD_V1_API_KEY',
    'docs.localCurl': 'curl https://www.node2mint.com/v1/models \\\n  -H "Authorization: Bearer $NODE2MINT_KEY"',
    'docs.intLocal': 'Публичный API endpoint', 'docs.intLocalWarn': 'Используйте этот канонический публичный endpoint из любого совместимого клиента.',
    'docs.intLocalKey': 'Используйте API-ключ Node2Mint из переменной среды NODE2MINT_KEY.',
    'docs.intProd': 'Рабочий endpoint', 'docs.intProdHint': 'Используйте этот канонический URL из любого клиента.',
  },
  jp: {
    'nav.market': 'マーケット', 'nav.models': 'モデル', 'nav.pricing': '料金', 'nav.blog': 'ブログ/ニュース',
    'nav.docs': 'ドキュメント', 'nav.status': 'サービス状況', 'nav.faq': 'よくある質問', 'nav.menu': 'メニュー',
    'nav.dashboard': 'ダッシュボード', 'nav.signin': '無料で始める', 'nav.lang': '言語', 'nav.home': 'Node2Mint、ホーム',
    'foot.market': 'マーケットを見る', 'foot.docs': 'ドキュメント', 'foot.status': 'サービス状況',
    'foot.product': 'プロダクト', 'foot.learn': '学ぶ', 'foot.company': '会社', 'foot.support': 'サポート',
    'foot.privacy': 'プライバシー', 'foot.community': 'コミュニティ', 'foot.guides': 'ガイド', 'foot.faq': 'よくある質問', 'foot.changelog': '更新履歴', 'foot.terms': '利用規約', 'foot.refund': '返金ポリシー',
    'foot.models': 'モデルカタログ', 'foot.packs': 'トークンパック', 'foot.pricing': '料金', 'foot.keys': 'API キー', 'foot.playground': 'プレイグラウンド', 'foot.sell': 'Node2Mint で販売する', 'foot.affiliate': 'アフィリエイト', 'foot.referral': '紹介プログラム', 'foot.help': 'ヘルプセンター', 'foot.articles': '記事',
    'foot.about': 'AI モデル、API トークン枠、デジタルマーケットプレイス、アフィリエイト報酬、紹介特典をひとつのアカウントで管理できます。', 'foot.copyright': '© 2026 Node2Mint', 'foot.signature': 'AI API とデジタルマーケットプレイス', 'foot.credit': '· 心を込めて制作', 'foot.creditAria': 'YanSTD が心を込めて制作', 'sb.dashboard': 'ダッシュボード', 'sb.pricing': 'パックと料金',
    'sb.market': 'マーケット', 'sb.orders': '注文', 'sb.wallet': 'ウォレット', 'sb.settings': 'アカウント設定',
    'sb.logout': 'ログアウト', 'sb.notifications': '通知', 'sb.playground': 'プレイグラウンド',
    'sb.gTransaksi': '取引と財務', 'sb.gKemitraan': 'パートナーシップと連携',
    'sb.store': 'マイストア', 'sb.affiliate': 'アフィリエイト', 'sb.referral': '紹介', 'sb.apikeys': 'API キー', 'sb.help': 'ヘルプセンター',
    // 管理サイドバー（dict/admin.ts）。これがないと cn/ru/jp は EN にフォールバックします。
    'sb.admin': 'Node2Mint 管理', 'sb.adminDash': '管理ダッシュボード', 'sb.gOps': '運用と財務', 'sb.gAi': 'AI とユーザー',
    'sb.gInfra': 'インフラとログ', 'sb.packages': 'トークンパック', 'sb.products': 'プロダクト', 'sb.users': 'ユーザー',
    'sb.providers': 'プロバイダー', 'sb.models': 'モデル', 'sb.uptime': '稼働時間', 'sb.health': 'ヘルス',
    'sb.auditlog': '監査ログ', 'sb.fraud': '不正イベント', 'sb.articles': '記事管理', 'sb.platform': 'プラットフォーム設定',
    'sb.disputes': '紛争', 'sb.redeem': 'クーポンコード',
    'auth.backHome': 'ホームに戻る', 'auth.badCreds': 'メールアドレスまたはパスワードが正しくありません。', 'auth.fail': 'エラーが発生しました。もう一度お試しください。',
    'auth.kickerIn': '// アカウントアクセス', 'auth.kickerUp': '// 新規アカウント', 'auth.hIn': 'ログイン', 'auth.hUp': 'アカウントを作成', 'auth.noAcc': 'アカウントをお持ちでないですか？', 'auth.hasAcc': 'すでにアカウントをお持ちですか？',
    'auth.toUp': '登録', 'auth.toIn': 'ログイン', 'auth.email': 'メールアドレス', 'auth.fullName': '氏名', 'auth.fullNamePh': 'フルネームを入力', 'auth.fullNameParts': '名前と姓を入力してください。',
    'auth.pass': 'パスワード', 'auth.showPass': 'パスワードを表示', 'auth.hidePass': 'パスワードを隠す', 'auth.passReq': 'パスワードは 8～72 文字で入力してください。', 'auth.forgot': 'パスワードをお忘れですか？', 'auth.passIn': 'パスワードを入力', 'auth.passUp': '8～72 文字',
    'auth.passConfirm': 'パスワードを確認', 'auth.passMismatch': 'パスワードが一致しません。', 'auth.refOptional': '紹介コード（任意）', 'auth.aff': '招待コードを適用しました：', 'auth.busy': '処理中…', 'auth.submitIn': 'ログイン', 'auth.submitUp': '登録して続行',
    'auth.or': 'または次で続行', 'auth.with': '{name} で続行', 'auth.unavailable': '未設定', 'auth.cbFail': 'ログインが完了しませんでした。リンクにトークンがありません。', 'auth.cbBack': 'ログインに戻る',
    'auth.brandTitle': 'ひとつの残高で AI 製品を販売・利用。', 'auth.brandSub': 'デジタル製品マーケットプレイスと API ゲートウェイ：原子的な残高、ハッシュ化された API キー、自動プロバイダーフェイルオーバー。', 'auth.brandFoot': 'マーケットプレイス + API ゲートウェイ',
    'models.showAll': '全 {n} モデルを表示', 'models.showLess': '表示を減らす',
    'mkt.demo': 'デモ · 販売していません', 'mkt.demoNote': 'このデモ掲載は閲覧専用です。購入やカートへの追加はできません。',
    'models.freeTerms': 'ログインが必要 · 1日 {n} API トークン · 超過分はトークンパック残高を使用します。', 'models.paidTerms': 'API トークンを ×{n} で消費します。有効なトークンパックが必要です。',
    'price.currencyBasis': 'インドネシア語では IDR、その他の言語では固定換算 1 USD = Rp{n} の USD を表示します。決済台帳は IDR のままです。',
    'price.billingFormula': '無料モデルには1日 {free} API トークンが含まれます。課金対象 API トークン = 有効入力（キャッシュ入力は {cache}%）+ 出力、その後プラットフォーム利用料 {surcharge}% とモデル倍率が適用されます。プロバイダー報告の使用量は応答後に精算されます。',
    'price.signupTerms': '登録時に一度だけ付与されます。上記のトークン数と有効期間が適用されます。',
    'faqPage.h': 'よくある質問', 'faqPage.q1': 'ウォレットとトークンの違いは？', 'faqPage.a1': 'ウォレットには取引用の残高が入り、トークンは API リクエストに使う利用枠です。',
    'faqPage.q2': '販売者はいつ代金を受け取りますか？', 'faqPage.a2': '購入者が注文を確認した時点、または販売者が配達済みにしてから 48 時間後に自動で受け取れます。未解決の紛争がある場合は除きます。',
    'faqPage.q3': '紹介プログラムはどのように機能しますか？', 'faqPage.a3': 'ローカル IP チェックを通過した紹介では、紹介者と新規ユーザーの両方に 250 万トークンと 1 日の有効期間が付与されます。',
    'faqPage.q4': 'ウォレットにチャージしてトークンパックを購入するには？', 'faqPage.a4': 'ウォレットページを開き支払い方法を選びます。決済完了後、残高がウォレットに入ります。トークンパックを購入するとトークンと有効期間がアカウントに追加されます。',
    'faqPage.q5': 'マーケットでは何を売買できますか？', 'faqPage.a5': 'プロンプト、データセット、ツール、テンプレート、デジタルサービスなどのデジタル製品です。購入者はウォレットから支払い、販売者はストアで在庫・配送・紛争を管理します。',
    'faqPage.q6': 'API トークンが尽きたらどうなりますか？', 'faqPage.a6': '別のトークンパックを有効化するか有効なコードを使うまで、新しい API リクエストは拒否されます。API リクエストでウォレット残高が自動的に引かれることはありません。',
    'faqPage.help': '答えが見つかりませんか？', 'faqPage.helpLink': 'ヘルプセンターを開く', 'docs.examplePrompt': 'こんにちは！', 'price.signupGiven': '登録時にすでに付与されています',
    'sup.kicker': 'サポート', 'sup.h': 'サポートに連絡', 'sup.sub': '保留中のチャージ、出金、返金、マーケットプレイスの紛争、アカウントのロックをまとめて受け付けます。', 'sup.emailH': 'サポートメール', 'sup.emailP': 'アカウントを照合できるよう、登録済みのメールアドレスから {email} 宛にご連絡ください。', 'sup.emailCta': 'メールの下書きを開く', 'sup.copyEmail': 'メールアドレスをコピー', 'sup.copied': 'メールアドレスをコピーしました。', 'sup.hoursH': '返信の目安', 'sup.hoursP': '最初の返信は 3～5 営業日以内です（月～金、WIB）。確認を早めるため、注文 ID またはチャージの参照番号を記載してください。', 'sup.needH': '記載する情報', 'sup.need1': '登録アカウントのメールアドレス', 'sup.need2': '注文 ID、出金 ID、またはチャージの参照番号', 'sup.need3': '金額、日付、送付先（銀行／電子ウォレット／ネットワーク）', 'sup.need4': '必要に応じてスクリーンショットまたはトランザクションハッシュ', 'sup.escalateH': '資金に関するお問い合わせ', 'sup.escalateP': 'チャージと出金は管理者の確認を待ち、マーケットプレイスの資金はエスクローで保管されます。資金に関する依頼に 3～5 営業日（月～金、WIB）以内に返信がない場合は、同じメールアドレスへ参照番号を添えて再度ご連絡ください。', 'sup.resetH': 'アカウントにログインできませんか？', 'sup.resetP': 'パスワード再設定メールはまだ有効ではありません。アカウントのメールアドレスからサポートに連絡し、再設定リンクを依頼してください。', 'sup.resetBtn': 'パスワード再設定を依頼', 'sup.faq': 'よくある質問を読む', 'sup.docs': 'ドキュメントを開く', 'sup.status': 'サービス状況',
    'st.bannerAria': 'サービス状況のお知らせ', 'st.maint': 'メンテナンス中', 'st.outage': 'サービス障害', 'st.bannerMore': '状況を見る',
    'tip.avail': '直近24時間でダウンしなかったチェックの割合。「—」は未チェックです。', 'tip.p90': '直近24時間のチェックの90%がこの値より高速でした。',
    'tip.base': 'すべてのモデルが使う公開APIエンドポイント。このURLだけ変更してください。', 'tip.mult': '精算時に実効 API トークン使用量へ適用されるトークン倍率。',
    'nf.code': '404 · 見つかりません', 'nf.h': 'ページが見つかりません', 'nf.p': 'お探しのページは存在しないか、移動しました。', 'nf.home': 'ホームに戻る', 'nf.help': 'ヘルプセンターを開く',
    'e5.code': '500 · サーバーエラー', 'e5.h': 'エラーが発生しました', 'e5.p': 'ページを読み込めませんでした。再試行するかホームに戻ってください。残高は引かれていません。', 'e5.retry': '再試行', 'e5.home': 'ホームに戻る',
    'docs.copyCode': 'コード例をコピー', 'docs.copyFail': 'コピーできませんでした。コードを選択して手動でコピーしてください.',
    'docs.localLabel': 'ローカルルーター（開発のみ)', 'docs.localWarn': 'このエンドポイントは、ローカルルーターを実行しているマシンでのみ使用できます。ブラウザや他のデバイスからはアクセスできません。',
    'docs.localHost': 'https://www.node2mint.com/v1', 'docs.localKeyEnv': 'HERMES_CUSTOM_YANSTD_V1_API_KEY',
    'docs.localCurl': 'curl https://www.node2mint.com/v1/models \\\n  -H "Authorization: Bearer $NODE2MINT_KEY"',
    'docs.intLocal': '公開 API エンドポイント', 'docs.intLocalWarn': '互換クライアントからこの正規の公開エンドポイントを使用してください。',
    'docs.intLocalKey': '環境変数 NODE2MINT_KEY の Node2Mint API キーを使用してください。',
    'docs.intProd': '本番エンドポイント', 'docs.intProdHint': '任意のクライアントからこの正規 URL を使用してください。',
  },
};

const localeOverrides = {
  cn: { ...generatedLocaleOverrides.cn, ...uiLocaleOverrides.cn, ...manualLocaleOverrides.cn },
  ru: { ...generatedLocaleOverrides.ru, ...uiLocaleOverrides.ru, ...manualLocaleOverrides.ru },
  jp: { ...generatedLocaleOverrides.jp, ...uiLocaleOverrides.jp, ...manualLocaleOverrides.jp },
};

const dict = {
  en: { ...base.en, ...appEn } as PublicEnDict,
  id: { ...base.id, ...appId } as PublicIdDict,
  cn: { ...base.en, ...appEn, ...localeOverrides.cn } as PublicEnDict,
  ru: { ...base.en, ...appEn, ...localeOverrides.ru } as PublicEnDict,
  jp: { ...base.en, ...appEn, ...localeOverrides.jp } as PublicEnDict,
};

type RuntimeAdminDictionary = { en: Record<string, string>; id: Record<string, string> };
let registeredAdminDict: RuntimeAdminDictionary | null = null;
const adminDictListeners = new Set<() => void>();

export function registerAdminDictionary(dictionary: RuntimeAdminDictionary) {
  registeredAdminDict = dictionary;
  adminDictListeners.forEach((listener) => listener());
}



export type Key = Extract<keyof EnDict, string>;



const Ctx = createContext<{ lang: Lang; prefix: string; setLang: (l: Lang) => void; t: (k: Key, vars?: Record<string, string | number>) => string }>({

  lang: 'en', prefix: '', setLang: () => {}, t: (k) => (dict.en as Record<string, string>)[k] ?? k,

});



export function LocaleProvider({ children, serverPrefix = '', serverLang = 'en' }: { children: ReactNode; serverPrefix?: string; serverLang?: string }) {
  const [lang, setLangState] = useState<Lang>(() => URL_LANG[serverPrefix.slice(1)] ?? (serverLang as Lang) ?? 'en');
  const pathname = usePathname();
  // Prefix ikut disemai dari server (prop) supaya tautan di HTML pertama sudah
  // ber-locale untuk perayap, bukan hanya setelah hidrasi.
  const [prefix, setPrefix] = useState(serverPrefix);
  const [adminDictVersion, setAdminDictVersion] = useState(0);

  useEffect(() => {
    const refresh = () => setAdminDictVersion((value) => value + 1);
    adminDictListeners.add(refresh);
    return () => { adminDictListeners.delete(refresh); };
  }, []);

  // URL locale mengalahkan localStorage: halaman /zh/* harus tampil Mandarin
  // walaupun cookie lama berisi 'en'. Re-run saat navigasi klien karena halaman
  // dengan/tanpa prefix bisa saling bergantian tanpa muat ulang.
  //
  // Server sudah memutuskan locale dari cookie/x-locale (layout.tsx) dan sudah
  // merender teksnya; di sini kita hanya menerapkan pilihan yang TERSIMPAN
  // (URL atau localStorage). Tanpa pilihan tersimpan, pertahankan serverLang
  // yang sudah dirender — menebak di klien memicu React #418.
  useEffect(() => {
    const fromUrl = resolveLang(pathname);
    setPrefix(fromUrl ? `/${LANG_URL[fromUrl]}` : '');
    if (fromUrl) { setLangState(fromUrl); return; }
    const saved = localStorage.getItem(KEY) as Lang | null;
    if (saved === 'en' || saved === 'id' || saved === 'cn' || saved === 'ru' || saved === 'jp') { setLangState(saved); }
  }, [pathname]);



  useEffect(() => { document.documentElement.lang = HTML_LANG[lang]; }, [lang]);



  const setLang = useCallback((l: Lang) => {
    localStorage.setItem(KEY, l);
    document.cookie = `${COOKIE}=${l}; Path=/; Max-Age=31536000; SameSite=Lax; Secure`; // A5-04: add Secure flag
    setLangState(l);
    if (l === 'id' || l === 'en') void patch('/account/locale', { locale: l }).catch(() => {});
    // Protected dashboard routes are intentionally unprefixed by proxy.ts.
    // Keep the current route unprefixed when the dashboard shell is mounted;
    // otherwise every language click causes a needless locale 308 redirect.
    if (document.querySelector('aside[aria-label]')) {
      const internalPath = stripLocale(location.pathname);
      location.assign(internalPath + location.search + location.hash);
      return;
    }
    // Bahasa = URL, bukan cookie: ganti prefix locale di alamat supaya halaman
    // yang dibagikan/di-indeks tetap menunjuk bahasa yang benar.
    const parts = location.pathname.split('/');
    if (resolveLang(location.pathname)) parts[1] = LANG_URL[l];
    else parts.splice(1, 0, LANG_URL[l]);
    // Navigasi sungguhan, bukan history.replaceState: Next harus menerima route
    // baru agar pathname, shell publik, metadata, dan HTML locale ikut sinkron.
    location.assign(parts.join('/') + location.search + location.hash);
  }, []);

  const t = useCallback((k: Key, vars?: Record<string, string | number>) => {
    const admin = registeredAdminDict?.[lang === 'id' ? 'id' : 'en'];
    let s: string = admin?.[k] ?? (dict[lang] as Record<string, string>)[k] ?? (dict.en as Record<string, string>)[k] ?? k;

    if (vars) for (const [name, v] of Object.entries(vars)) s = s.replaceAll(`{${name}}`, String(v));

    return s;

  }, [lang, adminDictVersion]);



  return <Ctx.Provider value={{ lang, prefix, setLang, t }}>{children}</Ctx.Provider>;

}



export const useT = () => useContext(Ctx);

