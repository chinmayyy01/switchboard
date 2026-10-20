"use client";

import { useEffect, useState } from "react";

const SUP_URL =
  process.env.NEXT_PUBLIC_SUPERVISOR_WS_URL ??
  "ws://localhost:8090/ws/supervisor";
const TOKEN = process.env.NEXT_PUBLIC_SUPERVISOR_TOKEN ?? "";

export function withToken(url: string): string {
  if (!TOKEN) return url;
  return `${url}${url.includes("?") ? "&" : "?"}token=${encodeURIComponent(TOKEN)}`;
}

export type Turn = { role: string; text: string };

export type Call = {
  id: string;
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
    const ws = new WebSocket(withToken(SUP_URL));
    ws.onopen = () => setConnected(true);
    ws.onclose = () => setConnected(false);
    ws.onerror = () => setConnected(false);
    ws.onmessage = (e) => {
      if (typeof e.data !== "string") return;
      setCalls(JSON.parse(e.data) as Call[]);
    };
    return () => ws.close();
  }, []);

  return { calls, connected };
}
