import { cn } from '@/lib/utils';

export interface TextareaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {}

const Textarea = ({ className, ...props }: TextareaProps) => (
  <textarea
    className={cn(
      'flex min-h-[88px] w-full rounded-xl border border-slate-500 bg-surface-hi px-3 py-2 text-sm text-white placeholder-slate-400 focus:border-cyan-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60',
      className,
    )}
    {...props}
  />
);
Textarea.displayName = 'Textarea';
export { Textarea };
