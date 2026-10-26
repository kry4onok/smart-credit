import json
import re
import uuid
from datetime import date, timezone

from google.protobuf.json_format import MessageToDict
from proto.assistant import help_pb2


class InputError(ValueError):
    pass


class PlanError(ValueError):
    pass


STATUS = {
    "ANSWERED": help_pb2.ANSWER_STATUS_ANSWERED,
    "PARTIAL": help_pb2.ANSWER_STATUS_PARTIAL,
    "INSUFFICIENT_DATA": help_pb2.ANSWER_STATUS_INSUFFICIENT_DATA,
    "OUT_OF_SCOPE": help_pb2.ANSWER_STATUS_OUT_OF_SCOPE,
    "CLARIFY": help_pb2.ANSWER_STATUS_CLARIFY,
}
BLOCK = {
    "SUMMARY": help_pb2.BLOCK_KIND_SUMMARY,
    "DETAILS": help_pb2.BLOCK_KIND_DETAILS,
    "LIMITATIONS": help_pb2.BLOCK_KIND_LIMITATIONS,
    "NEXT_STEPS": help_pb2.BLOCK_KIND_NEXT_STEPS,
    "CHANGES": help_pb2.BLOCK_KIND_CHANGES,
    "COMPARISON": help_pb2.BLOCK_KIND_COMPARISON,
}
ALLOWED_BLOCKS = {
    help_pb2.HELP_MODE_NEXT_STEP: {"SUMMARY", "DETAILS", "LIMITATIONS", "NEXT_STEPS"},
    help_pb2.HELP_MODE_REQUIREMENT: {"SUMMARY", "DETAILS", "LIMITATIONS", "NEXT_STEPS"},
    help_pb2.HELP_MODE_ASSESSMENT: {"SUMMARY", "DETAILS", "LIMITATIONS", "NEXT_STEPS"},
    help_pb2.HELP_MODE_COMPARE_OPTIONS: {"SUMMARY", "COMPARISON", "LIMITATIONS", "NEXT_STEPS"},
    help_pb2.HELP_MODE_DECISION: {"SUMMARY", "DETAILS", "LIMITATIONS", "NEXT_STEPS"},
    help_pb2.HELP_MODE_CASE_SUMMARY: set(BLOCK),
    help_pb2.HELP_MODE_CHANGES: {"SUMMARY", "CHANGES", "LIMITATIONS", "NEXT_STEPS"},
    help_pb2.HELP_MODE_QUESTION: set(BLOCK),
}
ALLOWED_TOPICS = {
    help_pb2.HELP_MODE_NEXT_STEP: {help_pb2.STATEMENT_TOPIC_NEXT_STEP, help_pb2.STATEMENT_TOPIC_PROFILE, help_pb2.STATEMENT_TOPIC_REQUIREMENT, help_pb2.STATEMENT_TOPIC_DATA_QUALITY},
    help_pb2.HELP_MODE_REQUIREMENT: {help_pb2.STATEMENT_TOPIC_REQUIREMENT, help_pb2.STATEMENT_TOPIC_POLICY, help_pb2.STATEMENT_TOPIC_DATA_QUALITY},
    help_pb2.HELP_MODE_ASSESSMENT: {help_pb2.STATEMENT_TOPIC_SCORE, help_pb2.STATEMENT_TOPIC_AFFORDABILITY, help_pb2.STATEMENT_TOPIC_POLICY, help_pb2.STATEMENT_TOPIC_DATA_QUALITY},
    help_pb2.HELP_MODE_COMPARE_OPTIONS: {help_pb2.STATEMENT_TOPIC_OPTION, help_pb2.STATEMENT_TOPIC_AFFORDABILITY, help_pb2.STATEMENT_TOPIC_DATA_QUALITY},
    help_pb2.HELP_MODE_DECISION: {help_pb2.STATEMENT_TOPIC_DECISION, help_pb2.STATEMENT_TOPIC_DATA_QUALITY},
    help_pb2.HELP_MODE_CHANGES: {help_pb2.STATEMENT_TOPIC_CHANGE, help_pb2.STATEMENT_TOPIC_DATA_QUALITY},
}


