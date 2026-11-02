"use client";

import { useEffect, useRef, useState } from "react";
import { Badge, Card } from "@/components/ui";

const AI = process.env.NEXT_PUBLIC_AI_URL ?? "http://localhost:8100";

type Status = "idle" | "analyzing" | "sent" | "error";

export function PhotoCard({
  inCall,
  onSendText,
}: {
  inCall: boolean;
  onSendText: (text: string) => boolean;
}) {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [preview, setPreview] = useState("");
  const [description, setDescription] = useState("");
  const [status, setStatus] = useState<Status>("idle");
  const [error, setError] = useState("");

  useEffect(() => {
    return () => {
      if (preview) URL.revokeObjectURL(preview);
    };
  }, [preview]);

  const pick = () => inputRef.current?.click();

  const analyze = async (file: File) => {
    if (preview) URL.revokeObjectURL(preview);
    setPreview(URL.createObjectURL(file));
    setDescription("");
    setError("");
    setStatus("analyzing");

    try {
      const res = await fetch(`${AI}/vision`, {
        method: "POST",
        headers: { "Content-Type": file.type || "application/octet-stream" },
        body: file,
      });
      if (!res.ok) throw new Error(await res.text());
      const data = (await res.json()) as {
        description?: string;
        available?: boolean;
      };
      const text = (data.description ?? "").trim();
      if (!text) throw new Error("no description");
      setDescription(text);
      if (inCall && onSendText(`The caller uploaded a photo. Vision: ${text}`)) {
        setStatus("sent");
      } else {
        setStatus("error");
        setError("Start a call to send this photo to the agent.");
      }
    } catch {
      setStatus("error");
      setError("Photo analysis is unavailable right now.");
    }
  };

  return (
    <Card className="p-5">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="text-base font-semibold tracking-tight text-slate-900">
            Show me the problem
          </h2>
          <p className="text-sm text-slate-500">
            Upload a photo; the agent looks at it.
          </p>
        </div>
        <Badge
          tone={
            status === "sent"
              ? "emerald"
              : status === "error"
                ? "rose"
                : status === "analyzing"
                  ? "amber"
                  : "slate"
          }
        >
          {status === "sent"
            ? "sent"
            : status === "error"
              ? "failed"
              : status === "analyzing"
                ? "analyzing…"
                : "idle"}
        </Badge>
      </div>

      {preview ? (
        <img
          src={preview}
          alt="Uploaded problem"
          className="mt-4 h-40 w-full rounded-xl border border-slate-200 object-cover"
        />
      ) : (
        <button
          type="button"
          onClick={pick}
          disabled={!inCall}
          className="mt-4 grid h-40 w-full place-items-center rounded-xl border border-dashed border-slate-300 bg-slate-50 text-sm text-slate-500 transition-colors hover:border-indigo-300 hover:bg-indigo-50/40 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {inCall ? "Take or choose a photo" : "Start a call to add a photo"}
        </button>
      )}

      {preview && (
        <div className="mt-3 flex items-center gap-2">
          <button
            type="button"
            onClick={pick}
            disabled={!inCall}
            className="rounded-lg bg-slate-900 px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-slate-700 disabled:opacity-50"
          >
            Change photo
          </button>
          {status === "sent" && (
            <span className="text-xs text-slate-400">Sent to the agent</span>
          )}
        </div>
      )}

      {status === "analyzing" && (
        <p className="mt-3 text-xs text-amber-600">
          Analyzing the photo locally… this can take a few seconds.
        </p>
      )}
      {description && (
        <p className="mt-3 rounded-xl bg-slate-50 p-3 text-xs leading-relaxed text-slate-600">
          {description}
        </p>
      )}
      {error && <p className="mt-3 text-xs text-rose-600">{error}</p>}

      <input
        ref={inputRef}
        type="file"
        accept="image/*"
        capture="environment"
        className="hidden"
        onChange={(e) => {
          const file = e.target.files?.[0];
          if (file) void analyze(file);
          e.target.value = "";
        }}
      />
    </Card>
  );
}
