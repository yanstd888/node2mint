'use client';
// PATCH/PUT + pembaca pesan error. lib/api.ts dipakai bersama (read-only untuk tugas ini)
// dan hanya punya get/post/del, jadi dua verb sisanya hidup di sini.
import axios from 'axios';
import { apiErrorMessage } from '@/lib/api-errors';

const opts = { withCredentials: true } as const;

const urlOf = (url: string) => (url.startsWith('/api/') ? url : `/api${url}`);

export const patch = <T,>(url: string, body: unknown) =>
  axios.patch<T>(urlOf(url), body, opts);

export const post = <T,>(url: string, body: unknown, headers?: Record<string, string>) =>
  axios.post<T>(urlOf(url), body, { ...opts, headers });

type ErrorData = {
  error?: unknown;
  code?: unknown;
  message?: unknown;
};

export const apiErrorCode = (e: unknown): string => {
  const d = (e as { response?: { data?: unknown } })?.response?.data;
  if (!d || typeof d !== 'object') return '';
  const body = d as ErrorData;
  const nested = body.error && typeof body.error === 'object' ? body.error as ErrorData : undefined;
  const code = nested?.code ?? body.code;
  return typeof code === 'string' ? code.trim().toLowerCase() : '';
};

// Gateway errors are rendered from the stable code, never from server copy.
export const pesanError = (e: unknown, fallback: string) => {
  const code = apiErrorCode(e);
  return code ? apiErrorMessage(code, fallback) : fallback;
};
