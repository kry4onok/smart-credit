import json
from pathlib import Path


class CatalogError(ValueError):
    pass


_REQUIRED_PHRASES = {"CLARIFY", "INSUFFICIENT_DATA", "OUT_OF_SCOPE", "LIMITED", "QUESTION_FALLBACK"}


def load_catalog(path: str | Path) -> dict:
    data = json.loads(Path(path).read_text(encoding="utf-8"))
    if not isinstance(data, dict) or set(data) != {"version", "phrases"}:
        raise CatalogError("Invalid catalog structure")
    if not isinstance(data["version"], str) or not data["version"]:
        raise CatalogError("Invalid catalog version")
    phrases = data["phrases"]
    if not isinstance(phrases, dict) or set(phrases) != _REQUIRED_PHRASES:
        raise CatalogError("Invalid catalog phrases")
    if any(not isinstance(value, str) or not value.strip() for value in phrases.values()):
        raise CatalogError("Invalid catalog phrase")
    return data
