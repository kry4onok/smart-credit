"""Wire compatibility: optional zero survives serialization for both producers."""

import pytest

from proto.bureau import bureau_pb2
from proto.scoring import scoring_pb2


@pytest.mark.parametrize(
    "message_type,fields",
    [
        (scoring_pb2.Features, ("company_age_months", "overdue_count_12m", "max_overdue_days")),
        (bureau_pb2.CreditHistoryResponse, ("overdue_count_12m", "max_overdue_days")),
        (scoring_pb2.ScoreResponse, ("score",)),
    ],
)
def test_optional_known_zero_survives_binary_roundtrip(message_type, fields):
    absent = message_type.FromString(message_type().SerializeToString())
    known = message_type.FromString(message_type(**dict.fromkeys(fields, 0)).SerializeToString())
    for field in fields:
        assert not absent.HasField(field)
        assert known.HasField(field) and getattr(known, field) == 0


def test_old_response_bytes_remain_readable_without_faking_assessment():
    # Legacy score=720, grade=B. New consumers must require explicit assessment status.
    response = scoring_pb2.ScoreResponse.FromString(bytes.fromhex("08d0051002"))
    assert response.HasField("score") and response.score == 720
    assert response.grade == scoring_pb2.GRADE_B
    assert response.assessment_status == scoring_pb2.ASSESSMENT_STATUS_UNSPECIFIED
