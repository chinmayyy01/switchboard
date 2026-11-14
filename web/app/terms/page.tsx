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
      <p className={`${type.caption2} mt-2 text-[#8A8379]`}>Last updated 14 November 2026</p>

      <div className={`${type.body3} mt-8 space-y-6 text-[#A8A092]`}>
        <section>
          <h2 className="text-base font-semibold text-[#EAE5DC]">The service</h2>
          <p className="mt-2">
            Switchboard is an AI voice receptionist for home-service businesses. It
            answers calls, books visits from the business&apos;s schedule, answers
            questions from uploaded documents, and asks for a person when needed.
            This deployment is operated by the business or workspace owner that gave
            you access.
          </p>
        </section>

        <section>
          <h2 className="text-base font-semibold text-[#EAE5DC]">AI limitations</h2>
          <p className="mt-2">
            Speech recognition and language models can mishear, misunderstand, or
            produce an incorrect answer. A booking is a request until the business
            confirms it. Do not use Switchboard for emergencies or for anything where
            a mistake could cause harm. If you need urgent help, call the business
            directly or the emergency services.
          </p>
        </section>

        <section>
          <h2 className="text-base font-semibold text-[#EAE5DC]">Accounts</h2>
          <p className="mt-2">
            You must give accurate account information and keep your password
            private. Workspace owners control who can join and what each role can do.
            You are responsible for activity under your account.
          </p>
        </section>

        <section>
          <h2 className="text-base font-semibold text-[#EAE5DC]">Acceptable use</h2>
          <p className="mt-2">
            Do not use Switchboard to harass people, to impersonate someone without
            permission, to make unlawful or emergency calls, to send spam, or to
            upload content you do not have the right to use. Do not attempt to bypass
            access controls or disrupt the service.
          </p>
        </section>

        <section>
          <h2 className="text-base font-semibold text-[#EAE5DC]">Calls and transcripts</h2>
          <p className="mt-2">
            Calls are not recorded. The receptionist says it is an AI assistant and
            asks before keeping a text transcript. If the caller declines, no
            transcript turns are stored and the call continues normally. Transcripts
            are deleted after the retention period described in the privacy page.
            Callers must not use the service to record another person without their
            consent.
          </p>
        </section>

        <section>
          <h2 className="text-base font-semibold text-[#EAE5DC]">Usage and budgets</h2>
          <p className="mt-2">
            Workspaces can set a monthly usage cap. When the cap is reached, the
            workspace chooses whether to transfer the caller to a person, stop
            booking, or continue with a cheaper configuration. Caps are a cost
            control, not a guarantee of service.
          </p>
        </section>

        <section>
          <h2 className="text-base font-semibold text-[#EAE5DC]">Availability and changes</h2>
          <p className="mt-2">
            The service depends on third-party media, speech, and language providers
            and may be interrupted. Features may change or be withdrawn. Planned
            maintenance and material changes should be communicated by the operator.
          </p>
        </section>

        <section>
          <h2 className="text-base font-semibold text-[#EAE5DC]">Intellectual property</h2>
          <p className="mt-2">
            The business keeps ownership of its documents, bookings, and customer
            data. The operator keeps ownership of the software. Nothing in these
            terms transfers ownership of either.
          </p>
        </section>

        <section>
          <h2 className="text-base font-semibold text-[#EAE5DC]">Liability</h2>
          <p className="mt-2">
            To the extent permitted by law, the service is provided as is, without
            warranties. The operator is not liable for indirect or consequential
            loss, lost bookings, or decisions made from an incorrect AI answer.
          </p>
        </section>

        <section>
          <h2 className="text-base font-semibold text-[#EAE5DC]">Termination</h2>
          <p className="mt-2">
            The operator may suspend or end access for misuse or non-payment. A
            workspace owner may stop using the service at any time. Data is handled
            as described in the privacy page.
          </p>
        </section>

        <section>
          <h2 className="text-base font-semibold text-[#EAE5DC]">Contact</h2>
          <p className="mt-2">
            Questions about these terms should go to the owner of the workspace you
            use or the operator of this deployment. These terms are governed by the
            laws of India.
          </p>
        </section>
      </div>
    </main>
  );
}