PRIMARY_TOPICS = {
    help_pb2.HELP_MODE_NEXT_STEP: {help_pb2.STATEMENT_TOPIC_NEXT_STEP, help_pb2.STATEMENT_TOPIC_PROFILE, help_pb2.STATEMENT_TOPIC_REQUIREMENT},
    help_pb2.HELP_MODE_REQUIREMENT: {help_pb2.STATEMENT_TOPIC_REQUIREMENT},
    help_pb2.HELP_MODE_ASSESSMENT: {help_pb2.STATEMENT_TOPIC_SCORE, help_pb2.STATEMENT_TOPIC_AFFORDABILITY, help_pb2.STATEMENT_TOPIC_POLICY},
    help_pb2.HELP_MODE_COMPARE_OPTIONS: {help_pb2.STATEMENT_TOPIC_OPTION},
    help_pb2.HELP_MODE_DECISION: {help_pb2.STATEMENT_TOPIC_DECISION},
    help_pb2.HELP_MODE_CHANGES: {help_pb2.STATEMENT_TOPIC_CHANGE},
}


def has_primary_context(request) -> bool:
    if request.mode == help_pb2.HELP_MODE_QUESTION:
        return bool(request.context.statements)
    if request.mode == help_pb2.HELP_MODE_COMPARE_OPTIONS:
        covered = {item.target_id for item in request.context.statements if item.topic == help_pb2.STATEMENT_TOPIC_OPTION}
        return all(key in covered for key in request.target_ids)
    if request.mode == help_pb2.HELP_MODE_CASE_SUMMARY:
        return any(item.topic != help_pb2.STATEMENT_TOPIC_DATA_QUALITY for item in request.context.statements)
    return any(relevant_statement(item, request) and item.topic in PRIMARY_TOPICS.get(request.mode, set()) for item in request.context.statements)


def relevant_statement(item, request) -> bool:
    if request.mode != help_pb2.HELP_MODE_CASE_SUMMARY and request.mode != help_pb2.HELP_MODE_QUESTION and item.topic not in ALLOWED_TOPICS.get(request.mode, set()):
        return False
    if request.target_ids and item.target_id and item.target_id not in request.target_ids and request.mode in (help_pb2.HELP_MODE_REQUIREMENT, help_pb2.HELP_MODE_COMPARE_OPTIONS, help_pb2.HELP_MODE_CHANGES, help_pb2.HELP_MODE_ASSESSMENT):
        return False
    return True


MODES = {
    "NEXT_STEP": help_pb2.HELP_MODE_NEXT_STEP,
    "REQUIREMENT": help_pb2.HELP_MODE_REQUIREMENT,
    "ASSESSMENT": help_pb2.HELP_MODE_ASSESSMENT,
    "COMPARE_OPTIONS": help_pb2.HELP_MODE_COMPARE_OPTIONS,
    "DECISION": help_pb2.HELP_MODE_DECISION,
    "CASE_SUMMARY": help_pb2.HELP_MODE_CASE_SUMMARY,
    "CHANGES": help_pb2.HELP_MODE_CHANGES,
    "QUESTION": help_pb2.HELP_MODE_QUESTION,
}


def _fail(message: str) -> None:
    raise InputError(message)


def _ids(items, kind: str) -> dict:
    result = {}
    for item in items:
        if not item.id or item.id in result:
            _fail(f"Invalid {kind} ID")
        result[item.id] = item
    return result


def _timestamp(value, name: str):
    if value.seconds == 0 and value.nanos == 0:
        _fail(f"Missing {name}")
    try:
        return value.ToDatetime(tzinfo=timezone.utc)
    except (ValueError, OverflowError) as exc:
        raise InputError(f"Invalid {name}") from exc


def _period(value: str, name: str) -> date:
    if not re.fullmatch(r"\d{4}-\d{2}-\d{2}", value):
        _fail(f"Invalid {name}")
    try:
        return date.fromisoformat(value)
    except ValueError as exc:
        raise InputError(f"Invalid {name}") from exc


