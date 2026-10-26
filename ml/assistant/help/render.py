from datetime import timezone
from proto.assistant import help_pb2

from .validation import BLOCK, STATUS, has_primary_context, relevant_statement


_REASON = {
    "NONE": help_pb2.FALLBACK_REASON_NONE,
    "MODEL_UNAVAILABLE": help_pb2.FALLBACK_REASON_MODEL_UNAVAILABLE,
    "MODEL_TIMEOUT": help_pb2.FALLBACK_REASON_MODEL_TIMEOUT,
    "INVALID_MODEL_OUTPUT": help_pb2.FALLBACK_REASON_INVALID_MODEL_OUTPUT,
    "QUEUE_FULL": help_pb2.FALLBACK_REASON_QUEUE_FULL,
    "INPUT_TOO_LARGE": help_pb2.FALLBACK_REASON_INPUT_TOO_LARGE,
    "NO_SUPPORTED_CONTEXT": help_pb2.FALLBACK_REASON_NO_SUPPORTED_CONTEXT,
}


def expand_required(plan: dict, request) -> dict:
    if plan["status"] in ("OUT_OF_SCOPE", "INSUFFICIENT_DATA", "CLARIFY"):
        return plan
    statements = {item.id: item for item in request.context.statements}
    blocks = [{"kind": item["kind"], "selections": list(item["selections"])} for item in plan["blocks"]]
    seen = {selection["statement_id"] for block in blocks for selection in block["selections"]}
    pending = [item.id for item in request.context.statements if item.required and relevant_statement(item, request)]
    pending += [selection["statement_id"] for block in blocks for selection in block["selections"]]
    while pending:
        key = pending.pop(0)
        item = statements[key]
        if key not in seen:
            kind = "LIMITATIONS" if item.topic == help_pb2.STATEMENT_TOPIC_DATA_QUALITY else "DETAILS"
            block = next((block for block in blocks if block["kind"] == kind), None)
            if block is None:
                block = {"kind": kind, "selections": []}
                blocks.append(block)
            variant = "detailed" if request.detail == help_pb2.DETAIL_DETAILED else "concise"
            block["selections"].append({"statement_id": key, "variant_id": variant})
            seen.add(key)
        pending.extend(linked for linked in item.companion_ids if linked not in seen)
    return {**plan, "blocks": blocks}


def fallback_plan(request) -> dict:
    if request.mode == help_pb2.HELP_MODE_QUESTION:
        targets = [item.id for item in request.context.targets[:3]]
        next_step = next((item for item in request.context.statements if item.topic == help_pb2.STATEMENT_TOPIC_NEXT_STEP), None)
        blocks = [{"kind": "NEXT_STEPS", "selections": [{"statement_id": next_step.id, "variant_id": "concise"}]}] if next_step else []
        return {"status": "CLARIFY" if targets else "INSUFFICIENT_DATA", "blocks": blocks, "action_ids": [], "clarify_target_ids": targets}
    if not has_primary_context(request):
        return {"status": "INSUFFICIENT_DATA", "blocks": [], "action_ids": [], "clarify_target_ids": []}
    selected = []
    chosen = set(request.target_ids)
    items = sorted(request.context.statements, key=lambda item: (not item.required, item.target_id not in chosen, -item.priority, item.id))
    for item in items:
        if relevant_statement(item, request) and (item.required or not chosen or item.target_id in chosen):
            selected.append({"statement_id": item.id, "variant_id": "detailed" if request.detail == help_pb2.DETAIL_DETAILED else "concise"})
        if len(selected) >= 18:
            break
    if not selected:
        return {"status": "INSUFFICIENT_DATA", "blocks": [], "action_ids": [], "clarify_target_ids": []}
    statements = {item.id: item for item in request.context.statements}
    main = [item for item in selected if statements[item["statement_id"]].topic != help_pb2.STATEMENT_TOPIC_DATA_QUALITY]
    limits = [item for item in selected if statements[item["statement_id"]].topic == help_pb2.STATEMENT_TOPIC_DATA_QUALITY]
    kind = "SUMMARY" if request.mode == help_pb2.HELP_MODE_CASE_SUMMARY else "DETAILS"
    blocks = []
    if main:
        blocks.append({"kind": kind, "selections": main})
    if limits:
        blocks.append({"kind": "LIMITATIONS", "selections": limits})
    status = "PARTIAL" if request.mode == help_pb2.HELP_MODE_CASE_SUMMARY and limits else "ANSWERED"
    return {"status": status, "blocks": blocks, "action_ids": [], "clarify_target_ids": []}


