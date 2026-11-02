import Link from "next/link";
import { type } from "@/components/landing/type";

export default function NotFound() {
  return (
    <main className="grid min-h-screen place-items-center bg-[#111111] px-6">
      <div className="max-w-[680px] text-center">
        <p className={`${type.caption3} uppercase tracking-wide text-[#A8A092]`}>
          404
        </p>
        <h1 className={`${type.heading3} mt-3`}>
          This page is not on the board.
        </h1>
        <p className={`${type.body3} mt-4 text-[#A8A092]`}>
          The call may have ended, or the link is wrong. The console and the
          demo are still here.
        </p>
        <div className="mt-8">
          <Link
            href="/"
            className={`inline-flex items-center justify-center rounded-xl bg-[#C87941] px-3 py-2 ${type.buttonMd} text-[#111111] transition-[background-color,transform] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)] hover:bg-[#D98E5A] active:scale-[0.98] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#D98E5A] focus-visible:ring-offset-2 focus-visible:ring-offset-[#111111]`}
          >
            Back to Switchboard
          </Link>
        </div>
      </div>
    </main>
  );
}
