"use client";

import { useEffect, useState } from "react";
import Link from "next/link";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

export default function VerifyPage() {
  const [status, setStatus] = useState<"checking" | "ok" | "error">("checking");
  const [message, setMessage] = useState("Verifying your email…");

  useEffect(() => {
    const token = new URLSearchParams(window.location.search).get("token") ?? "";
    if (!token) {
      setStatus("error");
      setMessage("This verification link is missing a token.");
      return;
    }
    fetch(`${API}/api/auth/verify`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token }),
    })
      .then(async (res) => {
        const data = (await res.json().catch(() => ({}))) as { error?: string };
        if (!res.ok) throw new Error(data.error ?? "Could not verify this link.");
        setStatus("ok");
        setMessage("Email verified. Opening the console…");
        window.setTimeout(() => window.location.assign("/console"), 800);
      })
      .catch((err: Error) => {
        setStatus("error");
        setMessage(err.message);
      });
  }, []);

  return (
    <main className="grid min-h-screen place-items-center bg-[#111111] px-6 text-[#EAE5DC]">
      <div className="w-full max-w-md rounded-2xl border border-[#2E2E2E] bg-[#1A1A1A] p-6 text-center">
        <h1 className="text-xl font-semibold tracking-tight">Email verification</h1>
        <p
          className={`mt-3 text-sm ${
            status === "ok"
              ? "text-emerald-300"
              : status === "error"
                ? "text-rose-300"
                : "text-[#A8A092]"
          }`}
        >
          {message}
        </p>
        {status === "error" && (
          <div className="mt-5 space-y-3">
            <Link href="/forgot-password" className="block text-sm text-[#D98E5A] hover:underline">
              Need help signing in?
            </Link>
            <Link href="/login" className="block text-sm text-[#A8A092] hover:text-[#D8D2C7]">
              Back to sign in
            </Link>
          </div>
        )}
      </div>
    </main>
  );
}
