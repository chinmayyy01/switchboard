"use client";

import { useCallback, useEffect, useRef, useState } from "react";

const SUP_URL =
  process.env.NEXT_PUBLIC_SUPERVISOR_WS_URL ??
  "ws://localhost:8090/ws/supervisor";
const TOKEN = process.env.NEXT_PUBLIC_SUPERVISOR_TOKEN ?? "";
const SAMPLE_RATE = 16000;

export function useBridge(id: string) {
  const [status, setStatus] = useState("connecting");
  const wsRef = useRef<WebSocket | null>(null);
  const ctxRef = useRef<AudioContext | null>(null);
  const nodeRef = useRef<AudioWorkletNode | null>(null);
  const streamRef = useRef<MediaStream | null>(null);

  const play = useCallback((buf: ArrayBuffer) => {
    const ctx = ctxRef.current;
    if (!ctx) return;
    const i16 = new Int16Array(buf);
    if (i16.length === 0) return;
    const f32 = new Float32Array(i16.length);
    for (let i = 0; i < i16.length; i++) f32[i] = i16[i] / 32768;
    const audio = ctx.createBuffer(1, f32.length, SAMPLE_RATE);
    audio.copyToChannel(f32, 0);
    const src = ctx.createBufferSource();
    src.buffer = audio;
    src.connect(ctx.destination);
    src.start();
  }, []);

  useEffect(() => {
    if (!id) return;
    let closed = false;
    const token = TOKEN ? `&token=${encodeURIComponent(TOKEN)}` : "";
    const ws = new WebSocket(`${SUP_URL}/call?id=${encodeURIComponent(id)}${token}`);
    ws.binaryType = "arraybuffer";
    wsRef.current = ws;

    ws.onopen = async () => {
      setStatus("connected");
      try {
        const stream = await navigator.mediaDevices.getUserMedia({
          audio: {
            channelCount: 1,
            echoCancellation: true,
            noiseSuppression: true,
            autoGainControl: true,
          },
        });
        const ctx = new AudioContext({ sampleRate: SAMPLE_RATE });
        await ctx.audioWorklet.addModule("/audio-worklet.js");
        const source = ctx.createMediaStreamSource(stream);
        const node = new AudioWorkletNode(ctx, "pcm-capture");
        node.port.onmessage = (e: MessageEvent<Int16Array>) => {
          if (ws.readyState === WebSocket.OPEN) {
            ws.send(e.data as unknown as ArrayBuffer);
          }
        };
        source.connect(node);
        streamRef.current = stream;
        ctxRef.current = ctx;
        nodeRef.current = node;
      } catch {
        setStatus("mic denied");
      }
    };
    ws.onclose = () => {
      if (!closed) setStatus("closed");
    };
    ws.onerror = () => setStatus("error");
    ws.onmessage = (e) => {
      if (typeof e.data !== "string") play(e.data as ArrayBuffer);
    };

    return () => {
      closed = true;
      ws.close();
      nodeRef.current?.disconnect();
      streamRef.current?.getTracks().forEach((t) => t.stop());
      ctxRef.current?.close();
    };
  }, [id, play]);

  return { status };
}
