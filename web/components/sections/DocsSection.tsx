'use client';

import { useState } from 'react';
import Link from 'next/link';
import { BookOpen, Check, Copy, Key, Terminal, Zap } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useToast } from '@/components/ui/toast';
import { type Key as I18nKey, useT, useUrlPrefix } from '@/lib/i18n';

const BASE_URL = 'https://www.node2mint.com/v1';
const MODEL_ID = '<MODEL_ID>';

const SAMPLES = {
  cURL: `export NODE2MINT_KEY="replace-with-your-key"

curl "${BASE_URL}/chat/completions" \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer $NODE2MINT_KEY" \\
  -d '{
    "model": "${MODEL_ID}",
    "messages": [{ "role": "user", "content": "Hello!" }]
  }'`,
  Python: `import os
from openai import OpenAI

client = OpenAI(
    base_url="${BASE_URL}",
    api_key=os.environ["NODE2MINT_KEY"],
)

response = client.chat.completions.create(
    model="${MODEL_ID}",
    messages=[{"role": "user", "content": "Hello!"}],
)
print(response.choices[0].message.content)`,
  JavaScript: `import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "${BASE_URL}",
  apiKey: process.env.NODE2MINT_KEY,
});

const response = await client.chat.completions.create({
  model: "${MODEL_ID}",
  messages: [{ role: "user", content: "Hello!" }],
});
console.log(response.choices[0].message.content);`,
  'cURL stream': `curl -N "${BASE_URL}/chat/completions" \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer $NODE2MINT_KEY" \\
  -d '{
    "model": "${MODEL_ID}",
    "messages": [{ "role": "user", "content": "Hello!" }],
    "stream": true
  }'`,
} as const;

const SAMPLE_LABELS: Record<keyof typeof SAMPLES, I18nKey> = {
  cURL: 'docs.sampleCurl',
  Python: 'docs.samplePython',
  JavaScript: 'docs.sampleJavaScript',
  'cURL stream': 'docs.sampleCurlStream',
};

const ENDPOINTS = [
  ['POST', '/chat/completions', 'docs.endpointChat'],
  ['POST', '/messages', 'docs.endpointMessages'],
  ['GET', '/models', 'docs.endpointModels'],
  ['GET', '/key', 'docs.endpointKey'],
] as const;

const ERROR_CODES = [
  ['400', 'invalid_request_error', 'docs.err400'],
  ['401', 'authentication_error', 'docs.err401'],
  ['402', 'invalid_request_error', 'docs.err402'],
  ['403', 'permission_error', 'docs.err403'],
  ['429', 'rate_limit_error', 'docs.err429'],
  ['502/503/504', 'server_error', 'docs.err5xx'],
] as const;

const STEPS: { icon: typeof Key; title: I18nKey; text: I18nKey }[] = [
  { icon: Key, title: 'docs.s1', text: 'docs.s1p' },
  { icon: Terminal, title: 'docs.s2', text: 'docs.s2p' },
  { icon: Zap, title: 'docs.s3', text: 'docs.s3p' },
];

