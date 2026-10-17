"use client";

import { useCallback, useEffect, useRef, useState } from "react";

const WS_URL = process.env.NEXT_PUBLIC_WS_URL ?? "ws://localhost:8090/ws";

export function useSwitchboard() {
  const [running, setRunning] = useState(false);
  const [status, setStatus] = useState("idle");
  const [rttMs, setRttMs] = useState<number | null>(null);

  const wsRef = useRef<WebSocket | null>(null);
  const ctxRef = useRef<AudioContext | null>(null);
  const nodeRef = useRef<AudioWorkletNode | null>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const pingTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const seqRef = useRef(0);
  const inflightRef = useRef<Map<number, number>>(new Map());

  const onMessage = useCallback((e: MessageEvent) => {
    if (typeof e.data !== "string") return;
    const msg = JSON.parse(e.data) as { seq?: number };
    if (typeof msg.seq !== "number") return;
    const t0 = inflightRef.current.get(msg.seq);
    if (t0 === undefined) return;
    inflightRef.current.delete(msg.seq);
    setRttMs(performance.now() - t0);
  }, []);

  const connect = useCallback(() => {
    const ws = new WebSocket(WS_URL);
    ws.binaryType = "arraybuffer";
    ws.onopen = () => {
      setStatus("socket: open");
      pingTimerRef.current = setInterval(() => {
        if (ws.readyState !== WebSocket.OPEN) return;
        const s = seqRef.current++;
        inflightRef.current.set(s, performance.now());
        ws.send(JSON.stringify({ type: "ping", seq: s }));
      }, 1000);
    };
    ws.onclose = () => setStatus("socket: closed");
    ws.onerror = () => setStatus("socket: error");
    ws.onmessage = onMessage;
    wsRef.current = ws;
  }, [onMessage]);

  const startAudio = useCallback(async () => {
    const stream = await navigator.mediaDevices.getUserMedia({
      audio: {
        channelCount: 1,
        echoCancellation: false,
        noiseSuppression: false,
        autoGainControl: false,
      },
    });

    const ctx = new AudioContext({ sampleRate: 16000 });
    await ctx.audioWorklet.addModule("/audio-worklet.js");

    const source = ctx.createMediaStreamSource(stream);
    const node = new AudioWorkletNode(ctx, "pcm-capture");
    node.port.onmessage = (e: MessageEvent<Int16Array>) => {
      const ws = wsRef.current;
      if (ws && ws.readyState === WebSocket.OPEN) ws.send(e.data as unknown as ArrayBuffer);
    };
    source.connect(node);

    streamRef.current = stream;
    ctxRef.current = ctx;
    nodeRef.current = node;
  }, []);

  const stopAudio = useCallback(async () => {
    nodeRef.current?.disconnect();
    streamRef.current?.getTracks().forEach((t) => t.stop());
    await ctxRef.current?.close();
    nodeRef.current = null;
    ctxRef.current = null;
    streamRef.current = null;
  }, []);

  const toggle = useCallback(async () => {
    if (running) {
      await stopAudio();
      setRunning(false);
      setStatus("idle");
      setRttMs(null);
      return;
    }
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) connect();
    await startAudio();
    setRunning(true);
    setStatus("streaming");
  }, [running, connect, startAudio, stopAudio]);

  useEffect(() => {
    return () => {
      if (pingTimerRef.current) clearInterval(pingTimerRef.current);
      wsRef.current?.close();
      ctxRef.current?.close();
      streamRef.current?.getTracks().forEach((t) => t.stop());
    };
  }, []);

  return { running, status, rttMs, toggle };
}
