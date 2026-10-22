import { cn } from '@/lib/utils';

export interface SelectProps extends React.SelectHTMLAttributes<HTMLSelectElement> {}

const Select = ({ className, ...props }: SelectProps) => (
  <select
    className={cn(
      'flex h-10 w-full rounded-xl border border-slate-500 bg-surface-hi px-3 py-2 text-sm text-white focus:border-cyan-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60',
      className,
    )}
    {...props}
  />
);
Select.displayName = 'Select';
export { Select };
