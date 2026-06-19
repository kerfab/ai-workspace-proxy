#!/usr/bin/env python3
# Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
# Proprietary software. No use, copy, modification, distribution, disclosure,
# or reverse engineering is permitted without prior written authorization
# from Opensense Ltd.

"""Run registered AI Workspace Proxy tests from testing/test-coverage.json."""

import argparse
import glob
import json
import os
import re
import shlex
import shutil
import subprocess
import sys
from pathlib import Path


SCRIPT_DIR = Path(__file__).resolve().parent
REPO_ROOT = SCRIPT_DIR.parent
COVERAGE_FILE = SCRIPT_DIR / "test-coverage.json"


class RunnerError(Exception):
    pass


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Run tests registered in testing/test-coverage.json.",
    )
    parser.add_argument(
        "start_at",
        nargs="?",
        type=parse_positive_int,
        help="Optional 1-based registered test number to resume from.",
    )
    parser.add_argument(
        "--live",
        metavar="TEST_ACCOUNT_EMAIL",
        help="Run live tests against the provided Google Workspace test account email address.",
    )
    args = parser.parse_args()

    try:
        live_workspace = normalize_live_workspace(args.live)
    except argparse.ArgumentTypeError as exc:
        parser.error(str(exc))
    tests = load_registered_tests(include_live=bool(live_workspace))
    if not tests:
        print("No registered tests to run.")
        return 0

    start_at = args.start_at or 1
    if start_at > len(tests):
        print(
            f"ERROR: resume number {start_at} is greater than the total registered tests ({len(tests)}).",
            file=sys.stderr,
        )
        return 2

    total = len(tests)
    for position, test in enumerate(tests[start_at - 1 :], start=start_at):
        try:
            run_registered_test(test, live_workspace)
        except RunnerError as exc:
            print(format_failure(position, total, test, str(exc)), file=sys.stderr)
            return 1
        print(f'[{position}/{total}] SUCCESS: test "{test["test_name"]}" passed succseffuly.', flush=True)

    return 0


def parse_positive_int(raw: str) -> int:
    try:
        value = int(raw, 10)
    except ValueError as exc:
        raise argparse.ArgumentTypeError("must be a positive integer") from exc
    if value < 1:
        raise argparse.ArgumentTypeError("must be a positive integer")
    return value


def normalize_live_workspace(raw: str | None) -> str:
    if raw is None:
        return ""
    value = raw.strip()
    if not value:
        raise argparse.ArgumentTypeError("--live requires a non-empty test account email address")
    if "@" not in value:
        raise argparse.ArgumentTypeError("--live requires a test account email address")
    return value