def validate_request(request, catalog: dict, now) -> None:
    try:
        uuid.UUID(request.request_id)
        uuid.UUID(request.context.application_id)
    except ValueError as exc:
        raise InputError("Invalid UUID") from exc
    context = request.context
    if context.schema_version != 1:
        _fail("Unsupported schema version")
    if context.audience not in (help_pb2.AUDIENCE_CLIENT, help_pb2.AUDIENCE_ANALYST):
        _fail("Invalid audience")
    if context.catalog_version != catalog["version"]:
        _fail("Unsupported catalog version")
    if not re.fullmatch(r"[0-9a-f]{64}", context.context_id):
        _fail("Invalid context ID")
    if not context.source_revision or not context.policy_version:
        _fail("Missing source or policy version")
    if request.mode not in MODES.values() or request.detail not in (help_pb2.DETAIL_CONCISE, help_pb2.DETAIL_DETAILED):
        _fail("Invalid mode or detail")
    if request.locale != "ru-RU":
        _fail("Invalid locale")
    if context.freshness not in (help_pb2.FRESHNESS_CURRENT, help_pb2.FRESHNESS_PENDING, help_pb2.FRESHNESS_STALE):
        _fail("Invalid freshness")
    if not context.HasField("generated_at") or not context.HasField("valid_until"):
        _fail("Missing validity timestamp")
    generated = _timestamp(context.generated_at, "generated_at")
    valid = _timestamp(context.valid_until, "valid_until")
    if valid <= generated or valid <= now or (valid - generated).total_seconds() > 60 or generated > now:
        _fail("Expired or invalid context")
    if len(context.statements) > 80 or len(context.evidence) > 120 or len(context.targets) > 120:
        _fail("Context collection limit exceeded")
    if len(request.target_ids) > 3 or len(set(request.target_ids)) != len(request.target_ids):
        _fail("Invalid target selection")
    serialized = json.dumps(MessageToDict(request, preserving_proto_field_name=True), ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    if len(serialized) > 48 * 1024:
        _fail("Input too large")
    targets = _ids(context.targets, "target")
    evidence = _ids(context.evidence, "evidence")
    statements = _ids(context.statements, "statement")
    actions = _ids(context.actions, "action")
    for target in context.targets:
        if target.kind not in help_pb2.TargetKind.values() or target.kind == help_pb2.TARGET_KIND_UNSPECIFIED or not target.label:
            _fail("Invalid target")
    for item in context.evidence:
        if item.kind not in help_pb2.EvidenceKind.values() or item.state not in help_pb2.EvidenceState.values() or item.kind == help_pb2.EVIDENCE_KIND_UNSPECIFIED or item.state == help_pb2.EVIDENCE_STATE_UNSPECIFIED or not item.source_ref or not item.label or not item.HasField("observed_at"):
            _fail("Invalid evidence")
        _timestamp(item.observed_at, "observed_at")
        start = _period(item.period_from, "period_from") if item.HasField("period_from") else None
        end = _period(item.period_to, "period_to") if item.HasField("period_to") else None
        if start and end and start > end:
            _fail("Invalid period range")
    for item in context.statements:
        if item.topic not in help_pb2.StatementTopic.values() or item.topic == help_pb2.STATEMENT_TOPIC_UNSPECIFIED or item.priority > 100:
            _fail("Invalid statement")
        if item.target_id and item.target_id not in targets:
            _fail("Unknown statement target")
        if not item.target_id and not item.required:
            _fail("Untargeted optional statement")
        variants = {variant.id: variant for variant in item.variants}
        if len(variants) != len(item.variants) or set(variants) != {"concise", "detailed"} or any(not variant.text.strip() for variant in item.variants):
            _fail("Invalid statement variants")
        if item.target_id and not item.evidence_ids:
            _fail("Missing evidence")
        if any(key not in evidence for key in item.evidence_ids) or len(set(item.evidence_ids)) != len(item.evidence_ids):
            _fail("Invalid evidence reference")
        if any(key not in statements for key in item.companion_ids) or len(set(item.companion_ids)) != len(item.companion_ids):
            _fail("Invalid companion reference")
    visiting = set()
    visited = set()
    def visit(key):
        if key in visiting:
            _fail("Companion cycle")
        if key in visited:
            return
        visiting.add(key)
        for linked in statements[key].companion_ids:
            visit(linked)
        visiting.remove(key)
        visited.add(key)
    for key in statements:
        visit(key)
    for item in context.actions:
        if item.kind not in help_pb2.ActionKind.values() or item.kind == help_pb2.ACTION_KIND_UNSPECIFIED or item.target_id not in targets or not item.label:
            _fail("Invalid action")
    if any(key not in targets for key in request.target_ids):
        _fail("Unknown selected target")
    if request.mode == help_pb2.HELP_MODE_QUESTION:
        if not 1 <= len(request.question) <= 1000:
            _fail("Invalid question")
    elif request.question:
        _fail("Question only allowed in QUESTION mode")
    if request.mode == help_pb2.HELP_MODE_REQUIREMENT and (len(request.target_ids) != 1 or targets[request.target_ids[0]].kind != help_pb2.TARGET_KIND_REQUIREMENT):
        _fail("REQUIREMENT needs one requirement target")
    if request.mode == help_pb2.HELP_MODE_COMPARE_OPTIONS and (not context.HasField("calculation_id") or not 2 <= len(request.target_ids) <= 3 or any(targets[key].kind != help_pb2.TARGET_KIND_OPTION for key in request.target_ids)):
        _fail("COMPARE_OPTIONS needs options from a calculation")
    if request.mode == help_pb2.HELP_MODE_CHANGES and (len(request.target_ids) != 1 or targets[request.target_ids[0]].kind != help_pb2.TARGET_KIND_CHANGE):
        _fail("CHANGES needs one change target")
    if request.mode == help_pb2.HELP_MODE_ASSESSMENT and (len(request.target_ids) > 1 or any(targets[key].kind != help_pb2.TARGET_KIND_FACTOR for key in request.target_ids)):
        _fail("ASSESSMENT target must be a factor")
    if request.mode == help_pb2.HELP_MODE_CASE_SUMMARY and context.audience != help_pb2.AUDIENCE_ANALYST:
        _fail("CASE_SUMMARY is analyst only")


def validate_plan(plan: dict, request) -> None:
    if not isinstance(plan, dict) or set(plan) != {"status", "blocks", "action_ids", "clarify_target_ids"}:
        raise PlanError("Invalid plan shape")
    if type(plan["status"]) is not str or plan["status"] not in STATUS:
        raise PlanError("Invalid status")
    if not isinstance(plan["blocks"], list) or len(plan["blocks"]) > 6:
        raise PlanError("Invalid blocks")
    statements = {item.id: item for item in request.context.statements}
    actions = {item.id for item in request.context.actions}
    targets = {item.id for item in request.context.targets}
    count = 0
    selected = set()
    selected_topics = set()
    for block in plan["blocks"]:
        if not isinstance(block, dict) or set(block) != {"kind", "selections"} or type(block["kind"]) is not str or block["kind"] not in ALLOWED_BLOCKS[request.mode] or not isinstance(block["selections"], list):
            raise PlanError("Invalid block")
        for selection in block["selections"]:
            if not isinstance(selection, dict) or set(selection) != {"statement_id", "variant_id"}:
                raise PlanError("Invalid selection")
            key, variant = selection["statement_id"], selection["variant_id"]
            if type(key) is not str or key not in statements or type(variant) is not str or variant not in {v.id for v in statements[key].variants} or key in selected:
                raise PlanError("Invalid statement selection")
            selected.add(key)
            selected_topics.add(statements[key].topic)
            if not relevant_statement(statements[key], request):
                raise PlanError("Unrelated statement")
            count += 1
    if count > 18:
        raise PlanError("Too many selections")
    if not isinstance(plan["action_ids"], list) or len(plan["action_ids"]) > 3 or any(type(key) is not str or key not in actions for key in plan["action_ids"]) or len(set(plan["action_ids"])) != len(plan["action_ids"]):
        raise PlanError("Invalid action IDs")
    if not isinstance(plan["clarify_target_ids"], list) or len(plan["clarify_target_ids"]) > 3 or any(type(key) is not str or key not in targets for key in plan["clarify_target_ids"]) or len(set(plan["clarify_target_ids"])) != len(plan["clarify_target_ids"]):
        raise PlanError("Invalid clarification targets")
    if plan["status"] == "CLARIFY":
        if not plan["clarify_target_ids"]:
            raise PlanError("Missing clarification targets")
    elif plan["clarify_target_ids"]:
        raise PlanError("Unexpected clarification targets")
    if plan["status"] in ("OUT_OF_SCOPE", "INSUFFICIENT_DATA") and plan["action_ids"]:
        raise PlanError("Actions without supported answer")
    if plan["status"] == "ANSWERED":
        primary = PRIMARY_TOPICS.get(request.mode)
        chosen_primary = any(statements[key].topic in primary for key in selected) if primary is not None else bool(selected)
        required_primary = any(item.required and relevant_statement(item, request) and item.topic in primary for item in request.context.statements) if primary is not None else False
        if count == 0 or not has_primary_context(request) or not (chosen_primary or required_primary):
            raise PlanError("Empty answer")
    if plan["status"] == "PARTIAL" and any(item.topic == help_pb2.STATEMENT_TOPIC_DATA_QUALITY for item in request.context.statements) and help_pb2.STATEMENT_TOPIC_DATA_QUALITY not in selected_topics and not any(item.required and item.topic == help_pb2.STATEMENT_TOPIC_DATA_QUALITY for item in request.context.statements):
        raise PlanError("Missing limitation")
