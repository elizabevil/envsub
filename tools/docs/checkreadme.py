#!/usr/bin/env python3
"""Run the commands documented in a README and compare their output.

Every ```console block is a transcript: a line starting with "$ " is a command,
the lines after it are the output it should produce. The ```bash blocks run
first, in one scratch directory, so the files they create are there when the
transcripts run. Blocks that only make sense on a developer machine are
skipped.
"""
import os
import re
import subprocess
import sys
import tempfile

CONSOLE = re.compile(r"^```console\n(.*?)^```", re.MULTILINE | re.DOTALL)
BASH = re.compile(r"^```bash\n(.*?)^```", re.MULTILINE | re.DOTALL)
PROMPT = re.compile(r"^\$ (.*)$")

SKIP = ("go install", "git clone", "cd examples", "just ", "go run")


def parse_console(text):
    for block in CONSOLE.findall(text):
        lines = block.splitlines()
        current = None
        expected = []
        for line in lines:
            match = PROMPT.match(line)
            if match:
                if current is not None:
                    yield current, expected
                current = match.group(1)
                expected = []
            elif current is not None:
                if line.startswith("$ "):
                    continue
                expected.append(line)
        if current is not None:
            yield current, expected


def main() -> int:
    binary = os.path.abspath("dist/envsub")
    if not os.path.exists(binary):
        print(f"{binary} not found; run `just build` first")
        return 2
    failures = 0
    total = 0
    last_code = 0
    with tempfile.TemporaryDirectory() as work:
        for path in sys.argv[1:]:
            with open(path, encoding="utf-8") as handle:
                text = handle.read()
            # The bash blocks create the files the transcripts read.
            for block in BASH.findall(text):
                if any(marker in block for marker in SKIP):
                    continue
                subprocess.run(block, shell=True, cwd=work, capture_output=True, text=True)
            for command, expected in parse_console(text):
                if "envsub " not in command:
                    continue
                command = command.replace("envsub ", f"{binary} ", 1)
                total += 1
                proc = subprocess.run(
                    command, shell=True, cwd=work, capture_output=True, text=True
                )
                got = (proc.stdout + proc.stderr).rstrip("\n")
                want = "\n".join(expected).rstrip("\n")
                if command.strip() == "echo $?":
                    # The transcript documents an exit code; check it against
                    # the code the previous command actually returned.
                    want = str(last_code)
                last_code = proc.returncode
                if want and got != want:
                    failures += 1
                    print(f"{path}: command: {command}")
                    print(f"  want:\n{want}")
                    print(f"  got:\n{got}")
    print(f"\nran {total} documented commands, {failures} mismatched")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
