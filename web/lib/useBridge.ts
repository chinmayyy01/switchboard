"use client";

import { useEffect, useState } from "react";
import { Room, RoomEvent, type RemoteTrack } from "livekit-client";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

type TokenResponse = { url: string; token: string; room: string };

// A supervisor joins the live LiveKit room to listen and speak. The agent mutes
// its own microphone when a supervisor joins, so the caller hears the human.
export function useBridge(id: string) {
  const [status, setStatus] = useState("idle");

  useEffect(() => {
    if (!id) {
      setStatus("idle");
      return;
    }
    let cancelled = false;
    let room: Room | null = null;

    const connect = async () => {
      setStatus("connecting");
      try {
        const response = await fetch(
          `${API}/api/calls/${encodeURIComponent(id)}/supervisor-token`,
          { credentials: "include" },
        );
        if (!response.ok) throw new Error(await response.text());
        const token = (await response.json()) as TokenResponse;
        room = new Room({ adaptiveStream: false, dynacast: false });
        room.on(RoomEvent.TrackSubscribed, (track: RemoteTrack) => {
          if (track.kind !== "audio") return;
          const element = track.attach() as HTMLAudioElement;
          element.autoplay = true;
          element.setAttribute("playsinline", "true");
          element.dataset.supervisorAudio = "true";
          document.body.appendChild(element);
          void element.play().catch(() => undefined);
        });
        room.on(RoomEvent.Disconnected, () => {
          if (!cancelled) setStatus("closed");
        });
        await room.connect(token.url, token.token);
        try {
          await room.localParticipant.setMicrophoneEnabled(true);
        } catch {
          if (!cancelled) setStatus("mic denied");
          return;
        }
        if (!cancelled) setStatus("connected");
      } catch {
        if (!cancelled) setStatus("error");
      }
    };

    void connect();
    return () => {
      cancelled = true;
      document.querySelectorAll("[data-supervisor-audio]").forEach((el) => el.remove());
      void room?.disconnect();
    };
  }, [id]);

  return { status };
}