export function DocsSection() {
  const { t } = useT();
  const prefix = useUrlPrefix();
  const toast = useToast();
  const [tab, setTab] = useState<keyof typeof SAMPLES>('cURL');
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(SAMPLES[tab]);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
      toast.ok(t('docs.copied'));
    } catch {
      toast.err(t('docs.copyFail'));
    }
  };

  return (
    <>
      <header className="text-center mb-12">
        <p className="label mb-2 flex items-center justify-center gap-2"><BookOpen className="w-3.5 h-3.5" aria-hidden="true" /> {t('docs.kicker')}</p>
        <h1 className="text-3xl md:text-4xl font-semibold tracking-tight anim-fade-up">{t('docs.h')}</h1>
        <p className="text-sm text-mute mt-3 max-w-prose mx-auto">{t('docs.sub')}</p>
        <div className="flex items-center justify-center gap-3 mt-6">
          <Link href={`${prefix}/api-keys`}><Button><Key className="w-4 h-4" aria-hidden="true" /> {t('docs.makeKey')}</Button></Link>
          <code className="text-xs text-cyan-300 select-all">{BASE_URL}</code>
        </div>
      </header>

      <ol className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-12">
        {STEPS.map(({ icon: StepIcon, title, text }, index) => (
          <li key={title} className="card lift p-5 anim-fade-up" style={{ animationDelay: `${index * 0.08}s` }}>
            <span className="w-8 h-8 rounded-lg bg-cyan-500/15 border border-cyan-400/30 flex items-center justify-center mb-3"><StepIcon className="w-4 h-4 text-cyan-300" aria-hidden="true" /></span>
            <p className="label mb-1">{t('docs.step', { n: index + 1 })}</p><h2 className="font-medium mb-2">{t(title)}</h2><p className="text-sm text-mute leading-relaxed">{t(text)}</p>
          </li>
        ))}
      </ol>

      <section aria-labelledby="code-title" className="mb-12">
        <h2 id="code-title" className="text-xl font-semibold tracking-tight mb-4">{t('docs.code')}</h2>
        <div role="group" aria-label={t('docs.chooseCode')} className="flex flex-wrap gap-1 bg-surface border border-border rounded-t-xl p-1 w-fit">
          {(Object.keys(SAMPLES) as (keyof typeof SAMPLES)[]).map((name) => <button key={name} aria-pressed={tab === name} onClick={() => setTab(name)} className={`px-4 h-8 rounded-lg text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60 ${tab === name ? 'bg-cyan-500 text-bg' : 'text-mute hover:text-text'}`}>{t(SAMPLE_LABELS[name])}</button>)}
        </div>
        <pre className="card rounded-t-none border-t-0 p-5 overflow-x-auto text-sm leading-relaxed" tabIndex={0}><code className="block w-0 min-w-full font-mono text-cyan-100">{SAMPLES[tab]}</code></pre>
        <div className="mt-2 flex flex-wrap items-center gap-2"><Button variant="outline" size="sm" onClick={copy}>{copied ? <Check className="w-3.5 h-3.5 text-emerald-300" aria-hidden="true" /> : <Copy className="w-3.5 h-3.5" aria-hidden="true" />}{copied ? t('docs.copied') : t('docs.copyButton')}</Button><span className="text-xs text-mute">{t('docs.replaceModel', { model: MODEL_ID, endpoint: `${BASE_URL}/models` })}</span></div>
      </section>

      <section aria-labelledby="endpoints-title" className="mb-12">
        <h2 id="endpoints-title" className="text-xl font-semibold tracking-tight mb-4">{t('docs.endpoints')}</h2>
        <ul className="space-y-3">{ENDPOINTS.map(([method, path, description]) => <li key={path} className="card p-4 flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4"><span className={`font-mono text-xs font-semibold px-2 py-1 rounded border w-fit ${method === 'POST' ? 'text-cyan-300 border-cyan-400/30 bg-cyan-500/10' : 'text-gold border-gold/30 bg-gold/10'}`}>{method}</span><code className="font-mono text-sm text-text break-all">{BASE_URL}{path}</code><span className="text-sm text-mute sm:ml-auto">{t(description as I18nKey)}</span></li>)}</ul>
      </section>

      <section aria-labelledby="capabilities-title" className="mb-12">
        <h2 id="capabilities-title" className="text-xl font-semibold tracking-tight mb-4">{t('docs.capabilities')}</h2>
        <div className="grid gap-4 md:grid-cols-2">
          <div className="card p-4"><h3 className="font-medium">{t('docs.streaming')}</h3><p className="text-sm text-mute mt-1">{t('docs.streamingP')} <code>data: &#123;...&#125;</code>{t('docs.streamingEnd')} <code>data: [DONE]</code>.</p></div>
          <div className="card p-4"><h3 className="font-medium">{t('docs.tools')}</h3><p className="text-sm text-mute mt-1">{t('docs.toolsP')} <code>tools</code>, <code>tool_choice</code>{t('docs.toolsEnd')}</p></div>
          <div className="card p-4"><h3 className="font-medium">{t('docs.image')}</h3><p className="text-sm text-mute mt-1">{t('docs.imageP')}</p></div>
          <div className="card p-4"><h3 className="font-medium">{t('docs.agents')}</h3><p className="text-sm text-mute mt-1">{t('docs.agentsP')}</p></div>
        </div>
      </section>

      <section aria-labelledby="errors-title" className="mb-12">
        <h2 id="errors-title" className="text-xl font-semibold tracking-tight mb-4">{t('docs.errors')}</h2>
        <p className="text-sm text-mute mb-4">{t('docs.errorsP')} <code>&#123;"error": &#123;"message", "type", "param", "code"&#125;&#125;</code>. {t('docs.errors429')} <code>Retry-After</code>, <code>X-RateLimit-Limit</code>, <code>X-RateLimit-Remaining</code>, dan <code>X-RateLimit-Reset</code>{t('docs.errorsRetry')}</p>
        <div className="card overflow-x-auto"><table className="w-full text-sm"><thead className="text-left text-mute"><tr><th className="p-3">{t('docs.status')}</th><th className="p-3">{t('docs.type')}</th><th className="p-3">{t('docs.meaning')}</th></tr></thead><tbody>{ERROR_CODES.map(([status, type, meaning]) => <tr key={status} className="border-t border-border"><td className="p-3 font-mono text-cyan-300">{status}</td><td className="p-3 font-mono">{type}</td><td className="p-3 text-mute">{t(meaning as I18nKey)}</td></tr>)}</tbody></table></div>
      </section>
    </>
  );
}