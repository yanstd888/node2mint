'use client';
// IntegrationsSection: OpenAI-compatible API key use + client configs.
// Public OpenAI-compatible endpoint. Reuses DocsSection card/code/tabs/copy patterns.
import { useMemo, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Copy, Check } from 'lucide-react';
import { useT, type Key } from '@/lib/i18n';
import { useToast } from '@/components/ui/toast';

type IntKey = {
  label: Key;
  warnKey?: Key;
  code: string;
  copyLabel: Key;
  fileHint?: Key;
};

const PROD_BASE = 'https://www.node2mint.com/v1';

const INT_CONFIGS: (IntKey & { icon: React.ElementType; id: string })[] = [
  {
    id: 'curl',
    icon: Copy,
    label: 'docs.intLocal',
    code: `curl ${PROD_BASE}/models \\\n  -H "Authorization: Bearer \$NODE2MINT_KEY"`,
    copyLabel: 'docs.copyCode',
  },
  {
    id: 'hermes',
    icon: Copy,
    label: 'docs.intHermes',
    code: `# ~/.hermes/config.yaml (provider entry)\nmodel:\n  provider: custom\n  base_url: ${PROD_BASE}\n  api_key: $NODE2MINT_KEY\n  default: n2m-3-0-Pro`,
    copyLabel: 'docs.copyCode',
    fileHint: 'docs.intHermesFile',
  },
  {
    id: 'codex',
    icon: Copy,
    label: 'docs.intCodex',
    code: `# ~/.codex/config.toml (OpenAI-compatible provider)\nmodel_provider = "node2mint"\n[[model_providers.node2mint]]\nname = "Node2Mint"\nbase_url = "${PROD_BASE}"\nenv_key = "NODE2MINT_KEY"\nwire_api = "chat"  # /v1/chat/completions\nmodel = "n2m-3-0-Pro"`,
    copyLabel: 'docs.copyCode',
    fileHint: 'docs.intCodexFile',
  },
  {
    id: 'opencode',
    icon: Copy,
    label: 'docs.intOpenCode',
    code: `{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "node2mint": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "Node2Mint",
      "options": {
        "baseURL": "${PROD_BASE}",
        "apiKey": "{env:NODE2MINT_KEY}"
      },
      "models": {
        "n2m-3-0-Pro": { "name": "n2m-3-0-Pro" }
      }
    }
  }
`,
    copyLabel: 'docs.copyCode',
    fileHint: 'docs.intOpenCodeFile',
  },
  {
    id: 'claude',
    icon: Copy,
    label: 'docs.intClaude',
    code: `# Claude Code (env vars)\nexport ANTHROPIC_BASE_URL="${PROD_BASE}"\nexport ANTHROPIC_API_KEY="$NODE2MINT_KEY"\n# Claude Code CLI: claude --model <model-id> (select model from GET /models)`,
    copyLabel: 'docs.copyCode',
    fileHint: 'docs.intClaudeFile',
  },
  {
    id: 'cursor',
    icon: Copy,
    label: 'docs.intCursor',
    code: `// ~/.cursor/settings.json (OpenAI-compatible provider override)
{
  "openai.baseUrl": "${PROD_BASE}",
  "openai.apiKey": "env:NODE2MINT_KEY",
  "openai.model": "n2m-3-0-Pro"
}`,
    copyLabel: 'docs.copyCode',
    fileHint: 'docs.intCursorFile',
  },
];

