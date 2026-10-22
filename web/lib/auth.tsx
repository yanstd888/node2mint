'use client';
import { createContext, useContext, useEffect, useState, ReactNode } from 'react';
import { api } from '@/lib/api';

type Dashboard = { user_id?: string; email?: string; name?: string; role?: string; balance?: number; tokenRemaining?: number };

type User = { id: string; email: string; name: string; role: string; balance: number; tokenRemaining: number };

const AuthCtx = createContext<{
  user: User | null;
  loading: boolean;
  login: (email: string, password: string, turnstileToken?: string, totp?: string) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<User | null>;
}>({ user: null, loading: true, login: async () => {}, logout: async () => {}, refresh: async () => null });

// Sesi hidup di cookie HttpOnly yang dipasang gateway (aisell_session). Skrip
// halaman tidak pernah melihat tokennya, jadi XSS tidak bisa mencurikannya —
// dulu token disimpan di localStorage dan itu justru target utamanya.
// api.ts sudah mengatur axios.defaults.withCredentials = true secara global.

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  const load = async (): Promise<User | null> => {
    try {
      const { data } = await api.get<Dashboard>('/dashboard');
      const u = {
        id: data?.user_id || '', email: data?.email || '', name: data?.name || '', role: data?.role || '',
        balance: data?.balance ?? 0, tokenRemaining: data?.tokenRemaining ?? 0,
      };
      setUser(u);
      setLoading(false);
      return u;
    } catch {
      setUser(null);
      setLoading(false);
      return null;
    }
  };

  useEffect(() => { load(); }, []);

  const login = async (email: string, password: string, turnstileToken?: string, totp?: string) => {
    // Cookie HttpOnly dipasang oleh balasan login ini sendiri.
    // `totp` wajib kalau akun sudah mengaktifkan 2FA; backend balas 401
    // "kode 2FA diperlukan" tanpa field ini.
    await api.post('/auth/login', {
      email,
      password,
      turnstile_token: turnstileToken || undefined,
      totp: totp || undefined,
    });
    // Ambil user asli (saldo/role/id) dari /api/dashboard. Dulu di sini balance dipatok 0
    // sehingga sidebar sempat menampilkan "Saldo Rp0" sampai refresh.
    await load();
  };

  const logout = async () => {
    try { await api.post('/auth/logout', {}); } catch { /* sesi lokal tetap dibuang */ }
    setUser(null);
  };

  // Tidak memblokir render: AppShell yang menampilkan skeleton saat loading.
  return <AuthCtx.Provider value={{ user, loading, login, logout, refresh: load }}>{children}</AuthCtx.Provider>;
}

export const useAuth = () => useContext(AuthCtx);
