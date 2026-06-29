#!/usr/bin/env bash
# SessionStart hook — injects a hard grounding directive + the live reality map
# into model context at the start of every session, so the model never works
# "blind" against the aspirational CLAUDE.md.
#
# Output on stdout is added to the model's context by Claude Code.
set -euo pipefail

ROOT="${CLAUDE_PROJECT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
STATUS="$ROOT/docs/STATUS.md"

cat <<'EOF'
=== ANTARIKSH SESSION GROUNDING (injected by SessionStart hook) ===

You have NOT yet seen the actual code. Before claiming anything is implemented:

1. `docs/STATUS.md` is AUTHORITATIVE for what is *built*. `CLAUDE.md` is the
   *intended* design and roadmap — most of it is NOT built yet.
2. The repo is a Phase-0 scaffold. Most modules are stubs (1-line Rust files,
   `501 not_implemented` HTTP handlers, services that just boot and park).
3. Do NOT assume any route, workflow, NATS subject, or module exists because
   CLAUDE.md mentions it. Verify against STATUS.md or by reading the file first.
4. `infra/`, `storage/`, and `build/` are empty placeholder directories.

Current reality map follows:
EOF

if [ -f "$STATUS" ]; then
  echo
  cat "$STATUS"
else
  echo
  echo "(WARNING: docs/STATUS.md not found at $STATUS — treat ALL of CLAUDE.md as unverified.)"
fi