export function IntegrationsSection() {
  const { t } = useT();
  const toast = useToast();
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const configs = useMemo(() => INT_CONFIGS, []);

  const salin = async (id: string, code: string) => {
    try {
      await navigator.clipboard.writeText(code);
      setCopiedId(id);
      setTimeout(() => setCopiedId(null), 1600);
      toast.ok(t('docs.copyCode'));
    } catch {
      toast.err(t('docs.copyFail'));
    }
  };

  return (
    <section aria-labelledby="integrations-title" className="mb-12">
      <header className="mb-8">
        <p className="label mb-2 flex items-center gap-2">
          <Copy className="w-3.5 h-3.5" aria-hidden="true" /> {t('docs.intKick')}
        </p>
        <h2 id="integrations-title" className="text-2xl font-semibold tracking-tight">{t('docs.intKick')}</h2>
        <p className="text-sm text-mute mt-2 max-w-prose">{t('docs.intSub')}</p>
      </header>

      {/* Canonical public endpoint */}
      <div className="mb-8">
        <div className="card saldo p-5 border-gold/30 bg-gold/5">
          <div className="flex items-start gap-3">
            <Copy className="w-5 h-5 text-gold/70 shrink-0 mt-0.5" aria-hidden="true" />
            <div className="flex-1">
              <h3 className="font-medium text-gold">{t('docs.intLocal')}</h3>
              <p className="text-sm text-gold mt-1 leading-relaxed">{t('docs.intLocalWarn')}</p>
              <div className="mt-3">
                <span className="label">{t('docs.keyHint')}</span>
                <code className="font-mono text-sm text-cyan-300 select-all break-all block mt-1">{PROD_BASE}</code>
                <span className="text-xs text-mute mt-1 block">{t('docs.intLocalKey')}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Client configs */}
      <div className="grid gap-6">
        {configs.map((c) => (
          <div key={c.id} className="card p-5">
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 mb-3">
              <h3 className="font-medium text-sm flex items-center gap-2">
                <c.icon className="w-4 h-4 text-cyan-300" aria-hidden="true" />
                {t(c.label)}
              </h3>
              <Button variant="outline" size="sm" onClick={() => salin(c.id, c.code)}>
                {copiedId === c.id ? <Check className="w-3.5 h-3.5 text-emerald-300" aria-hidden="true" /> : <Copy className="w-3.5 h-3.5" aria-hidden="true" />}
                {copiedId === c.id ? t('models.copied') : t(c.copyLabel)}
              </Button>
            </div>
            {c.warnKey && (
              <p role="alert" className="text-xs text-gold bg-gold/10 border border-gold/30 rounded-lg px-3 py-2 mb-2">
                {t(c.warnKey)}
              </p>
            )}
            {c.fileHint && (
              <p className="text-xs text-mute mb-2">{t(c.fileHint)}</p>
            )}
            <pre className="bg-bg border border-border rounded-lg p-4 overflow-x-auto text-sm leading-relaxed mt-2">
              <code className="block w-full min-w-[480px] font-mono text-cyan-100 whitespace-pre">{c.code}</code>
            </pre>
          </div>
        ))}
      </div>

      {/* Troubleshooting */}
      <div className="mt-10">
        <h3 className="text-xl font-semibold tracking-tight mb-4">{t('docs.troubleshooting')}</h3>
        <div className="space-y-4">
          <div className="card p-4">
            <h4 className="font-medium text-sm text-gold">{t('docs.trLocal')}</h4>
            <p className="text-sm text-mute mt-1">{t('docs.trLocalMsg')}</p>
          </div>
          <div className="card p-4">
            <h4 className="font-medium text-sm text-gold">{t('docs.tr401')}</h4>
            <p className="text-sm text-mute mt-1">{t('docs.tr401Msg')}</p>
          </div>
          <div className="card p-4">
            <h4 className="font-medium text-sm text-gold">{t('docs.tr429')}</h4>
            <p className="text-sm text-mute mt-1">{t('docs.tr429Msg')}</p>
          </div>
          <div className="card p-4">
            <h4 className="font-medium text-sm text-gold">{t('docs.trModel')}</h4>
            <p className="text-sm text-mute mt-1">{t('docs.trModelMsg')}</p>
          </div>
        </div>
      </div>

      {/* Production endpoint note */}
      <div className="mt-8 card p-4 border-border bg-surface-hi/30">
        <p className="text-xs text-mute">
          {t('docs.intProd')}: <code className="font-mono text-cyan-300 select-all">{PROD_BASE}</code> {t('docs.intProdHint')}
        </p>
      </div>
    </section>
  );
}
