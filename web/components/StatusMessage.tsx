'use client';
// Satu tampilan untuk 404 / error route / error global supaya tiga berkas tidak
// menjadi tiga variasi markup yang berbeda.
import { ReactNode } from 'react';
import { AlertTriangle, FileQuestion } from 'lucide-react';

export default function StatusMessage({ code, title, body, children, danger = false }: {
  code: string; title: string; body: string; children?: ReactNode; danger?: boolean;
}) {
  const Icon = danger ? AlertTriangle : FileQuestion;
  return (
    <main className="grid min-h-dvh place-items-center p-6">
      <div className="card w-full max-w-md p-10 text-center">
        <Icon className={`mx-auto mb-4 h-10 w-10 ${danger ? 'text-danger' : 'text-mute'}`} aria-hidden="true" />
        <p className="label mb-2">{code}</p>
        <h1 className="text-xl font-semibold tracking-tight">{title}</h1>
        <p className="mt-2 text-sm leading-6 text-mute">{body}</p>
        {children && <div className="mt-6 flex flex-wrap items-center justify-center gap-2">{children}</div>}
      </div>
    </main>
  );
}