def _plan_proto(plan: dict):
    result = help_pb2.AnswerPlan(status=STATUS[plan["status"]])
    for block in plan["blocks"]:
        result.blocks.add(kind=BLOCK[block["kind"]], selections=[help_pb2.Selection(**selection) for selection in block["selections"]])
    result.action_ids.extend(plan["action_ids"])
    result.clarify_target_ids.extend(plan["clarify_target_ids"])
    return result


def render_plan(plan: dict, request, catalog: dict, source: int, model_version: str, reason: str, now) -> help_pb2.HelpResponse:
    expanded = expand_required(plan, request)
    context = request.context
    statements = {item.id: item for item in context.statements}
    actions = {item.id: item for item in context.actions}
    targets = {item.id: item for item in context.targets}
    evidence = {item.id: item for item in context.evidence}
    response = help_pb2.HelpResponse(
        request_id=request.request_id,
        context_id=context.context_id,
        source_revision=context.source_revision,
        valid_until=context.valid_until,
        status=STATUS[expanded["status"]],
        source=source,
        model_version=model_version,
        prompt_version="help-plan-v1",
        catalog_version=catalog["version"],
        fallback_reason=_REASON[reason],
        plan=_plan_proto(expanded),
    )
    response.generated_at.FromDatetime(now.astimezone(timezone.utc))
    used = set()
    for block in expanded["blocks"]:
        rendered = response.blocks.add(kind=BLOCK[block["kind"]])
        for selection in block["selections"]:
            statement = statements[selection["statement_id"]]
            text = next(variant.text for variant in statement.variants if variant.id == selection["variant_id"])
            rendered.items.add(statement_id=statement.id, text=text, evidence_ids=statement.evidence_ids)
            used.update(statement.evidence_ids)
    if expanded["status"] in ("CLARIFY", "INSUFFICIENT_DATA", "OUT_OF_SCOPE"):
        response.blocks.add(kind=help_pb2.BLOCK_KIND_SUMMARY).items.add(text=catalog["phrases"][expanded["status"]])
    response.actions.extend(actions[key] for key in expanded["action_ids"])
    response.clarify_targets.extend(targets[key] for key in expanded["clarify_target_ids"])
    response.evidence.extend(evidence[key] for key in sorted(used))
    limit = 1800 if context.audience == help_pb2.AUDIENCE_CLIENT else 5000
    if sum(len(item.text) for block in response.blocks for item in block.items) > limit:
        response.ClearField("blocks")
        response.ClearField("evidence")
        response.ClearField("actions")
        response.ClearField("clarify_targets")
        response.blocks.add(kind=help_pb2.BLOCK_KIND_SUMMARY).items.add(text=catalog["phrases"]["LIMITED"])
        response.status = help_pb2.ANSWER_STATUS_PARTIAL
        response.source = help_pb2.RESPONSE_SOURCE_TEMPLATE
        response.model_version = ""
        response.plan.CopyFrom(_plan_proto({"status": "PARTIAL", "blocks": [], "action_ids": [], "clarify_target_ids": []}))
    return response


def fallback(request, catalog: dict, reason: str, now) -> help_pb2.HelpResponse:
    return render_plan(fallback_plan(request), request, catalog, help_pb2.RESPONSE_SOURCE_TEMPLATE, "", reason, now)


def limited_fallback(request, catalog: dict, now) -> help_pb2.HelpResponse:
    plan = {"status": "INSUFFICIENT_DATA", "blocks": [], "action_ids": [], "clarify_target_ids": []}
    return render_plan(plan, request, catalog, help_pb2.RESPONSE_SOURCE_TEMPLATE, "", "INPUT_TOO_LARGE", now)
