"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useBridge } from "@/lib/useBridge";
import { useSupervisor } from "@/lib/useSupervisor";

export default function CallPage() {
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const { status } = useBridge(id);
  const { calls } = useSupervisor();
  const call = calls.find((c) => c.id === id);

  return (
    <main className="mx-auto max-w-3xl px-6 py-16 font-mono leading-relaxed">
      <Link href="/supervisor" className="text-sm text-blue-600 hover:underline">
        ← supervisor
      </Link>

      <h1 className="mt-2 text-2xl font-semibold">Call {id}</h1>
      <p className="mt-1 text-sm text-neutral-500">
        bridge:{" "}
        <span className={status === "connected" ? "text-emerald-600" : "text-neutral-500"}>
          {status}
        </span>
      </p>

      {call?.reason && (
        <p className="mt-3 text-sm text-red-600">why escalated: {call.reason}</p>
      )}

      <ul className="mt-4 space-y-1">
        {(call?.transcript ?? []).map((t, i) => (
          <li key={i} className="text-sm">
            <span className={t.role === "caller" ? "text-neutral-500" : "text-emerald-700"}>
              {t.role}:{" "}
            </span>
            {t.text}
          </li>
        ))}
        {(call?.transcript ?? []).length === 0 && (
          <li className="text-sm text-neutral-400">transcript…</li>
        )}
      </ul>
    </main>
  );
}
