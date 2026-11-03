"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

export type Account = {
  id: string;
  email: string;
  tenant_id: string;
  is_owner: boolean;
  free_call_limit: number;
  calls_used: number;
};

type SessionState = {
  account: Account | null;
  loading: boolean;
  error: string;
  refresh: () => Promise<void>;
  logout: () => Promise<void>;
};

const SessionContext = createContext<SessionState | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [account, setAccount] = useState<Account | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(async (showLoading = false) => {
    if (showLoading) setLoading(true);
    try {
      const res = await fetch(`${API}/api/auth/me`, {
        credentials: "include",
        cache: "no-store",
      });
      if (res.status === 401) {
        setAccount(null);
        setError("");
        return;
      }
      if (!res.ok) {
        setError("The console API is not responding.");
        return;
      }
      const data = (await res.json()) as { account?: Account };
      setAccount(data.account ?? null);
      setError("");
    } catch {
      setError("Cannot reach the console API. Is the Go service running?");
    } finally {
      if (showLoading) setLoading(false);
    }
  }, []);

  useEffect(() => {
    load(true);
  }, [load]);

  const refresh = useCallback(async () => {
    await load(false);
  }, [load]);

  const logout = useCallback(async () => {
    try {
      await fetch(`${API}/api/auth/logout`, {
        method: "POST",
        credentials: "include",
      });
    } catch {}
    setAccount(null);
    window.location.assign("/login");
  }, []);

  return (
    <SessionContext.Provider
      value={{ account, loading, error, refresh, logout }}
    >
      {children}
    </SessionContext.Provider>
  );
}

export function useSession(): SessionState {
  const value = useContext(SessionContext);
  if (!value) {
    throw new Error("useSession must be used inside SessionProvider");
  }
  return value;
}
