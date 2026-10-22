#!/usr/bin/env python3
# /// script
# requires-python = ">=3.12,<3.13"
# dependencies = ["grpcio-tools==1.78.0", "protobuf==6.33.5"]
# ///
"""Generate all shared contracts with pinned tools; never patch generated imports."""

import argparse
import importlib.metadata
import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path

import grpc_tools

ROOT = Path(__file__).resolve().parents[1]
TOOLS = json.loads((ROOT / "scripts/proto-tools.json").read_text())
SOURCES = sorted(str(path.relative_to(ROOT)) for path in (ROOT / "proto").glob("*/*.proto"))


def run(*args: str, env: dict[str, str] | None = None) -> None:
    subprocess.run(args, cwd=ROOT, check=True, env=env)


def generate(output: Path, language: str) -> None:
    actual = importlib.metadata.version("grpcio-tools")
    if actual != TOOLS["grpcio-tools"]:
        raise SystemExit(f"grpcio-tools {TOOLS['grpcio-tools']} required; found {actual}")
    include = Path(grpc_tools.__file__).parent / "_proto"
    args = [sys.executable, "-m", "grpc_tools.protoc", "-I.", f"-I{include}"]
    destination = output / language
    destination.mkdir(parents=True, exist_ok=True)
    if language == "python":
        args += [f"--python_out={destination}", f"--grpc_python_out={destination}"]
    else:
        binary_dir = ROOT / ".tools/bin"
        binary_dir.mkdir(parents=True, exist_ok=True)
        env = dict(os.environ, GOBIN=str(binary_dir))
        plugins = {
            "protoc-gen-go": "google.golang.org/protobuf/cmd/protoc-gen-go",
            "protoc-gen-go-grpc": "google.golang.org/grpc/cmd/protoc-gen-go-grpc",
        }
        for name, module in plugins.items():
            version = TOOLS[name]
            binary = binary_dir / name
            installed = (
                subprocess.check_output([str(binary), "--version"], text=True).strip()
                if binary.exists()
                else ""
            )
            if installed.split()[-1:] != [version] and installed.split()[-1:] != [version[1:]]:
                run("go", "install", f"{module}@{version}", env=env)
            args += [f"--plugin={name}={binary}"]
        for plugin in ("go", "go-grpc"):
            args += [
                f"--{plugin}_out={destination}",
                f"--{plugin}_opt=module={TOOLS['go_module']}/proto/gen/go",
            ]
    run(*args, *SOURCES)
    if language == "python":
        for directory in sorted({p.parent for p in destination.rglob("*.py")}):
            current = directory
            while current != destination:
                (current / "__init__.py").write_text("# Generated package marker.\n")
                current = current.parent


def file_map(directory: Path) -> dict[str, bytes]:
    return {
        str(path.relative_to(directory)): path.read_bytes()
        for path in directory.rglob("*")
        if path.is_file() and "__pycache__" not in path.parts and path.suffix != ".pyc"
    }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--language", choices=("all", "go", "python"), default="all")
    parser.add_argument("--check", action="store_true", help="Fail on generated-code drift")
    options = parser.parse_args()
    languages = ("go", "python") if options.language == "all" else (options.language,)
    with tempfile.TemporaryDirectory(prefix="smart-credit-proto-") as temporary:
        target = Path(temporary)
        for language in languages:
            generate(target, language)
            expected = file_map(target / language)
            destination = ROOT / "proto/gen" / language
            actual = file_map(destination)
            if options.check:
                changes = sorted(
                    k for k in actual.keys() | expected.keys() if actual.get(k) != expected.get(k)
                )
                if changes:
                    raise SystemExit("Generated contract drift: " + ", ".join(changes))
            else:
                for removed in actual.keys() - expected.keys():
                    (destination / removed).unlink()
                for name, content in expected.items():
                    path = destination / name
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_bytes(content)
            action = "checked" if options.check else "written"
            print(f"{language}: {len(expected)} generated files {action}")


if __name__ == "__main__":
    main()
