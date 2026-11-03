"use client";

import { useEffect, type ReactNode } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useSession } from "@/lib/session";

const authDisabled = process.env.NEXT_PUBLIC_AUTH_DISABLED === "true";

export function AuthGate({ children }: { children: ReactNode }) {
  const { account, loading, error } = useSession();
  const pathname = usePathname();
  const router = useRouter();

  useEffect(() => {
    if (loading || account || error) return;
    const next = `${pathname}${window.location.search}`;
    router.replace(`/login?next=${encodeURIComponent(next)}`);
  }, [account, error, loading, pathname, router]);

  if (loading) {
    return (
      <div className="grid min-h-screen place-items-center bg-[#111111]">
        <div className="flex items-center gap-3 text-sm text-[#A8A092]">
          <span className="h-2 w-2 animate-pulse rounded-full bg-[#C87941]" />
          Checking session…
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="grid min-h-screen place-items-center bg-[#111111] px-6">
        <div className="max-w-sm rounded-2xl border border-[#3A3A3A] bg-[#242424] p-6 text-center">
          <p className="text-sm font-medium text-[#EAE5DC]">{error}</p>
          <button
            onClick={() => window.location.reload()}
            className="mt-4 inline-block rounded-xl bg-[#C87941] px-4 py-2 text-sm font-semibold text-[#111111]"
          >
            Retry
          </button>
          {!authDisabled && (
            <a
              href="/login"
              className="mt-2 block text-xs font-medium text-[#D98E5A]"
            >
              Go to sign in
            </a>
          )}
        </div>
      </div>
    );
  }

  if (!account) {
    return (
      <div className="grid min-h-screen place-items-center bg-[#111111]">
        <div className="text-sm text-[#A8A092]">Redirecting to sign in…</div>
      </div>
    );
  }

  return <>{children}</>;
}
