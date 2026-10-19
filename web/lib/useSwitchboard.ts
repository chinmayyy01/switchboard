"use client";

import { useCallback, useEffect, useRef, useState } from "react";

const WS_URL = process.env.NEXT_PUBLIC_WS_URL ?? "ws://localhost:8090/ws";
const SAMPLE_RATE = 16000;

type ServerEvent = {
  type?: string;
  seq?: number;
  text?: string;
  state?: string;
  name?: string;
  lang?: string;
  stt_ms?: number;
  brain_ms?: number;
  tts_ms?: number;
  turn_ms?: number;
};

export function useSwitchboard() {
  const [inCall, setInCall] = useState(false);
  const [status, setStatus] = useState("idle");
  const [rttMs, setRttMs] = useState<number | null>(null);
  const [transcript, setTranscript] = useState("");
  const [agentText, setAgentText] = useState("");
  const [agentState, setAgentState] = useState("");
  const [agentLang, setAgentLang] = useState("");
  const [tool, setTool] = useState("");
  const [latency, setLatency] = useState<{
    stt: number;
    brain: number;
    tts: number;
    turn: number;
  } | null>(null);

  const wsRef = useRef<WebSocket | null>(null);
  const ctxRef = useRef<AudioContext | null>(null);
  const nodeRef = useRef<AudioWorkletNode | null>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const pingTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const streamingRef = useRef(false);
  const mutedRef = useRef(false);
  const seqRef = useRef(0);
  const inflightRef = useRef<Map<number, number>>(new Map());

  const playPCM = useCallback((buf: ArrayBuffer) => {
    const ctx = ctxRef.current;
    if (!ctx) return;
    const i16 = new Int16Array(buf);
    if (i16.length === 0) return;

    const f32 = new Float32Array(i16.length);
    for (let i = 0; i < i16.length; i++) f32[i] = i16[i] / 32768;
    const audio = ctx.createBuffer(1, f32.length, SAMPLE_RATE);
    audio.copyToChannel(f32, 0);

    mutedRef.current = true;
    setStatus("agent speaking…");

    const src = ctx.createBufferSource();
    src.buffer = audio;
    src.connect(ctx.destination);
    src.onended = () => {
      mutedRef.current = false;
      setStatus("listening…");
    };
    src.start();
  }, []);

  const onMessage = useCallback(
    (e: MessageEvent) => {
      if (typeof e.data !== "string") {
        playPCM(e.data as ArrayBuffer);
        return;
      }
      const msg = JSON.parse(e.data) as ServerEvent;

      if (typeof msg.seq === "number") {
        const t0 = inflightRef.current.get(msg.seq);
        if (t0 !== undefined) {
          inflightRef.current.delete(msg.seq);
          setRttMs(performance.now() - t0);
        }
      }

      switch (msg.type) {
        case "stt.final":
          setTranscript(msg.text ?? "");
          setAgentLang(msg.lang ?? "");
          break;
        case "agent.text":
          setAgentText(msg.text ?? "");
          break;
        case "agent.state":
          setAgentState(msg.state ?? "");
          break;
        case "tool.call":
          setTool(msg.name ?? "");
          break;
        case "latency":
          setLatency({
            stt: msg.stt_ms ?? 0,
            brain: msg.brain_ms ?? 0,
            tts: msg.tts_ms ?? 0,
            turn: msg.turn_ms ?? 0,
          });
          break;
        case "agent.speaking":
          mutedRef.current = true;
          break;
      }
    },
    [playPCM],
  );

  const connect = useCallback((): Promise<WebSocket> => {
    return new Promise((resolve) => {
      const ws = new WebSocket(WS_URL);
      ws.binaryType = "arraybuffer";
      ws.onopen = () => {
        setStatus("connected");
        pingTimerRef.current = setInterval(() => {
          if (ws.readyState !== WebSocket.OPEN) return;
          const s = seqRef.current++;
          inflightRef.current.set(s, performance.now());
          ws.send(JSON.stringify({ type: "ping", seq: s }));
        }, 1000);
        resolve(ws);
      };
      ws.onclose = () => setStatus("closed");
      ws.onerror = () => setStatus("error");
      ws.onmessage = onMessage;
      wsRef.current = ws;
    });
  }, [onMessage]);

  const ensureAudio = useCallback(async () => {
    if (ctxRef.current) return;
    const stream = await navigator.mediaDevices.getUserMedia({
      audio: {
        channelCount: 1,
        echoCancellation: false,
        noiseSuppression: false,
        autoGainControl: false,
      },
    });
    const ctx = new AudioContext({ sampleRate: SAMPLE_RATE });
    await ctx.audioWorklet.addModule("/audio-worklet.js");
    const source = ctx.createMediaStreamSource(stream);
    const node = new AudioWorkletNode(ctx, "pcm-capture");
    node.port.onmessage = (e: MessageEvent<Int16Array>) => {
      const ws = wsRef.current;
      if (!streamingRef.current || mutedRef.current) return;
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(e.data as unknown as ArrayBuffer);
      }
    };
    source.connect(node);
    streamRef.current = stream;
    ctxRef.current = ctx;
    nodeRef.current = node;
  }, []);

  const startCall = useCallback(
    async (lang: string) => {
      await ensureAudio();
      await ctxRef.current?.resume();
      let ws = wsRef.current;
      if (!ws || ws.readyState !== WebSocket.OPEN) ws = await connect();
      ws.send(JSON.stringify({ type: "call.start", lang }));
      streamingRef.current = true;
      setInCall(true);
      setStatus("listening…");
    },
    [connect, ensureAudio],
  );

  const endCall = useCallback(async () => {
    streamingRef.current = false;
    mutedRef.current = false;
    setInCall(false);
    if (pingTimerRef.current) clearInterval(pingTimerRef.current);
    wsRef.current?.close();
    wsRef.current = null;
    nodeRef.current?.disconnect();
    streamRef.current?.getTracks().forEach((t) => t.stop());
    await ctxRef.current?.close();
    nodeRef.current = null;
    ctxRef.current = null;
    streamRef.current = null;
    setStatus("idle");
    setTranscript("");
    setAgentText("");
    setAgentState("");
    setAgentLang("");
    setTool("");
    setLatency(null);
  }, []);

  useEffect(() => {
    return () => {
      if (pingTimerRef.current) clearInterval(pingTimerRef.current);
      wsRef.current?.close();
      ctxRef.current?.close();
      streamRef.current?.getTracks().forEach((t) => t.stop());
    };
  }, []);

  return {
    inCall,
    status,
    rttMs,
    transcript,
    agentText,
    agentState,
    agentLang,
    tool,
    latency,
    startCall,
    endCall,
  };
}
