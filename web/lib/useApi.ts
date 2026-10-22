'use client';
// Satu pola fetch untuk semua halaman: loading / err / reload. Dipakai ~20 halaman;
// menulis ulang try/catch+state di tiap file hanya menambah baris yang sama.
import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '@/lib/api';
import { pesanError } from '@/lib/http';

// `initial` = data awal dari server component (katalog homepage ikut HTML pertama
// supaya crawler melihat nama model). Kalau ada data awal: tanpa skeleton dan
// refresh pertama berjalan senyap, jadi isi yang sudah dirender tidak berkedip.
export function useFetch<T>(url: string, skip = false, pollMs = 0, initial: T | null = null) {
  const [data, setData] = useState<T | null>(initial);
  const [loading, setLoading] = useState(!skip && initial === null);
  const [err, setErr] = useState('');
  const seeded = useRef(initial !== null);
  // Ada isi = jangan tampilkan error. Refresh senyap (awal ter-seed / poll 60
  // detik) yang gagal sesaat tidak boleh menimpa katalog yang sudah dirender
  // dengan "belum ada data" palsu.
  const adaIsi = useRef(initial !== null);

  const load = useCallback(async (silent = false) => {
    if (!silent) setLoading(true);
    try {
      const res = await api.get<T>(url);
      setData(res.data);
      adaIsi.current = true;
      setErr('');
    } catch (e) {
      // Tanpa catch: gagal muat tampil sebagai "belum ada data" (empty state palsu).
      if (!adaIsi.current) setErr(pesanError(e, 'Could not load this. Check the connection and reload.'));
    } finally {
      if (!silent) setLoading(false);
    }
  }, [url]);

  useEffect(() => {
    if (skip) return;
    void load(seeded.current);
    if (pollMs <= 0) return;
    const timer = window.setInterval(() => { void load(true); }, pollMs);
    return () => window.clearInterval(timer);
  }, [load, pollMs, skip]);

  return { data, loading, err, reload: () => load(), setData };
}
