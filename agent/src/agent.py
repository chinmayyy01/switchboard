from __future__ import annotations

import asyncio
import logging

from livekit.agents import Agent, RunContext, function_tool
from livekit.agents.llm import ToolError

from config import SessionConfig
from events import EventPublisher
from goclient import GoClient
from prompts import build_instructions
from recording import start_consented_recording
from transfer import warm_transfer

logger = logging.getLogger("switchboard.agent")


class Receptionist(Agent):
    def __init__(
        self,
        cfg: SessionConfig,
        client: GoClient,
        ctx,
        events: EventPublisher | None = None,
        turns=None,
    ) -> None:
        super().__init__(instructions=build_instructions(cfg))
        self.cfg = cfg
        self.client = client
        self.ctx = ctx
        self.events = events or EventPublisher(ctx.room)
        self.turns = turns
        self.consent_decided = False
        self.recording_allowed = False
        self.transcript_allowed = False
        self.recording_started = False
        self.egress_id = ""
        self.transferred = False

    @function_tool()
    async def check_availability(self, context: RunContext, service: str, date: str) -> dict:
        """List the visit windows that are actually open for a service on a date.

        Args:
            service: The service the caller needs, for example "leak repair".
            date: The requested date in YYYY-MM-DD format.
        """
        try:
            return await self.client.post(
                "/internal/tools/availability", {"service": service, "date": date}
            )
        except ToolError as exc:
            raise ToolError(
                "I'm having trouble checking the schedule. Let me connect you to a person."
            ) from exc

    @function_tool()
    async def save_address(self, context: RunContext, address: str) -> dict:
        """Save the service address on the call and return the exact read-back.

        Args:
            address: The full address, including building or flat, street, area, and a landmark.
        """
        return await self.client.post("/internal/tools/save-address", {"address": address})

    @function_tool()
    async def save_callback_number(self, context: RunContext, phone: str) -> dict:
        """Save and validate the caller's 10-digit Indian mobile number.

        Pass the number exactly as the caller said it. Spaces, brackets, dashes,
        and +91 are accepted; do not reject or reformat the caller's number
        before calling this tool.

        Args:
            phone: The caller's number as spoken, for example "6202416257",
                "+91 62024 16257", or "(620) 241-6257".
        """
        return await self.client.post("/internal/tools/save-phone", {"phone": phone})

    @function_tool()
    async def confirm_booking_details(
        self,
        context: RunContext,
        service: str,
        date: str,
        window: str,
    ) -> dict:
        """Validate the booking and return a confirmation_id and read-back.

        The address and phone come from the values already saved on this call,
        so do not repeat them here. After calling this, read the returned
        read_back to the caller and wait for an explicit yes.

        Args:
            service: The service the caller needs, in their words.
            date: The visit date in YYYY-MM-DD format, resolved from today's date.
            window: One of morning, afternoon, or evening.
        """
        context.disallow_interruptions()
        return await self.client.post(
            "/internal/tools/confirm-booking",
            {"service": service, "date": date, "window": window},
        )

    @function_tool()
    async def book_visit(self, context: RunContext, confirmation_id: str) -> dict:
        """Book the visit using a confirmation_id from confirm_booking_details.

        Only call this after the caller explicitly confirms the read-back.

        Args:
            confirmation_id: The confirmation_id returned by confirm_booking_details.
        """
        context.disallow_interruptions()
        try:
            data = await self.client.post(
                "/internal/tools/book", {"confirmation_id": confirmation_id}
            )
        except ToolError as exc:
            raise ToolError(
                "I couldn't complete the booking. Let me read the details back again."
            ) from exc
        job = data.get("job") or {}
        await self.events.emit(
            "booking",
            job_id=job.get("id", ""),
            service=job.get("service", ""),
            address=job.get("address", ""),
            date=job.get("date", ""),
            window=job.get("window", ""),
            phone=job.get("phone", ""),
            employee=job.get("employee_name", ""),
        )
        return data

    @function_tool()
    async def reschedule_visit(
        self, context: RunContext, phone: str, date: str, window: str
    ) -> dict:
        """Move an existing visit to a new date and window.

        Args:
            phone: The phone number on the booking.
            date: The new date in YYYY-MM-DD format.
            window: One of morning, afternoon, or evening.
        """
        context.disallow_interruptions()
        return await self.client.post(
            "/internal/tools/reschedule",
            {"phone": phone, "date": date, "window": window},
        )

    @function_tool()
    async def cancel_visit(self, context: RunContext, phone: str) -> dict:
        """Cancel an existing visit after the caller confirms.

        Args:
            phone: The phone number on the booking.
        """
        context.disallow_interruptions()
        return await self.client.post("/internal/tools/cancel", {"phone": phone})

    @function_tool()
    async def lookup_job(self, context: RunContext, phone: str) -> dict:
        """Find existing visits for a caller's phone number.

        Args:
            phone: The phone number on the booking.
        """
        return await self.client.post("/internal/tools/lookup", {"phone": phone})

    @function_tool()
    async def lookup_policy(self, context: RunContext, query: str) -> dict:
        """Answer a policy or FAQ question from the business knowledge base.

        Args:
            query: The caller's question, in their own words.
        """
        return await self.client.post(
            "/internal/tools/policy",
            {"query": query, "lang": str(getattr(context, "language", "") or "en")},
        )

    @function_tool()
    async def set_recording_consent(
        self,
        context: RunContext,
        allowed: bool,
        transcript_allowed: bool | None = None,
    ) -> str:
        """Record the caller's recording and transcript choice exactly once.

        Args:
            allowed: True only if the caller clearly agreed to audio recording.
            transcript_allowed: True if the caller agreed to a saved text
                transcript. Omit it for a plain yes or no; it then follows
                `allowed`.
        """
        if self.consent_decided:
            return "The caller's recording choice is already saved."
        self.consent_decided = True
        transcript = allowed if transcript_allowed is None else transcript_allowed
        try:
            data = await self.client.post(
                "/internal/tools/consent",
                {
                    "allowed": allowed,
                    "recording_allowed": allowed,
                    "transcript_allowed": transcript,
                    "language": str(getattr(context, "language", "") or "en"),
                },
            )
        except ToolError:
            self.consent_decided = False
            raise
        self.recording_allowed = bool(data.get("recording_allowed", allowed))
        self.transcript_allowed = bool(data.get("transcript_allowed", transcript))
        if self.turns is not None:
            self.turns.consent_set()
        await self.events.emit(
            "consent",
            allowed=self.recording_allowed,
            transcript_allowed=self.transcript_allowed,
        )

        if self.recording_allowed:
            self.egress_id = await start_consented_recording(self.ctx, self.cfg.call_id)
            self.recording_started = bool(self.egress_id)
            if self.recording_started:
                try:
                    await self.client.post(
                        "/internal/tools/recording/started",
                        {"egress_id": self.egress_id, "channels": 2},
                    )
                except ToolError:
                    logger.warning("could not persist recording start", exc_info=True)
                return "Recording is on. Thank the caller and ask how you can help."
            asyncio.create_task(self.client.report_metric("recording_failure"))
            return (
                "Consent is saved, but recording is unavailable right now. "
                "Do not claim the call is being recorded; ask how you can help."
            )
        if self.transcript_allowed:
            return "Audio recording stays off. I'll keep a text note of the call. Ask how you can help."
        return "Nothing will be recorded for this call. Thank the caller and ask how you can help."

    @function_tool()
    async def end_conversation(self, context: RunContext) -> None:
        """End the call after the caller says goodbye or asks to hang up.

        Say a brief, warm goodbye first, then this tool ends the session.
        """
        handle = context.session.say("Thank you for calling. Goodbye!")
        await handle.wait_for_playout()
        try:
            await self.client.post("/internal/tools/end", {"outcome": "completed"})
        except ToolError:
            pass
        await self.ctx.shutdown()

    @function_tool()
    async def transfer_to_human(
        self, context: RunContext, reason: str, urgency: str = "normal"
    ) -> None:
        """Warm-transfer the caller to the business manager. Use for emergencies or any request for a person.

        Args:
            reason: A short reason such as emergency, caller_requested_human, or no_availability.
            urgency: One of normal or emergency.
        """
        context.disallow_interruptions()
        try:
            data = await self.client.post("/internal/tools/escalate", {"reason": reason})
        except ToolError:
            data = {}
        manager = str(data.get("manager_number") or self.cfg.manager_number or "")
        self.transferred = True
        await self.events.emit("escalation", reason=reason, urgency=urgency)

        try:
            await warm_transfer(self, manager)
        except Exception:
            logger.exception("warm transfer failed", extra={"call_id": self.cfg.call_id})
            raise ToolError(
                "I can't reach a teammate right now. I can take a message and have them call you back."
            ) from None
