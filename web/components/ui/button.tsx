import { ReactNode } from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/utils';

const buttonVariants = cva(
  'inline-flex items-center justify-center gap-2 rounded-xl text-sm font-medium transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/60 focus-visible:ring-offset-2 focus-visible:ring-offset-bg active:scale-[0.98] disabled:pointer-events-none disabled:opacity-50',
  {
    variants: {
      variant: {
        default: 'bg-cyan-500 text-bg hover:bg-cyan-400',
        destructive: 'bg-danger text-text hover:bg-danger/85',
        outline: 'border border-border bg-transparent text-text hover:bg-surface-hi',
        ghost: 'bg-transparent text-mute hover:bg-surface-hi hover:text-text',
      },
      size: { default: 'h-10 px-4 py-2', sm: 'h-8 px-3', lg: 'h-12 px-6' },
    },
    defaultVariants: { variant: 'default', size: 'default' },
  },
);

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> {}

const Button = ({ className, variant, size, ...props }: ButtonProps) => (
  <button className={cn(buttonVariants({ variant, size, className }))} {...props} />
);
Button.displayName = 'Button';
export { Button, buttonVariants };
