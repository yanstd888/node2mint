import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..');
const files = {
  privacy: readFileSync(resolve(root, 'app/privacy/page.tsx'), 'utf8'),
  terms: readFileSync(resolve(root, 'app/terms/page.tsx'), 'utf8'),
  refund: readFileSync(resolve(root, 'app/refund/page.tsx'), 'utf8'),
  layout: readFileSync(resolve(root, 'components/LegalLayout.tsx'), 'utf8'),
};
const required = {
  privacy: ['provider AI aktif', 'Pasal 34', 'UU No. 27 Tahun 2022'],
  terms: ['Perubahan, hukum, dan sengketa'],
  refund: ['error provider', '5%', 'Tidak ada endpoint banding terpisah'],
  layout: ['binding version', 'Юридически обязательной'],
};
const forbiddenTermsTypos = ['dikompulkan', 'diseletwenggarahi', 'mengharga-walidasi', 'mensyyaratkan'];

for (const [file, phrases] of Object.entries(required)) {
  for (const phrase of phrases) {
    if (!files[file].includes(phrase)) throw new Error(`${file}: missing ${JSON.stringify(phrase)}`);
  }
}
for (const typo of forbiddenTermsTypos) {
  if (files.terms.includes(typo)) throw new Error(`terms: contains typo ${JSON.stringify(typo)}`);
}
console.log('legal static-copy checks passed');