def load_registered_tests(include_live: bool) -> list[dict]:
    try:
        data = json.loads(COVERAGE_FILE.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise RunnerError(f"Missing coverage registry: {COVERAGE_FILE}") from exc
    except json.JSONDecodeError as exc:
        raise RunnerError(f"Invalid JSON in {COVERAGE_FILE}: {exc}") from exc

    items = data.get("items")
    if not isinstance(items, list):
        raise RunnerError(f"{COVERAGE_FILE} must contain an items array")

    registered = []
    for item in items:
        if not isinstance(item, dict):
            continue
        item_tests = item.get("tests")
        if not isinstance(item_tests, list):
            continue
        for test in item_tests:
            if not isinstance(test, dict):
                raise RunnerError(f"Invalid test entry under {item.get('id', '<unknown item>')}: expected object")
            merged = dict(test)
            merged["coverage_item_id"] = item.get("id", "")
            merged["coverage_item_name"] = item.get("name", "")
            validate_test_entry(merged)
            if is_live_test(merged) and not include_live:
                continue
            registered.append(merged)
    return registered


def validate_test_entry(test: dict) -> None:
    for key in ("test_name", "test_summary", "test_script"):
        value = test.get(key)
        if not isinstance(value, str) or not value.strip():
            item_id = test.get("coverage_item_id") or "<unknown item>"
            raise RunnerError(f"Invalid registered test under {item_id}: missing non-empty {key}")
    if "live" in test and not isinstance(test["live"], bool):
        item_id = test.get("coverage_item_id") or "<unknown item>"
        raise RunnerError(f"Invalid registered test under {item_id}: live must be a boolean")


def is_live_test(test: dict) -> bool:
    return test.get("live") is True or test["test_name"].lower().startswith("live ")


def run_registered_test(test: dict, live_workspace: str) -> None:
    script = test["test_script"]
    name = test["test_name"]
    if script.endswith(".go"):
        run_go_unit_test(test, live_workspace)
        return
    if is_shell_glob(script):
        run_shell_syntax_glob(test, live_workspace)
        return
    if script.endswith(".sh"):
        run_command(test, ["sh", "-n", script], live_workspace)
        return
    if name.startswith("Python syntax check"):
        try:
            run_command(test, ["python3", "-m", "py_compile", script], live_workspace)
        finally:
            cleanup_pycache(Path(script).parent)
        return
    if script.endswith(".py"):
        try:
            run_command(test, ["python3", script], live_workspace)
        finally:
            cleanup_pycache(Path(script).parent)
        return

    path = REPO_ROOT / script
    if path.exists() and os.access(path, os.X_OK):
        run_command(test, [str(path)], live_workspace)
        return
    raise RunnerError(
        "Unsupported test registration. "
        "Use a Go *_test.go file, a Python script, a shell script, or an executable repository-relative file."
    )


def run_go_unit_test(test: dict, live_workspace: str) -> None:
    script_path = Path(test["test_script"])
    package_dir = script_path.parent
    package_arg = "./" + package_dir.as_posix() if package_dir.as_posix() != "." else "."
    pattern = "^" + re.escape(test["test_name"]) + "$"
    run_command(test, ["go", "test", package_arg, "-run", pattern, "-count=1"], live_workspace)


def run_shell_syntax_glob(test: dict, live_workspace: str) -> None:
    matches = sorted(glob.glob(str(REPO_ROOT / test["test_script"])))
    if not matches:
        raise RunnerError(f'No files matched shell test glob: {test["test_script"]}')
    for match in matches:
        rel = Path(match).relative_to(REPO_ROOT).as_posix()
        run_command(test, ["sh", "-n", rel], live_workspace)


def run_command(test: dict, command: list[str], live_workspace: str) -> None:
    env = os.environ.copy()
    if command and command[0] == "go":
        env.setdefault("GOCACHE", "/tmp/ai-workspace-proxy-go-build-cache")
    if live_workspace:
        env["AIWP_LIVE_TESTS"] = "1"
        env["AIWP_TEST_WORKSPACE"] = live_workspace
    else:
        env.pop("AIWP_LIVE_TESTS", None)
    env["AIWP_COVERAGE_ITEM_ID"] = test.get("coverage_item_id", "")
    env["AIWP_COVERAGE_ITEM_NAME"] = test.get("coverage_item_name", "")
    env["AIWP_TEST_NAME"] = test.get("test_name", "")

    completed = subprocess.run(
        command,
        cwd=REPO_ROOT,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )
    if completed.returncode == 0:
        return

    raise RunnerError(
        "\n".join(
            [
                f"Command: {shlex.join(command)}",
                f"Exit code: {completed.returncode}",
                "STDOUT:",
                completed.stdout.strip() or "<empty>",
                "STDERR:",
                completed.stderr.strip() or "<empty>",
            ]
        )
    )


def cleanup_pycache(relative_dir: Path) -> None:
    shutil.rmtree(REPO_ROOT / relative_dir / "__pycache__", ignore_errors=True)


def is_shell_glob(script: str) -> bool:
    return any(char in script for char in "*?[")


def format_failure(position: int, total: int, test: dict, technical_error: str) -> str:
    details = [
        f'[{position}/{total}] FAILURE: test "{test["test_name"]}" failed.',
        f'Coverage item: {test.get("coverage_item_id") or "<unknown>"}',
        f'Summary: {test.get("test_summary") or "<none>"}',
        f'Script: {test.get("test_script") or "<none>"}',
        "Technical error:",
        technical_error,
    ]
    return "\n".join(details)


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except RunnerError as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        raise SystemExit(1)
