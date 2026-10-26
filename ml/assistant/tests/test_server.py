import sys
import json
import unittest
from io import BytesIO
from pathlib import Path
from unittest.mock import patch

import grpc

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import server
from proto.assistant import assistant_pb2


class Context:
    def time_remaining(self):
        return 30.0

    def abort(self, code, details):
        raise grpc.RpcError((code, details))


class AssistantTests(unittest.TestCase):
    def setUp(self):
        self.service = server.AssistantService()
        self.context = Context()

    def test_summary_accepts_grounded_text(self):
        request = assistant_pb2.SummarizeCaseRequest(
            product_name="Потребительский кредит", purpose="ремонт квартиры", grade="B"
        )
        text = "Продукт: Потребительский кредит. Цель: ремонт квартиры. Грейд: B."
        with patch.object(server, "generate_json", return_value=({"text": text}, "qwen2.5:7b")):
            result = self.service.SummarizeCase(request, self.context)
        self.assertEqual(result.text, text)
        self.assertEqual(result.model_version, "qwen2.5:7b")

    def test_summary_uses_verified_descriptions_without_guessing_codes(self):
        request = assistant_pb2.SummarizeCaseRequest(
            product_name="Оборотный кредит",
            purpose="WORKING_CAPITAL",
            purpose_description="Пополнение оборотных средств",
            status="MANUAL_REVIEW",
            status_description="Ручное рассмотрение",
            grade="B",
            reason_codes=["NO_OVERDUE"],
            reason_facts=[assistant_pb2.SummaryFact(code="NO_OVERDUE", description="Нет просрочек")],
            triggered_rule_codes=["REVIEW_REQUIRED"],
            triggered_rule_facts=[
                assistant_pb2.SummaryFact(
                    code="REVIEW_REQUIRED",
                    description="Необходим ручной разбор; конкретная причина этим кодом не указана",
                )
            ],
        )
        text = (
            "Оборотный кредит: Пополнение оборотных средств. Ручное рассмотрение, грейд B. "
            "Нет просрочек. Нужен ручной разбор."
        )
        with patch.object(server, "generate_json", return_value=({"text": text}, "qwen2.5:7b")):
            result = self.service.SummarizeCase(request, self.context)
        self.assertEqual(result.model_version, "qwen2.5:7b")
        self.assertEqual(result.text, text)

        guessed = text + " Кредит одобрен."
        with patch.object(server, "generate_json", return_value=({"text": guessed}, "qwen2.5:7b")):
            rejected = self.service.SummarizeCase(request, self.context)
        self.assertEqual(rejected.model_version, "template")
        self.assertIn("Нужен ручной разбор", rejected.text)
        self.assertNotIn("конкретная причина", rejected.text)
        self.assertNotIn("одобрен", rejected.text)

    def test_summary_prompt_omits_missing_score_and_raw_known_codes(self):
        request = assistant_pb2.SummarizeCaseRequest(
            product_name="Оборотный кредит",
            purpose="WORKING_CAPITAL",
            purpose_description="Пополнение оборотных средств",
            status="NEEDS_INFO",
            status_description="Запрошены дополнительные данные",
        )
        text = "Продукт: Оборотный кредит. Цель: Пополнение оборотных средств. Статус: Запрошены дополнительные данные."
        with patch.object(server, "generate_json", return_value=({"text": text}, "qwen2.5:7b")) as generate:
            result = self.service.SummarizeCase(request, self.context)
        prompt, facts = generate.call_args.args[:2]
        self.assertEqual(result.model_version, "qwen2.5:7b")
        self.assertNotIn("грейд", prompt)
        self.assertNotIn("факторы скоринга", prompt)
        self.assertIn("связное резюме", prompt)
        self.assertIn("Не перечисляй поля", prompt)
        self.assertNotIn("'Продукт: <product>.'", prompt)
        self.assertNotIn("WORKING_CAPITAL", json.dumps(facts))
        self.assertNotIn("NEEDS_INFO", json.dumps(facts))

    def test_summary_rejects_claim_about_missing_rules(self):
        request = assistant_pb2.SummarizeCaseRequest(
            product_name="Потребительский кредит", purpose="ремонт квартиры", grade="B"
        )
        invented = "Потребительский кредит, ремонт квартиры, B. Правила не применялись."
        with patch.object(server, "generate_json", return_value=({"text": invented}, "qwen2.5:7b")):
            result = self.service.SummarizeCase(request, self.context)
        self.assertEqual(result.model_version, "template")
        self.assertNotIn("Правила", result.text)
        self.assertNotIn("причин", result.text)

    def test_summary_rejects_invented_decision(self):
        request = assistant_pb2.SummarizeCaseRequest(product_name="Кредит", grade="B")
        with patch.object(server, "generate_json", return_value=({"text": "Кредит одобрен, B."}, "qwen2.5:7b")):
            result = self.service.SummarizeCase(request, self.context)
        self.assertEqual(result.model_version, "template")
        self.assertNotIn("одобрен", result.text)

    def test_rejection_uses_exact_public_reason(self):
        reason = "Подтверждённого дохода недостаточно для запрошенной суммы."
        request = assistant_pb2.ExplainDecisionRequest(
            outcome="REJECT", product_name="Потребительский кредит", analyst_public_reason=reason
        )
        with patch.object(server, "generate_json", return_value=({"style": "empathetic"}, "qwen2.5:7b")):
            result = self.service.ExplainDecision(request, self.context)
        self.assertIn("решение об отказе", result.text)
        self.assertIn(reason, result.text)
        self.assertNotIn("повторно", result.text)
        self.assertEqual(result.model_version, "qwen2.5:7b")

    def test_ollama_failure_returns_safe_template(self):
        request = assistant_pb2.ExplainDecisionRequest(
            outcome="REJECT", analyst_public_reason="Недостаточно подтверждённого дохода."
        )
        with patch.object(server, "generate_json", side_effect=server.ModelUnavailable("offline")):
            result = self.service.ExplainDecision(request, self.context)
        self.assertEqual(result.model_version, "template")
        self.assertIn("отказе", result.text)
        self.assertIn("Недостаточно подтверждённого дохода.", result.text)

    def test_counter_includes_only_approved_offer_terms(self):
        request = assistant_pb2.ExplainDecisionRequest(
            outcome="COUNTER", product_name="Оборотный кредит",
            analyst_public_reason="Предлагаем меньшую сумму по другому продукту.",
            selected_offer=assistant_pb2.DecisionOffer(
                product_name="Гибкий кредит", amount_kopecks=400000050,
                term_months=24, rate_bps=1850,
            ),
        )
        with patch.object(server, "generate_json", return_value=({"style": "neutral"}, "qwen2.5:7b")):
            result = self.service.ExplainDecision(request, self.context)
        self.assertEqual(result.model_version, "qwen2.5:7b")
        for fragment in ("Оборотный кредит", "Гибкий кредит", "4 000 000,50 ₽", "24 мес.", "18,50%"):
            self.assertIn(fragment, result.text)

        with patch.object(server, "generate_json", side_effect=server.ModelUnavailable("offline")):
            fallback = self.service.ExplainDecision(request, self.context)
        self.assertEqual(fallback.model_version, "template")
        for fragment in ("Гибкий кредит", "4 000 000,50 ₽", "24 мес.", "18,50%"):
            self.assertIn(fragment, fallback.text)

    def test_invalid_offer_is_rejected(self):
        request = assistant_pb2.ExplainDecisionRequest(
            outcome="COUNTER",
            selected_offer=assistant_pb2.DecisionOffer(
                product_name="Гибкий кредит", amount_kopecks=0, term_months=24, rate_bps=1850,
            ),
        )
        with self.assertRaises(grpc.RpcError):
            self.service.ExplainDecision(request, self.context)

    def test_summary_rejects_invented_number(self):
        request = assistant_pb2.SummarizeCaseRequest(product_name="Кредит", grade="B")
        invented = "Продукт: Кредит. Грейд скоринга: B. Ставка 12%."
        with patch.object(server, "generate_json", return_value=({"text": invented}, "qwen2.5:7b")):
            result = self.service.SummarizeCase(request, self.context)
        self.assertEqual(result.model_version, "template")
        self.assertNotIn("12", result.text)

    def test_summary_rejects_grade_when_score_is_missing(self):
        request = assistant_pb2.SummarizeCaseRequest(
            product_name="Оборотный кредит", purpose="WORKING_CAPITAL"
        )
        invented = "Продукт: Оборотный кредит. Цель: WORKING_CAPITAL. Грейд скоринга: B."
        with patch.object(server, "generate_json", return_value=({"text": invented}, "qwen2.5:7b")):
            result = self.service.SummarizeCase(request, self.context)
        self.assertEqual(result.model_version, "template")
        self.assertNotIn("Грейд", result.text)

    def test_ollama_transport_uses_custom_url_model_and_key(self):
        answer = BytesIO(json.dumps({"response": '{"style":"neutral"}', "model": "local"}).encode())
        with patch.multiple(server, LLM_PROVIDER="ollama", LLM_BASE_URL="http://localhost:11434", LLM_MODEL="local", LLM_API_KEY="secret"):
            with patch.object(server, "urlopen", return_value=answer) as open_mock:
                data, version = server.generate_json("Выбери стиль", {"outcome": "COUNTER"}, server.STYLE_FORMAT, 5)
        request = open_mock.call_args.args[0]
        self.assertEqual(request.full_url, "http://localhost:11434/api/generate")
        self.assertEqual(request.get_header("Authorization"), "Bearer secret")
        self.assertEqual(json.loads(request.data)["model"], "local")
        self.assertEqual((data, version), ({"style": "neutral"}, "local"))

    def test_openai_compatible_transport_uses_base_url_model_and_key(self):
        answer = BytesIO(json.dumps({"choices": [{"message": {"content": '{"style":"empathetic"}'}}], "model": "remote"}).encode())
        with patch.multiple(server, LLM_PROVIDER="openai", LLM_BASE_URL="https://example.test/v1", LLM_MODEL="remote", LLM_API_KEY="secret"):
            with patch.object(server, "urlopen", return_value=answer) as open_mock:
                data, version = server.generate_json("Выбери стиль", {"outcome": "REJECT"}, server.STYLE_FORMAT, 5)
        request = open_mock.call_args.args[0]
        payload = json.loads(request.data)
        self.assertEqual(request.full_url, "https://example.test/v1/chat/completions")
        self.assertEqual(request.get_header("Authorization"), "Bearer secret")
        self.assertEqual(payload["model"], "remote")
        self.assertEqual((data, version), ({"style": "empathetic"}, "remote"))

    def test_provider_rejects_oversized_response(self):
        with patch.object(server, "urlopen", return_value=BytesIO(b"x" * 65537)):
            with self.assertRaises(server.ModelUnavailable):
                server.generate_json("prompt", {}, server.STYLE_FORMAT, 1)

    def test_all_decision_outcomes_use_known_status(self):
        expected = {
            "APPROVE": "одобрении",
            "COUNTER": "встречное предложение",
            "REQUEST_INFO": "дополнительные сведения",
            "MANUAL_REVIEW": "дополнительное рассмотрение",
            "REJECT": "отказе",
        }
        with patch.object(server, "generate_json", return_value=({"style": "neutral"}, "qwen2.5:7b")):
            for outcome, phrase in expected.items():
                with self.subTest(outcome=outcome):
                    request = assistant_pb2.ExplainDecisionRequest(outcome=outcome)
                    result = self.service.ExplainDecision(request, self.context)
                    self.assertIn(phrase, result.text)
                    self.assertNotIn("повторно", result.text)

    def test_unknown_outcome_is_rejected(self):
        request = assistant_pb2.ExplainDecisionRequest(outcome="DISBURSED")
        with self.assertRaises(grpc.RpcError):
            self.service.ExplainDecision(request, self.context)

    def test_all_bank_demo_scenarios_work_without_model(self):
        scenarios = json.loads((Path(__file__).resolve().parents[1] / "scenarios.json").read_text())
        seed = (Path(__file__).resolve().parents[3] / "services/bank-service/seeds/demo.sql").read_text()
        self.assertEqual([case["number"] for case in scenarios], list(range(1, 8)))
        with patch.object(server, "generate_json", side_effect=server.ModelUnavailable("offline")):
            for case in scenarios:
                with self.subTest(case=case["number"]):
                    fields = case["summary"]
                    self.assertIn(fields["application_id"], seed)
                    summary = self.service.SummarizeCase(assistant_pb2.SummarizeCaseRequest(**fields), self.context)
                    self.assertEqual(summary.model_version, "template")
                    self.assertEqual(summary.text, server.summary_template(server.summary_facts(assistant_pb2.SummarizeCaseRequest(**fields))))
                    self.assertNotIn("Продукт:", summary.text)
                    self.assertNotIn("Статус:", summary.text)
                    if explanation_fields := case.get("explanation"):
                        explanation = self.service.ExplainDecision(
                            assistant_pb2.ExplainDecisionRequest(**explanation_fields), self.context
                        )
                        self.assertEqual(explanation.model_version, "template")
                        self.assertIn(explanation_fields["analyst_public_reason"], explanation.text)


if __name__ == "__main__":
    unittest.main()
