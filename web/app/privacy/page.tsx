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
      <p className={`${type.caption2} mt-2 text-[#8A8379]`}>Last updated 14 November 2026</p>

      <div className={`${type.body3} mt-8 space-y-6 text-[#A8A092]`}>
        <p>
          Switchboard is an AI voice receptionist for home-service businesses. This
          page explains what this deployment stores, why, and for how long.
        </p>

        <section>
          <h2 className="text-base font-semibold text-[#EAE5DC]">Calls are not recorded</h2>
          <p className="mt-2">
            Switchboard does not store audio recordings of calls. Audio is processed
            in real time by LiveKit and the speech providers listed below so the
            receptionist can listen and reply. It is not written to storage by this
            application.
          </p>
        </section>

        <section>
          <h2 className="text-base font-semibold text-[#EAE5DC]">What we store</h2>
          <ul className="mt-2 list-disc space-y-2 pl-5">
            <li>
              <span className="text-[#D8D2C7]">Account data:</span> name, email
              address, a hashed password, role, and session records.
            </li>
            <li>
              <span className="text-[#D8D2C7]">Workspace data:</span> business name,
              services, visit windows, staff, transfer number, disclosure text, and
              the policy documents you upload.
            </li>
            <li>
              <span className="text-[#D8D2C7]">Call data:</span> caller and
              destination numbers, timestamps, duration, outcome, and booking
              details.
            </li>
            <li>
              <span className="text-[#D8D2C7]">Transcripts:</span> the text of the
              conversation, only when the caller agrees. The receptionist asks before
              saving a transcript, and the call continues normally if the caller
              declines.
            </li>
            <li>
              <span className="text-[#D8D2C7]">Technical data:</span> IP address,
              browser details, and error logs used to run and secure the service.
            </li>
          </ul>
        </section>

        <section>
          <h2 className="text-base font-semibold text-[#EAE5DC]">Who processes it</h2>
          <p className="mt-2">
            Depending on the voice stack selected for a call, data is processed by
            LiveKit (media and SIP), Sarvam, Deepgram, Google, OpenAI, Cartesia, or
            ElevenLabs (speech recognition, language, and voice), and by the hosting
            and email providers used by this deployment. These providers process data
            on their own terms and may process it outside your country.
          </p>
        </section>

        <section>
          <h2 className="text-base font-semibold text-[#EAE5DC]">How long we keep it</h2>
          <ul className="mt-2 list-disc space-y-2 pl-5">
            <li>Transcript turns are deleted after 180 days by default.</li>
            <li>
              Call records, booking records, and consent evidence are kept while the
              workspace is active so the business can review its history.
            </li>
            <li>Account data is kept until the account or workspace is deleted.</li>
          </ul>
        </section>

        <section>
          <h2 className="text-base font-semibold text-[#EAE5DC]">Cookies</h2>
          <p className="mt-2">
            The console uses one session cookie to keep you signed in. It is
            HTTP-only and is not used for advertising or cross-site tracking.
          </p>
        </section>

        <section>
          <h2 className="text-base font-semibold text-[#EAE5DC]">Your choices</h2>
          <p className="mt-2">
            To access, correct, or delete your data, or to withdraw consent for a
            transcript, contact the owner of the workspace you called or the operator
            of this deployment. A caller can decline a transcript at the start of the
            call and still use every other part of the service.
          </p>
        </section>

        <section>
          <h2 className="text-base font-semibold text-[#EAE5DC]">Security</h2>
          <p className="mt-2">
            Passwords are stored as salted hashes. Console access is role-based, and
            production deployments use HTTPS. No system is completely secure, so
            please use a unique password and report anything suspicious to the
            workspace owner.
          </p>
        </section>

        <section>
          <h2 className="text-base font-semibold text-[#EAE5DC]">Changes</h2>
          <p className="mt-2">
            This page may change as the product changes. Material changes should be
            communicated by the operator of the deployment.
          </p>
        </section>
      </div>
    </main>
  );
}
