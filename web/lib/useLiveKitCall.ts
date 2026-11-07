"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  ConnectionState,
  Room,
  RoomEvent,
  type RemoteParticipant,
  type RemoteTrack,
  type TextStreamReader,
} from "livekit-client";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";
const EVENTS_TOPIC = "switchboard.events";

export type LiveTranscriptLine = {
  id: string;
  role: "caller" | "agent";
  text: string;
  final: boolean;
};

export type LiveBooking = {
  jobId: string;
  service: string;
  address: string;
  date: string;
  window: string;
  employee: string;
};

export type LiveEvent = {
  type: string;
  [key: string]: unknown;
};

type TokenResponse = {
  url: string;
  token: string;
  room: string;
  tenant_id: string;
};

export function useLiveKitCall() {
  const roomRef = useRef<Room | null>(null);
  const audioHostRef = useRef<HTMLDivElement | null>(null);
  const [status, setStatus] = useState<"idle" | "connecting" | "connected" | "error">("idle");
  const [message, setMessage] = useState("");
  const [roomName, setRoomName] = useState("");
  const [transcript, setTranscript] = useState<LiveTranscriptLine[]>([]);
  const [booking, setBooking] = useState<LiveBooking | null>(null);
  const [events, setEvents] = useState<LiveEvent[]>([]);
  const [agentSpeaking, setAgentSpeaking] = useState(false);

  const registerTranscription = useCallback((room: Room) => {
    room.registerTextStreamHandler(
      "lk.transcription",
      async (reader: TextStreamReader, participant: { identity: string }) => {
        let text = "";
        try {
          text = (await reader.readAll()).trim();
        } catch {
          return;
        }
        if (!text) return;
        const final = reader.info.attributes?.["lk.transcription_final"] === "true";
        const segment = reader.info.attributes?.["lk.segment_id"] ?? `${Date.now()}`;
        const role: "caller" | "agent" =
          participant.identity === room.localParticipant.identity ? "caller" : "agent";
        const id = `${participant.identity}:${segment}`;
        // Keep arrival order. Segment IDs are opaque, so sorting by ID
        // scrambles the conversation.
        setTranscript((current) => {
          const index = current.findIndex((line) => line.id === id);
          if (index === -1) return [...current, { id, role, text, final }];
          const next = [...current];
          next[index] = { id, role, text, final };
          return next;
        });
      },
    );
  }, []);

  const registerEvents = useCallback((room: Room) => {
    room.on(
      RoomEvent.DataReceived,
      (payload: Uint8Array, _participant?: RemoteParticipant, _kind?: unknown, topic?: string) => {
        if (topic !== EVENTS_TOPIC) return;
        try {
          const text = new TextDecoder().decode(payload);
          const event = JSON.parse(text) as LiveEvent;
          setEvents((current) => [...current.slice(-49), event]);
          if (event.type === "booking") {
            setBooking({
              jobId: String(event.job_id ?? ""),
              service: String(event.service ?? ""),
              address: String(event.address ?? ""),
              date: String(event.date ?? ""),
              window: String(event.window ?? ""),
              employee: String(event.employee ?? ""),
            });
          }
        } catch {
          // Ignore malformed UI events; booking state still comes from Go.
        }
      },
    );
  }, []);

  const attachAudio = useCallback((track: RemoteTrack) => {
    if (track.kind !== "audio") return;
    const host = audioHostRef.current;
    if (!host) return;
    const element = track.attach() as HTMLAudioElement;
    element.autoplay = true;
    host.appendChild(element);
  }, []);

  const start = useCallback(
    async (tenant = "rivera") => {
      if (roomRef.current) return;
      setStatus("connecting");
      setMessage("");
      setTranscript([]);
      setBooking(null);
      setEvents([]);
      try {
        const response = await fetch(
          `${API}/api/livekit/token?tenant=${encodeURIComponent(tenant)}`,
          { credentials: "include" },
        );
        if (!response.ok) {
          throw new Error((await response.text()) || "Could not get a room token.");
        }
        const token = (await response.json()) as TokenResponse;
        const room = new Room({ adaptiveStream: false, dynacast: false });
        roomRef.current = room;
        setRoomName(token.room);

        registerTranscription(room);
        registerEvents(room);
        room.on(RoomEvent.TrackSubscribed, (track) => attachAudio(track));
        room.on(RoomEvent.ActiveSpeakersChanged, (speakers) => {
          setAgentSpeaking(
            speakers.some((speaker) => speaker.identity !== room.localParticipant.identity),
          );
        });
        room.on(RoomEvent.Disconnected, () => {
          setStatus("idle");
          roomRef.current = null;
        });

        await room.connect(token.url, token.token);
        await room.localParticipant.setMicrophoneEnabled(true);
        await room.startAudio();
        setStatus("connected");
      } catch (error) {
        setStatus("error");
        setMessage(error instanceof Error ? error.message : "Could not start the call.");
        await roomRef.current?.disconnect();
        roomRef.current = null;
      }
    },
    [attachAudio, registerEvents, registerTranscription],
  );

  const end = useCallback(async () => {
    const room = roomRef.current;
    roomRef.current = null;
    setStatus("idle");
    setAgentSpeaking(false);
    if (audioHostRef.current) audioHostRef.current.replaceChildren();
    await room?.disconnect();
  }, []);

  useEffect(() => {
    return () => {
      void roomRef.current?.disconnect();
      roomRef.current = null;
    };
  }, []);

  return {
    status,
    message,
    roomName,
    transcript,
    booking,
    events,
    agentSpeaking,
    audioHostRef,
    start,
    end,
    connected: status === "connected" || status === "connecting",
    connectionState: roomRef.current?.state ?? ConnectionState.Disconnected,
  };
}
