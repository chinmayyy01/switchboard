import type { Metadata } from "next";
import Link from "next/link";
import { type } from "@/components/landing/type";

export const metadata: Metadata = {
  title: "Privacy | Switchboard",
};

export default function Privacy() {
  return (
    <main className="mx-auto max-w-3xl px-6 py-24">
      <Link
        href="/"
        className={`${type.caption2} text-[#D98E5A] transition-colors duration-300 hover:text-[#EAE5DC]`}
      >
        Back to Switchboard
      </Link>
      <h1 className={`${type.heading3} mt-8`}>Privacy</h1>
      <div className={`${type.body3} mt-6 space-y-4 text-[#A8A092]`}>
        <p>
          This is a demo. Call transcripts are stored locally in the demo data
          file. Audio is sent to Sarvam for speech recognition, voice, and
          language processing.
        </p>
        <p>
          Do not use this demo with sensitive data. The demo may reset its data
          at any time.
        </p>
        <p>
          The source is public. See the repository for the exact data paths and
          processing steps.
        </p>
      </div>
    </main>
  );
}
