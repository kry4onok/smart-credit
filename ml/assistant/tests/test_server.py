import sys
import unittest
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
        with patch.object(server, "generate_json", return_value=({"text": text}, "qwen2.5:3b")):
            result = self.service.SummarizeCase(request, self.context)
        self.assertEqual(result.text, text)
        self.assertEqual(result.model_version, "qwen2.5:3b")

    def test_summary_rejects_claim_about_missing_rules(self):
        request = assistant_pb2.SummarizeCaseRequest(
            product_name="Потребительский кредит", purpose="ремонт квартиры", grade="B"
        )
        invented = "Потребительский кредит, ремонт квартиры, B. Правила не применялись."
        with patch.object(server, "generate_json", return_value=({"text": invented}, "qwen2.5:3b")):
            result = self.service.SummarizeCase(request, self.context)
        self.assertEqual(result.model_version, "template")
        self.assertNotIn("Правила", result.text)
        self.assertNotIn("причин", result.text)

    def test_summary_rejects_invented_decision(self):
        request = assistant_pb2.SummarizeCaseRequest(product_name="Кредит", grade="B")
        with patch.object(server, "generate_json", return_value=({"text": "Кредит одобрен, B."}, "qwen2.5:3b")):
            result = self.service.SummarizeCase(request, self.context)
        self.assertEqual(result.model_version, "template")
        self.assertNotIn("одобрен", result.text)

    def test_rejection_uses_exact_public_reason(self):
        reason = "Подтверждённого дохода недостаточно для запрошенной суммы."
        request = assistant_pb2.ExplainDecisionRequest(
            outcome="REJECT", product_name="Потребительский кредит", analyst_public_reason=reason
        )
        with patch.object(server, "generate_json", return_value=({"style": "empathetic"}, "qwen2.5:3b")):
            result = self.service.ExplainDecision(request, self.context)
        self.assertIn("решение об отказе", result.text)
        self.assertIn(reason, result.text)
        self.assertNotIn("повторно", result.text)
        self.assertEqual(result.model_version, "qwen2.5:3b")

    def test_ollama_failure_returns_safe_template(self):
        request = assistant_pb2.ExplainDecisionRequest(
            outcome="REJECT", analyst_public_reason="Недостаточно подтверждённого дохода."
        )
        with patch.object(server, "generate_json", side_effect=server.OllamaUnavailable("offline")):
            result = self.service.ExplainDecision(request, self.context)
        self.assertEqual(result.model_version, "template")
        self.assertIn("отказе", result.text)
        self.assertIn("Недостаточно подтверждённого дохода.", result.text)

    def test_summary_rejects_invented_number(self):
        request = assistant_pb2.SummarizeCaseRequest(product_name="Кредит", grade="B")
        invented = "Продукт: Кредит. Грейд скоринга: B. Ставка 12%."
        with patch.object(server, "generate_json", return_value=({"text": invented}, "qwen2.5:3b")):
            result = self.service.SummarizeCase(request, self.context)
        self.assertEqual(result.model_version, "template")
        self.assertNotIn("12", result.text)

    def test_all_decision_outcomes_use_known_status(self):
        expected = {
            "APPROVE": "одобрении",
            "COUNTER": "встречное предложение",
            "REQUEST_INFO": "дополнительные сведения",
            "MANUAL_REVIEW": "дополнительное рассмотрение",
            "REJECT": "отказе",
        }
        with patch.object(server, "generate_json", return_value=({"style": "neutral"}, "qwen2.5:3b")):
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


if __name__ == "__main__":
    unittest.main()
