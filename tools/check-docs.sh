#!/usr/bin/env bash
# Guards the document contract from CLAUDE.md § "Repo documents": status tokens
# in ROADMAP.md, ISO dates in the tracked documents, and — when the language-en
# module is selected — the decimal point in figures.
#
# Run from the repo root. The script takes no arguments.
#
# The language rule from CLAUDE.md § Language is not checked here. A repo-wide
# sweep for German markers was part of this script until 2026-10-13 and was
# removed; the reasoning is in claude-project-standards/DECISIONS.md under that
# date.
#
# Which checks apply is read from .claude/project-standards.json, so this script
# is copied between projects unchanged.
#
# Exit codes are three-valued on purpose, because "found nothing" and "could not
# look" are different answers and a caller must be able to tell them apart:
#
#   0   the checks ran and found nothing
#   1   the checks ran and found something
#   2   the checks could not run — bad usage, or the stamp cannot be read
#
# This detects. It does not prevent — for prevention, call it from
# .githooks/pre-commit.
set -euo pipefail
fail=0

# --language and --all selected the removed sweep. They are rejected rather than
# accepted as no-ops, so a caller that still passes one reports itself instead
# of appearing to run a check that no longer exists.
if [ "$#" -ne 0 ]; then
  echo "usage: check-docs.sh   (no arguments; --language and --all were removed on 2026-10-13)" >&2
  exit 2
fi

STAMP=".claude/project-standards.json"

# The module selection is read once, and three outcomes are kept apart:
#
#   no stamp          this project was never aligned. Checks that need a module
#                     are skipped; project-audit is what reports the absence.
#   readable stamp    its "modules" list decides which checks apply.
#   unreadable stamp  an error. Not an empty selection.
#
# The last distinction is why this is not inline in has_module. Reading an
# unreadable stamp as "no modules selected" makes every module-gated check skip
# itself while the script still exits 0 — a green check that verified nothing,
# and nothing else in the pipeline would report it.
MODULES=""
if [ -f "$STAMP" ]; then
  if ! command -v python3 >/dev/null 2>&1; then
    echo "check-docs: $STAMP exists but python3 is missing — cannot determine which checks apply" >&2
    exit 2
  fi
  if ! MODULES=$(python3 - "$STAMP" <<'PY'
import json, sys
with open(sys.argv[1]) as fh:
    data = json.load(fh)
mods = data.get("modules", [])
if not isinstance(mods, list):
    raise TypeError('"modules" is not a list')
print("\n".join(str(m) for m in mods))
PY
  ); then
    echo "check-docs: $STAMP cannot be parsed — repair the stamp or remove it" >&2
    exit 2
  fi
else
  echo "check-docs: no $STAMP — running only the checks that need no module selection" >&2
fi

# has_module <id> — true when the module is selected for this project.
has_module() {
  printf '%s\n' "$MODULES" | grep -qxF -- "$1"
}

# The documents that exist depend on the selection. A missing file is not a
# failure here — this checks contracts, not completeness; project-audit checks
# completeness.
DOCS=(ROADMAP.md DECISIONS.md)
has_module incidents && DOCS+=(INCIDENTS.md)
has_module policy && DOCS+=(STANDARDS.md)

check_documents() {
  if [ -f ROADMAP.md ]; then
    if grep -q '~~' ROADMAP.md; then
      echo "ROADMAP.md: struck-through rows found — move them out instead"
      fail=1
    fi

    if grep -qE '\[(done|dropped)' ROADMAP.md; then
      echo "ROADMAP.md: closed items do not belong here"
      fail=1
    fi

    # The whole token is matched, closing bracket included: a prefix match on
    # "[open" would accept "[opened]" or "[open-ish]".
    if grep -oE '^\| *\[[^]|]*\]?' ROADMAP.md | grep -vqE '^\| *\[open\]$'; then
      echo "ROADMAP.md: unknown status token"
      fail=1
    fi
  fi

  for f in "${DOCS[@]}"; do
    [ -f "$f" ] || continue

    # Non-ISO dates. The surrounding character classes keep the match out of
    # dotted number runs — version strings (v12.13.2) and IPv4 addresses
    # (46.225.64.29) are not dates and must not be reformatted.
    if grep -qE '(^|[^0-9.])[0-9]{2}\.[0-9]{2}\.(20[0-9]{2})?([^0-9]|$)' "$f"; then
      echo "$f: non-ISO date found — use YYYY-MM-DD"
      fail=1
    fi

    if has_module language-en; then
      # Decimal comma in a measurement. A comma followed by exactly three digits
      # is a thousands separator (1,234 GB), which the language rule allows; a
      # comma followed by one, two, or four or more digits is a decimal comma
      # (1,5 GB, 1,82 GB). The leading class keeps the match from starting in
      # the middle of a grouped number (12,345,6 GB).
      if grep -qE '(^|[^0-9,])[0-9]+(,[0-9]{3})*,([0-9]{1,2}|[0-9]{4,}) ?(B|KB|MB|GB|TB|KiB|MiB|GiB|TiB|k|M|G|T)\b' "$f"; then
        echo "$f: decimal comma in a figure — use a decimal point"
        fail=1
      fi
    fi
  done
}

check_documents

exit $fail
