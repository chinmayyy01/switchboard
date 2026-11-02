"use client";

import { useCallback, useEffect, useRef, useState } from "react";

const WS_URL = process.env.NEXT_PUBLIC_WS_URL ?? "ws://localhost:8090/ws";
const SAMPLE_RATE = 16000;
const PLAYBACK_RATE = 22050;
const BARGE_RMS = 800;
const BARGE_GRACE_MS = 250;

type ServerEvent = {
  type?: string;
  seq?: number;
  text?: string;
  state?: string;
  name?: string;
  args?: Record<string, unknown>;
  reason?: string;
  lang?: string;
  stt_ms?: number;
  brain_ms?: number;
  tts_ms?: number;
  first_audio_ms?: number;
  turn_ms?: number;
  docs?: string[];
};

export type Booking = {
  service: string;
  address: string;
  date: string;
  window: string;
  phone: string;
  booked: boolean;
};

const emptyBooking: Booking = {
  service: "",
  address: "",
  date: "",
  window: "",
  phone: "",
  booked: false,
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
  const [citations, setCitations] = useState<string[]>([]);
  const [booking, setBooking] = useState<Booking>(emptyBooking);
  const [level, setLevel] = useState(0);
  const [sampleRate, setSampleRate] = useState(0);
  const [devices, setDevices] = useState<{ deviceId: string; label: string }[]>([]);
  const [latency, setLatency] = useState<{
    stt: number;
    brain: number;
    tts: number;
    firstAudio: number;
    turn: number;
  } | null>(null);

  const wsRef = useRef<WebSocket | null>(null);
  const ctxRef = useRef<AudioContext | null>(null);
  const playCtxRef = useRef<AudioContext | null>(null);
  const nodeRef = useRef<AudioWorkletNode | null>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const pingTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const streamingRef = useRef(false);
  const speakingRef = useRef(false);
  const interruptedRef = useRef(false);
  const sourcesRef = useRef<Set<AudioBufferSourceNode>>(new Set());
  const nextStartRef = useRef(0);
  const doneRef = useRef(false);
  const graceUntilRef = useRef(0);
  const lastLevelRef = useRef(0);
  const deviceIdRef = useRef("");
  const tenantRef = useRef("rivera");
  const seqRef = useRef(0);
  const inflightRef = useRef<Map<number, number>>(new Map());

  const interrupt = useCallback(() => {    interruptedRef.current = true;
    for (const src of sourcesRef.current) {
      try {
        src.stop();
      } catch {
        /* already stopped */
      }
    }
    sourcesRef.current.clear();
    nextStartRef.current = 0;
    doneRef.current = false;
    speakingRef.current = false;
    setStatus("listening…");
    wsRef.current?.send(JSON.stringify({ type: "interrupt" }));
  }, []);

  const sendText = useCallback(
    (text: string) => {
      const ws = wsRef.current;
      if (!ws || ws.readyState !== WebSocket.OPEN) return false;
      ws.send(
        JSON.stringify({ type: "user.text", text, lang: agentLang || "en" }),
      );
      return true;
    },
    [agentLang],
  );

  const playPCM = useCallback((buf: ArrayBuffer) => {
    if (interruptedRef.current) return;
    const ctx = playCtxRef.current;
    if (!ctx) return;
    const i16 = new Int16Array(buf);
    if (i16.length === 0) return;

    const f32 = new Float32Array(i16.length);
    for (let i = 0; i < i16.length; i++) f32[i] = i16[i] / 32768;
    const audio = ctx.createBuffer(1, f32.length, PLAYBACK_RATE);
    audio.copyToChannel(f32, 0);

    if (!speakingRef.current) {
      speakingRef.current = true;
      doneRef.current = false;
      graceUntilRef.current = performance.now() + BARGE_GRACE_MS;
      setStatus("agent speaking…");
    }

    const startAt = Math.max(ctx.currentTime, nextStartRef.current);
    const src = ctx.createBufferSource();
    src.buffer = audio;
    src.connect(ctx.destination);
    sourcesRef.current.add(src);
    src.onended = () => {
      sourcesRef.current.delete(src);
      if (sourcesRef.current.size === 0 && doneRef.current) {
        speakingRef.current = false;
        doneRef.current = false;
        setStatus("listening…");
      }
    };
    src.start(startAt);
    nextStartRef.current = startAt + audio.duration;
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
          interruptedRef.current = false;
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
          {
            const a = msg.args ?? {};
            setBooking((b) => {
              switch (msg.name) {
                case "set_service":
                  return { ...b, service: String(a.service ?? b.service) };
                case "save_address":
                  return { ...b, address: String(a.address ?? b.address) };
                case "set_time":
                  return {
                    ...b,
                    date: String(a.date ?? b.date),
                    window: String(a.window ?? b.window),
                  };
                case "set_phone":
                  return { ...b, phone: String(a.phone ?? b.phone) };
                case "book":
                  return { ...b, booked: true };
                default:
                  return b;
              }
            });
          }
          break;
        case "call.end":
          if (msg.reason === "booked") {
            setBooking((b) => ({ ...b, booked: true }));
          }
          break;
        case "citation":
          setCitations(msg.docs ?? []);
          break;
        case "latency":
          setLatency({
            stt: msg.stt_ms ?? 0,
            brain: msg.brain_ms ?? 0,
            tts: msg.tts_ms ?? 0,
            firstAudio: msg.first_audio_ms ?? -1,
            turn: msg.turn_ms ?? 0,
          });
          break;
        case "agent.speaking":
          speakingRef.current = true;
          graceUntilRef.current = performance.now() + BARGE_GRACE_MS;
          doneRef.current = false;
          setStatus("agent speaking…");
          break;
        case "agent.done":
          interruptedRef.current = false;
          doneRef.current = true;
          if (sourcesRef.current.size === 0) {
            speakingRef.current = false;
            setStatus("listening…");
          }
          break;
      }
    },
    [playPCM],
  );

  const connect = useCallback((): Promise<WebSocket> => {
    return new Promise((resolve) => {
      const url = new URL(WS_URL);
      url.searchParams.set("tenant", tenantRef.current);
      const ws = new WebSocket(url.toString());
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

  const refreshDevices = useCallback(async () => {
    try {
      const all = await navigator.mediaDevices.enumerateDevices();
      const inputs = all
        .filter((d) => d.kind === "audioinput")
        .map((d, i) => ({
          deviceId: d.deviceId,
          label: d.label || `Microphone ${i + 1}`,
        }));
      setDevices(inputs);
    } catch {
      /* enumerate not available */
    }
  }, []);

  useEffect(() => {
    refreshDevices();
    const md = navigator.mediaDevices;
    if (md?.addEventListener) {
      md.addEventListener("devicechange", refreshDevices);
      return () => md.removeEventListener("devicechange", refreshDevices);
    }
  }, [refreshDevices]);

  const ensureAudio = useCallback(async () => {
    if (ctxRef.current) return;
    const stream = await navigator.mediaDevices.getUserMedia({
      audio: {
        ...(deviceIdRef.current
          ? { deviceId: { exact: deviceIdRef.current } }
          : {}),
        channelCount: 1,
        echoCancellation: true,
        noiseSuppression: false,
        autoGainControl: false,
      },
    });
    await refreshDevices();
    const ctx = new AudioContext({ sampleRate: SAMPLE_RATE });
    setSampleRate(ctx.sampleRate);
    await ctx.audioWorklet.addModule("/audio-worklet.js");
    if (!playCtxRef.current) {
      try {
        playCtxRef.current = new AudioContext({ sampleRate: PLAYBACK_RATE });
      } catch {
        playCtxRef.current = new AudioContext();
      }
      await playCtxRef.current.resume();
    }
    const source = ctx.createMediaStreamSource(stream);
    const node = new AudioWorkletNode(ctx, "pcm-capture");
    node.port.onmessage = (e: MessageEvent<Int16Array>) => {
      const ws = wsRef.current;
      if (!streamingRef.current || !ws || ws.readyState !== WebSocket.OPEN) return;

      const frameLevel = rms(e.data);
      const now = performance.now();
      if (now - lastLevelRef.current > 100) {
        lastLevelRef.current = now;
        setLevel(frameLevel);
      }

      const agentSpeaking = speakingRef.current && now > graceUntilRef.current;
      if (agentSpeaking) {
        if (frameLevel > BARGE_RMS) {
          interrupt();
          ws.send(e.data as unknown as ArrayBuffer);
        }
        return;
      }

      ws.send(e.data as unknown as ArrayBuffer);
    };
    source.connect(node);
    const sink = ctx.createGain();
    sink.gain.value = 0;
    node.connect(sink);
    sink.connect(ctx.destination);
    streamRef.current = stream;
    ctxRef.current = ctx;
    nodeRef.current = node;
  }, [interrupt, refreshDevices]);

  const startCall = useCallback(
    async (lang: string, tenant: string, deviceId: string) => {
      tenantRef.current = tenant;
      deviceIdRef.current = deviceId;
      interruptedRef.current = false;
      setBooking(emptyBooking);
      try {
        await ensureAudio();
      } catch {
        setStatus("mic error");
        return;
      }
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
    speakingRef.current = false;
    for (const src of sourcesRef.current) {
      try {
        src.stop();
      } catch {
        /* already stopped */
      }
    }
    sourcesRef.current.clear();
    nextStartRef.current = 0;
    doneRef.current = false;
    interruptedRef.current = false;
    setInCall(false);
    if (pingTimerRef.current) clearInterval(pingTimerRef.current);
    wsRef.current?.close();
    wsRef.current = null;
    nodeRef.current?.disconnect();
    streamRef.current?.getTracks().forEach((t) => t.stop());
    await ctxRef.current?.close();
    await playCtxRef.current?.close();
    nodeRef.current = null;
    ctxRef.current = null;
    playCtxRef.current = null;
    streamRef.current = null;
    setStatus("idle");
    setTranscript("");
    setAgentText("");
    setAgentState("");
    setAgentLang("");
    setTool("");
    setCitations([]);
    setLatency(null);
    setLevel(0);
    setSampleRate(0);
  }, []);

  useEffect(() => {
    return () => {
      if (pingTimerRef.current) clearInterval(pingTimerRef.current);
      wsRef.current?.close();
      ctxRef.current?.close();
      playCtxRef.current?.close();
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
    citations,
    booking,
    latency,
    level,
    sampleRate,
    devices,
    refreshDevices,
    startCall,
    endCall,
    sendText,
  };
}

function rms(i16: Int16Array): number {
  if (i16.length === 0) return 0;
  let sum = 0;
  for (let i = 0; i < i16.length; i++) {
    const v = i16[i];
    sum += v * v;
  }
  return Math.sqrt(sum / i16.length);
}
