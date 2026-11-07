from __future__ import annotations

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
    def __init__(self, cfg: SessionConfig, client: GoClient, ctx, events: EventPublisher | None = None) -> None:
        super().__init__(instructions=build_instructions(cfg))
        self.cfg = cfg
        self.client = client
        self.ctx = ctx
        self.events = events or EventPublisher(ctx.room)
        self.consent_decided = False
        self.recording_started = False
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

        Args:
            phone: The caller's 10-digit mobile number, with or without +91.
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
    async def set_recording_consent(self, context: RunContext, allowed: bool) -> str:
        """Record the caller's recording choice exactly once, before anything is recorded.

        Args:
            allowed: True only if the caller clearly agreed to recording.
        """
        if self.consent_decided:
            return "The caller's recording choice is already saved."
        self.consent_decided = True
        try:
            await self.client.post("/internal/tools/consent", {"allowed": allowed})
        except ToolError:
            self.consent_decided = False
            raise
        await self.events.emit("consent", allowed=allowed)

        if allowed:
            self.recording_started = await start_consented_recording(self.ctx, self.cfg.call_id)
            if self.recording_started:
                return "Recording is on. Thank the caller and ask how you can help."
            return (
                "Consent is saved, but recording is unavailable right now. "
                "Do not claim the call is being recorded; ask how you can help."
            )
        return "Recording will stay off for this call. Thank the caller and ask how you can help."

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
