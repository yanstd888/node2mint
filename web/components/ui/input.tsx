import { ReactNode } from 'react';
import { cn } from '@/lib/utils';

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {}

const Input = ({ className, ...props }: InputProps) => (
  <input
    className={cn(
      // placeholder-slate-400 (5.9:1) & border-slate-500 (3.2:1) — slate-500/slate-600 lama
      // hanya 3.17:1 & 2.1:1 terhadap bg input, di bawah ambang WCAG AA/1.4.11.
      'flex h-10 w-full rounded-xl border border-slate-500 bg-surface-hi px-3 py-2 text-base text-white placeholder-slate-400 focus:border-cyan-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60 sm:text-sm',
      className,
    )}
    {...props}
  />
);
Input.displayName = 'Input';
export { Input };
