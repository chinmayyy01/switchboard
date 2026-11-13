"use client";

import { useEffect, useState } from "react";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

export default function LogoutPage() {
  const [message, setMessage] = useState("Signing out…");

  useEffect(() => {
    fetch(`${API}/api/auth/logout`, {
      method: "POST",
      credentials: "include",
    })
      .catch(() => undefined)
      .finally(() => {
        setMessage("Signed out. Opening sign in…");
        window.setTimeout(() => window.location.assign("/login"), 400);
      });
  }, []);

  return (
    <main className="grid min-h-screen place-items-center bg-[#111111] px-6 text-[#EAE5DC]">
      <p className="text-sm text-[#A8A092]">{message}</p>
    </main>
  );
}
