'use client';

import { useEffect, useRef } from 'react';
import { Bold, Heading2, Heading3, Italic, List, ListOrdered, Quote, RemoveFormatting, Underline } from 'lucide-react';
import { Button } from '@/components/ui/button';

type Props = { id: string; value: string; onChange: (value: string) => void; required?: boolean };

const actions = [
  ['bold', Bold, 'Tebal'], ['italic', Italic, 'Miring'], ['underline', Underline, 'Garis bawah'],
  ['formatBlock:h2', Heading2, 'Judul bagian'], ['formatBlock:h3', Heading3, 'Subjudul'],
  ['insertUnorderedList', List, 'Daftar poin'], ['insertOrderedList', ListOrdered, 'Daftar bernomor'],
  ['formatBlock:blockquote', Quote, 'Kutipan'], ['removeFormat', RemoveFormatting, 'Hapus format'],
] as const;

export function RichTextEditor({ id, value, onChange, required }: Props) {
  const editor = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (editor.current && document.activeElement !== editor.current && editor.current.innerHTML !== value) {
      editor.current.innerHTML = value;
    }
  }, [value]);

  const command = (action: string) => {
    const [name, arg] = action.split(':');
    editor.current?.focus();
    document.execCommand(name, false, arg || undefined);
    onChange(editor.current?.innerHTML || '');
  };

  return <div className="overflow-hidden rounded-xl border border-border bg-surface-hi focus-within:ring-2 focus-within:ring-cyan-400/60">
    <div className="flex flex-wrap gap-1 border-b border-border p-2" role="toolbar" aria-label="Format artikel">
      {actions.map(([action, Icon, label]) => <Button key={action} type="button" size="sm" variant="ghost" className="h-8 w-8 px-0" aria-label={label} title={label} onMouseDown={(e) => e.preventDefault()} onClick={() => command(action)}><Icon className="h-4 w-4" /></Button>)}
    </div>
    <div id={id} ref={editor} contentEditable suppressContentEditableWarning role="textbox" aria-multiline="true" aria-required={required} tabIndex={0} onInput={() => onChange(editor.current?.innerHTML || '')} className="min-h-[280px] px-3 py-3 text-sm leading-relaxed outline-none [&_blockquote]:border-l-2 [&_blockquote]:border-cyan-400/60 [&_blockquote]:pl-3 [&_h2]:mb-3 [&_h2]:mt-5 [&_h2]:text-xl [&_h2]:font-semibold [&_h3]:mb-2 [&_h3]:mt-4 [&_h3]:font-semibold [&_ol]:my-3 [&_ol]:list-decimal [&_ol]:pl-6 [&_p]:my-2 [&_ul]:my-3 [&_ul]:list-disc [&_ul]:pl-6" />
  </div>;
}
