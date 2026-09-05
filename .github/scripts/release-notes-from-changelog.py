#!/usr/bin/env python3
"""Extract the CHANGELOG.md section for VERSION into NOTES_FILE.

If ## VERSION is missing, create it: move the ## Unreleased body under the
new heading (or write a one-line fallback), leave an empty Unreleased
section, and continue. Releases must not fail because the heading was omitted.
"""

from __future__ import annotations

import os
import pathlib
import re
import sys


def heading_pattern(version: str) -> re.Pattern[str]:
    return re.compile(rf"^## \[?{re.escape(version)}\]?(?:\s|$|-)")


def find_section(lines: list[str], version: str) -> int | None:
    pattern = heading_pattern(version)
    return next((i for i, line in enumerate(lines) if pattern.match(line)), None)


def next_heading(lines: list[str], start: int) -> int:
    return next(
        (j for j in range(start + 1, len(lines)) if lines[j].startswith("## ")),
        len(lines),
    )


def unreleased_index(lines: list[str]) -> int | None:
    return next(
        (
            i
            for i, line in enumerate(lines)
            if re.match(r"^## \[?Unreleased\]?", line, re.I)
        ),
        None,
    )


def trim_blank(body: list[str]) -> list[str]:
    while body and not body[0].strip():
        body = body[1:]
    while body and not body[-1].strip():
        body = body[:-1]
    return body


def is_empty_body(body: list[str]) -> bool:
    return not any(line.strip() for line in body)


def ensure_section(lines: list[str], version: str) -> tuple[list[str], bool]:
    if find_section(lines, version) is not None:
        return lines, False

    heading = f"## {version}"
    unrel = unreleased_index(lines)
    if unrel is None:
        insert_at = 0
        for i, line in enumerate(lines):
            if line.startswith("## "):
                insert_at = i
                break
            if line.startswith("# "):
                insert_at = i + 1
        block = [heading, "", f"Release {version}.", ""]
        return lines[:insert_at] + block + lines[insert_at:], True

    unrel_end = next_heading(lines, unrel)
    body = trim_blank(lines[unrel + 1 : unrel_end])
    if is_empty_body(body):
        body = [f"Release {version}."]

    prefix = list(lines[: unrel + 1])
    new_section = ["", heading, ""] + body + [""]
    return prefix + new_section + lines[unrel_end:], True


def main() -> None:
    version = os.environ["VERSION"]
    notes_file = pathlib.Path(os.environ["NOTES_FILE"])
    changelog = pathlib.Path("CHANGELOG.md")

    if not changelog.is_file():
        sys.exit("CHANGELOG.md not found")

    lines = changelog.read_text(encoding="utf-8").splitlines()
    lines, updated = ensure_section(lines, version)
    if updated:
        changelog.write_text("\n".join(lines) + "\n", encoding="utf-8")
        print(f"Added ## {version} to CHANGELOG.md", file=sys.stderr)

    start = find_section(lines, version)
    if start is None:
        sys.exit(f"CHANGELOG.md has no ## {version} section")

    end = next_heading(lines, start)
    body = "\n".join(lines[start:end]).strip() + "\n"
    notes_file.write_text(body, encoding="utf-8")

    github_output = os.environ.get("GITHUB_OUTPUT")
    if github_output:
        with open(github_output, "a", encoding="utf-8") as handle:
            handle.write(f"file={notes_file}\n")
            handle.write(f"updated={'true' if updated else 'false'}\n")

    print(body)


if __name__ == "__main__":
    main()
