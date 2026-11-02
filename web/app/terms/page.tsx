import type { Metadata } from "next";
import Link from "next/link";
import { type } from "@/components/landing/type";

export const metadata: Metadata = {
  title: "Terms | Switchboard",
};

export default function Terms() {
  return (
    <main className="mx-auto max-w-3xl px-6 py-24">
      <Link
        href="/"
        className={`${type.caption2} text-[#D98E5A] transition-colors duration-300 hover:text-[#EAE5DC]`}
      >
        Back to Switchboard
      </Link>
      <h1 className={`${type.heading3} mt-8`}>Terms</h1>
      <div className={`${type.body3} mt-6 space-y-4 text-[#A8A092]`}>
        <p>
          This is a demo. It is provided as is, with no warranty. Do not use it
          for emergency calls or production dispatch.
        </p>
        <p>
          Bookings are written to an in memory store for the demo. Sarvam
          processes speech and language, and its terms also apply.
        </p>
        <p>
          The source is public. See the repository for the current behavior and
          limitations.
        </p>
      </div>
    </main>
  );
}
