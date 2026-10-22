'use client';
import axios from 'axios';

export const api = {
  // Cookie sesi (HttpOnly) dikirim browser otomatis; tidak ada header
  // Authorization yang bisa dibaca atau ditiru oleh skrip halaman.
  // Relative URL: Next rewrite /api -> gateway. Absolute :8080 dies on CORS
  // when the dev origin isn't the one the gateway allow-lists.
  _url(url: string) { return url.startsWith('/api/') ? url : `/api${url}`; },
  post<T>(url: string, body: any): Promise<{ data: T }> {
    return axios.post(this._url(url), body, { withCredentials: true });
  },
  patch<T>(url: string, body: any): Promise<{ data: T }> {
    return axios.patch(this._url(url), body, { withCredentials: true });
  },
  get<T>(url: string): Promise<{ data: T }> {
    return axios.get(this._url(url), { withCredentials: true });
  },
  del(url: string): Promise<void> {
    return axios.delete(this._url(url), { withCredentials: true }).then(() => {});
  },
};
