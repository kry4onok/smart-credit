import threading
from datetime import datetime, timezone

import grpc
from proto.assistant import help_pb2, help_pb2_grpc

from .planner import plan_answer
from .render import fallback, fallback_plan, limited_fallback, render_plan
from .validation import InputError, PlanError, validate_plan, validate_request


class HelpService(help_pb2_grpc.AssistantHelpServiceServicer):
    def __init__(self, provider, catalog, clock=None, max_inflight: int = 2, timeout_error=TimeoutError, output_error=ValueError):
        self.provider = provider
        self.catalog = catalog
        self.clock = clock or (lambda: datetime.now(timezone.utc))
        self.timeout_error = timeout_error
        self.output_error = output_error
        self.slots = threading.BoundedSemaphore(max_inflight)

    def Help(self, request, context):
        if not request.HasField("context"):
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "Missing context")
        if self.catalog is None or request.context.catalog_version != self.catalog["version"]:
            context.set_trailing_metadata((("assistant-error", "UNSUPPORTED_CATALOG_VERSION"),))
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, "UNSUPPORTED_CATALOG_VERSION")
        now = self.clock()
        try:
            validate_request(request, self.catalog, now)
        except InputError as exc:
            if str(exc) in ("Input too large", "Context collection limit exceeded"):
                return limited_fallback(request, self.catalog, now)
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(exc))
        if not context.is_active():
            context.abort(grpc.StatusCode.CANCELLED, "Request cancelled")
        if request.context.freshness == help_pb2.FRESHNESS_STALE:
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, "CONTEXT_EXPIRED")
        if not request.context.statements or fallback_plan(request)["status"] == "INSUFFICIENT_DATA":
            return self._fallback(request, context, "NO_SUPPORTED_CONTEXT")
        remaining = context.time_remaining()
        timeout = max(0.0, min(40.0, remaining - 0.3)) if remaining is not None else 4.7
        if timeout <= 0:
            return self._fallback(request, context, "MODEL_TIMEOUT")
        if not self.slots.acquire(timeout=min(0.1, timeout)):
            return self._fallback(request, context, "QUEUE_FULL")
        try:
            try:
                plan, model_version = plan_answer(request, timeout, self.provider)
            except self.timeout_error:
                return self._fallback(request, context, "MODEL_TIMEOUT")
            except self.output_error:
                return self._fallback(request, context, "INVALID_MODEL_OUTPUT")
            except Exception:
                return self._fallback(request, context, "MODEL_UNAVAILABLE")
        finally:
            self.slots.release()
        if not context.is_active():
            context.abort(grpc.StatusCode.CANCELLED, "Request cancelled")
        now = self.clock()
        if request.context.valid_until.ToDatetime(tzinfo=timezone.utc) <= now:
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, "CONTEXT_EXPIRED")
        try:
            validate_plan(plan, request)
            return render_plan(plan, request, self.catalog, help_pb2.RESPONSE_SOURCE_MODEL, model_version, "NONE", now)
        except (PlanError, TypeError, ValueError, KeyError):
            return self._fallback(request, context, "INVALID_MODEL_OUTPUT")

    def _fallback(self, request, context, reason):
        now = self.clock()
        if not context.is_active():
            context.abort(grpc.StatusCode.CANCELLED, "Request cancelled")
        if request.context.valid_until.ToDatetime(tzinfo=timezone.utc) <= now:
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, "CONTEXT_EXPIRED")
        return fallback(request, self.catalog, reason, now)
