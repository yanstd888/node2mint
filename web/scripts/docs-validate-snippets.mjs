import assert from 'node:assert/strict';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';

const docsPath = new URL('../components/sections/DocsSection.tsx', import.meta.url);
const source = readFileSync(docsPath, 'utf8');
const block = source.match(/const SAMPLES = \{([\s\S]*?)\n\} as const;/)?.[1];
assert(block, 'Could not find SAMPLES in DocsSection.tsx');

const samples = Object.fromEntries(
  [...block.matchAll(/^  (?:'([^']+)'|(\w+)): `([\s\S]*?)`,?$/gm)].map(([, quoted, bare, code]) => [quoted || bare, code.replaceAll('${BASE_URL}', 'https://www.node2mint.com/v1').replaceAll('${MODEL_ID}', '<MODEL_ID>')]),
);
assert.deepEqual(Object.keys(samples).sort(), ['JavaScript', 'Python', 'cURL', 'cURL stream']);

const temp = mkdtempSync(join(tmpdir(), 'node2mint-docs-'));
const run = (command, args, input) => {
  const result = spawnSync(command, args, { input, encoding: 'utf8' });
  assert.equal(result.status, 0, `${command} ${args.join(' ')} failed:\n${result.stderr || result.stdout}`);
};

try {
  for (const name of ['cURL', 'cURL stream']) {
    const sample = samples[name];
    run('bash', ['-n'], sample);
    const json = sample.match(/-d '([\s\S]*?)'$/)?.[1];
    assert(json, `${name} is missing a JSON request body`);
    JSON.parse(json);
  }

  run('python3', ['-c', 'import ast, sys; ast.parse(sys.stdin.read())'], samples.Python);
  const jsPath = join(temp, 'sample.mjs');
  writeFileSync(jsPath, samples.JavaScript);
  run('node', ['--check', jsPath]);

  assert(!source.includes('proces..._KEY'), 'Truncated JavaScript API key placeholder found');
  assert(!source.includes('window.location.origin'), 'Non-canonical Base URL found');
  assert(source.includes("const BASE_URL = 'https://www.node2mint.com/v1';"), 'Canonical Base URL missing');
  console.log('Validated 4 displayed snippets: shell + JSON (2), Python, JavaScript.');
} finally {
  rmSync(temp, { recursive: true, force: true });
}
