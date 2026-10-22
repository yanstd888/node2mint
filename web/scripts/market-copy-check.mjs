import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..');
const files = {
  affiliate: 'app/afiliasi/page.tsx',
  market: 'app/market/page.tsx',
  referral: 'components/sections/ReferralSection.tsx',
};
const source = Object.fromEntries(await Promise.all(
  Object.entries(files).map(async ([name, file]) => [name, await readFile(resolve(root, file), 'utf8')]),
));

for (const [file, label] of [
  ['referral', 'Reward ini diproses saat pendaftaran, bukan setelah top-up.'],
  ['affiliate', 'Buat link afiliasi produk'],
  ['market', 'Komisi platform yang dikonfigurasi adalah 10% dari subtotal order.'],
  ['market', 'Belum ada listing aktif. Kami tidak menampilkan contoh produk fiktif'],
]) assert.ok(source[file].includes(label), `${files[file]} is missing: ${label}`);

console.log('market copy check passed');
