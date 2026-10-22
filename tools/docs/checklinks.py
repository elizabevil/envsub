#!/usr/bin/env python3
"""Report broken relative Markdown links in the docs tree.

A relative link is one that does not start with a scheme (http, https, mailto)
or a fragment. It is resolved against the directory of the file that holds it.
"""
import os
import re
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
LINK = re.compile(r"\[[^\]]*\]\(([^)\s]+)(?:\s+\"[^\"]*\")?\)")
INLINE_CODE = re.compile(r"`[^`\n]*`")
SKIP_DIRS = {".git", "dist", ".tmp", "node_modules"}


def strip_code(text: str) -> str:
    """Blank out code so its contents are not read as links.

    A diff line such as ``errors.As(err, &de)`` looks like a Markdown link to
    a file called ``err``; it is not one. The same holds for a code span
    inside a table cell.
    """
    out = []
    fence = None
    for line in text.splitlines(keepends=True):
        stripped = line.rstrip("\n")
        if stripped.startswith("```"):
            fence = None if fence is not None else stripped
            out.append("\n")
            continue
        out.append("\n" if fence is not None else line)
    text = "".join(out)
    return INLINE_CODE.sub(lambda m: " " * len(m.group(0)), text)


def is_external(target: str) -> bool:
    return target.startswith(("http://", "https://", "mailto:", "#", "/"))


def main() -> int:
    broken = []
    checked = 0
    for dirpath, dirnames, filenames in os.walk(ROOT):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        for name in filenames:
            if not name.endswith(".md"):
                continue
            path = os.path.join(dirpath, name)
            with open(path, encoding="utf-8") as handle:
                raw = handle.read()
            text = strip_code(raw)
            for match in LINK.finditer(text):
                target = match.group(1)
                if is_external(target):
                    continue
                target = target.split("#", 1)[0]
                if not target:
                    continue
                checked += 1
                resolved = os.path.normpath(os.path.join(dirpath, target))
                if not os.path.exists(resolved):
                    line = text[: match.start()].count("\n") + 1
                    rel = os.path.relpath(path, ROOT)
                    broken.append(f"{rel}:{line}: {target}")
    for item in broken:
        print(item)
    print(f"\nchecked {checked} relative links, {len(broken)} broken")
    return 1 if broken else 0


if __name__ == "__main__":
    sys.exit(main())
