"use client";

import { useEffect, useState } from "react";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

export type Turn = { role: string; text: string };

export type Call = {
  id: string;
  tenant: string;
  lang: string;
  state: string;
  last_text: string;
  escalated: boolean;
  reason: string;
  taken_over: boolean;
  transcript: Turn[] | null;
  started_at: string;
};

export function useSupervisor() {
  const [calls, setCalls] = useState<Call[]>([]);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const response = await fetch(`${API}/api/calls/live`, { credentials: "include" });
        if (!response.ok) {
          if (!cancelled) setConnected(false);
          return;
        }
        const data = (await response.json()) as Call[];
        if (!cancelled) {
          setCalls(Array.isArray(data) ? data : []);
          setConnected(true);
        }
      } catch {
        if (!cancelled) setConnected(false);
      }
    };
    void load();
    const timer = setInterval(load, 2000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, []);

  return { calls, connected };
}
