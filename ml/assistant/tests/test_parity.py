import json
import unittest
from datetime import datetime
from pathlib import Path

from google.protobuf.json_format import MessageToDict, ParseDict
from proto.assistant import help_pb2 as pb
from help.render import fallback, render_plan
from help.validation import InputError, validate_request, validate_plan

ROOT = Path(__file__).resolve().parents[3]


class CrossLanguageFixtures(unittest.TestCase):
    def test_complete_contexts_match_go_goldens(self):
        fixtures = json.loads((ROOT / "pkg/assistanthelp/testdata/parity.json").read_text())
        catalog = json.loads((ROOT / "pkg/assistanthelp/catalog.json").read_text())
        self.assertEqual(len(fixtures), 30)
        for case in fixtures:
            with self.subTest(case=case["id"]):
                request = ParseDict(case["request"], pb.HelpRequest())
                now = datetime.fromisoformat(case["now"])
                if "expected_error" in case:
                    with self.assertRaises(InputError):
                        validate_request(request, catalog, now)
                    continue
                validate_request(request, catalog, now)
                if case["operation"] == "render":
                    validate_plan(case["plan"], request)
                    actual = render_plan(case["plan"], request, catalog, pb.RESPONSE_SOURCE_MODEL,
                                         "fixture-model", "NONE", now)
                else:
                    actual = fallback(request, catalog, case["reason"], now)
                self.assertEqual(MessageToDict(actual, preserving_proto_field_name=True), case["expected"])
                self.assertEqual([s.statement_id for b in actual.blocks for s in b.items if s.statement_id],
                                 case["expected_statement_ids"])
