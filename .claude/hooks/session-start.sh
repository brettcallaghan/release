#!/bin/bash
# SessionStart hook — provision what a cloud session needs to work here.
#
#   1. Install `bd` (Beads) and hydrate the tracker from the `beads/data`
#      branch via scripts/beads-setup.sh.
#   2. Build the `release` CLI from this working tree onto PATH.
#   3. Build the `specs` CLI from a sibling specs checkout, when one
#      is attached, so `specs check --strict` can gate the corpus.
#   4. Install the `strata` compiler when STRATA_ACCESS_TOKEN is set, so
#      scripts/generate-schemas.sh can regenerate and check internal/ types.
#
# Idempotent and non-interactive. Claude Code registers hooks from the
# PROJECT ROOT's .claude/settings.json only, so a session that attaches this
# repo alongside another never runs it. CLAUDE.md carries the manual fallback;
# docs/session-bootstrap.md describes the durable fix.
#
# To run it by hand:
#   CLAUDE_PROJECT_DIR=/path/to/release bash .claude/hooks/session-start.sh
set -euo pipefail

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
cd "$PROJECT_DIR"

# Only provision in the cloud. A local machine has its own toolchain and bd.
if [ "${CLAUDE_CODE_REMOTE:-}" != "true" ]; then
  exit 0
fi

log() { echo "[session-start] $*"; }

# bd has been observed writing its global config to a literal ./~/ inside the
# repo instead of $HOME. Pin HOME, and sweep the artifact if it appears anyway.
export HOME="${HOME:-/root}"
sweep_literal_tilde() {
  if [ -d "$PROJECT_DIR/~" ]; then
    rm -rf "$PROJECT_DIR/~"
    log "removed a literal ./~ directory (bd wrote its global config there)"
  fi
}
trap sweep_literal_tilde EXIT

# --- 1. bd (Beads) -----------------------------------------------------------
GOBIN_DIR="$(go env GOPATH 2>/dev/null)/bin"

# bd warns on a group/world-readable beads dir. Quiet it up front, before any
# bd call, so the warning does not appear on every invocation and train the
# agent to ignore bd's stderr.
[ -d .beads ] && chmod 700 .beads 2>/dev/null || true

if ! command -v bd >/dev/null 2>&1 && [ ! -x /usr/local/bin/bd ]; then
  # Beads embeds Dolt, whose regex support is cgo-bound to ICU. Without these
  # headers any source build fails at `unicode/uregex.h: No such file`.
  if [ ! -f /usr/include/unicode/uregex.h ]; then
    log "installing libicu-dev (ICU headers, required by the Dolt dependency)"
    apt-get install -y libicu-dev >/dev/null 2>&1 \
      || log "WARNING: libicu-dev install failed; a source build will likely fail"
  fi
  # Use the official installer rather than calling `go install` directly: it
  # prefers a prebuilt release binary and falls back to a source build only
  # when the release lookup is unavailable — which it is behind this egress
  # proxy, where api.github.com and the releases pages return 403 while
  # raw.githubusercontent.com and proxy.golang.org do not. Delegating the
  # choice means this gets faster for free if release access is ever restored,
  # with no edit here.
  log "installing bd (prebuilt if reachable, else a source build)"
  curl -sSL https://raw.githubusercontent.com/steveyegge/beads/main/scripts/install.sh 2>/dev/null \
    | bash >/dev/null 2>&1 \
    || log "WARNING: bd installer failed"
fi

# Symlink onto the default PATH. A PATH edit alone is not enough: subagents and
# hook subprocesses do not reliably inherit this session's environment, and a
# skill shelling out to `bd` from one of those would fail with "command not
# found" — which reads as a skill defect rather than a missing dependency.
if [ -x "$GOBIN_DIR/bd" ] && [ ! -e /usr/local/bin/bd ]; then
  ln -sf "$GOBIN_DIR/bd" /usr/local/bin/bd
fi
export PATH="$GOBIN_DIR:$PATH"
if [ -n "${CLAUDE_ENV_FILE:-}" ]; then
  echo "export PATH=\"$GOBIN_DIR:\$PATH\"" >> "$CLAUDE_ENV_FILE"
fi

if command -v bd >/dev/null 2>&1; then
  log "bd ready: $(bd --version 2>/dev/null | grep -o 'version [0-9.]*' | head -1)"
else
  log "WARNING: bd unavailable — grooming and orchestration skills will not run"
  # Fatal at the end of the hook: without bd the tracker block below is
  # skipped, and nothing else would stop the session reporting ready.
  BB_DOLT_FATAL="bd is not installed"
fi

# --- 2. Beads telemetry and database ----------------------------------------
if command -v bd >/dev/null 2>&1; then
  # bd stores data in Dolt, which phones home to eventsapi.dolthub.com. Killed
  # here rather than making every session re-do it.
  if command -v dolt >/dev/null 2>&1 \
     && ! dolt config --global --list 2>/dev/null | grep -q 'metrics.disabled = true'; then
    dolt config --global --add metrics.disabled true >/dev/null 2>&1 || true
  fi
  export BD_DISABLE_TELEMETRY=1
  [ -n "${CLAUDE_ENV_FILE:-}" ] && echo 'export BD_DISABLE_TELEMETRY=1' >> "$CLAUDE_ENV_FILE"

  # bd must never add a Dolt remote on its own: one added without the
  # `beads/data` ref points at refs/dolt/data. scripts/beads-setup.sh writes it.
  export BD_NO_REMOTE_ADOPT=1
  [ -n "${CLAUDE_ENV_FILE:-}" ] && echo 'export BD_NO_REMOTE_ADOPT=1' >> "$CLAUDE_ENV_FILE"

  # --- 2b. A database from an earlier tracker ---------------------------------
  #
  # Must run BEFORE anything reads the tracker. If `dolt_database` names a
  # database not on disk, bd CREATES an empty one under that name and reports
  # a healthy, empty tracker — measured on bd 1.2.2: `bd stats` printed
  # "Total Issues: 0", exit 0, no error. A container reused from before the
  # tracker moved holds exactly that: a database under another name.
  # Move the whole directory aside — never delete it, it may hold unpushed
  # beads — so the setup script below clones the named database fresh.
  if [ -d .beads/embeddeddolt ]; then
    _want="$(python3 -c 'import json,io;print(json.load(io.open(".beads/metadata.json",encoding="utf-8")).get("dolt_database",""))' 2>/dev/null)" || _want=""
    if [ -n "$_want" ] && [ ! -d ".beads/embeddeddolt/$_want" ]; then
      _stale=".beads/embeddeddolt.stale.$$"
      mv .beads/embeddeddolt "$_stale" \
        && log "moved aside a database from an earlier tracker (no '$_want' in it): $_stale"
    fi
  fi

  # --- 2c. The database -------------------------------------------------------
  # scripts/beads-setup.sh clones the tracker from the `beads/data` branch on a
  # fresh clone, points the Dolt remote at that branch, and pulls otherwise.
  # It verifies by issue count and exits non-zero on an empty tracker.
  BB_DOLT_FATAL=""
  if _setup_out="$(scripts/beads-setup.sh 2>&1)"; then
    printf '%s\n' "$_setup_out" | sed 's/^beads-setup: /[session-start] /'
  else
    BB_DOLT_FATAL="scripts/beads-setup.sh failed: $(printf '%s' "$_setup_out" | tr '\n' ' ' | cut -c1-400)"
  fi

  # A database whose configured prefix disagrees with the ids it holds mints a
  # SECOND id namespace silently, because the prefix lives in the database and
  # travels with the remote. Warn only: `bd rename-prefix --repair` rewrites
  # ids to random hashes when it misdetects the prefix, which is worse than the
  # stray namespace it would fix.
  if [ -z "$BB_DOLT_FATAL" ] && [ -d .beads/embeddeddolt ]; then
    # Read the prefix positively: under `pipefail` a transient `bd config list`
    # failure is indistinguishable from "the prefix is wrong".
    _config="$(bd config list 2>/dev/null)" || _config=""
    _prefix="$(sed -n 's/^[[:space:]]*issue_prefix = //p' <<<"$_config")"
    _prefix="${_prefix%%$'\n'*}"
    if [ -z "$_prefix" ]; then
      log "note: could not read the tracker prefix — skipping the namespace guard"
    elif [ "$_prefix" != "release" ]; then
      log "WARNING: tracker prefix is '$_prefix', not 'release' — new beads will mint a second namespace"
    fi
  fi
fi

# --- 3. release CLI -----------------------------------------------------------
# Built from this working tree, so the binary on PATH matches the code in
# front of the session. Non-fatal: `go run ./cmd/release` works without it.
if command -v go >/dev/null 2>&1; then
  if CGO_ENABLED=0 go build -trimpath -o /usr/local/bin/release ./cmd/release >/dev/null 2>&1; then
    log "release CLI ready: $(/usr/local/bin/release --version 2>/dev/null)"
  else
    log "WARNING: could not build the release CLI — run 'go build ./cmd/release' to see why"
  fi
fi

# --- 4. specs CLI --------------------------------------------------------------
# `specs check --strict` gates the corpus under specs/. The CLI lives in
# brettcallaghan/specs, which is private, so it is built from a sibling
# checkout when the session has one, and skipped otherwise. The checks that
# need it say SKIPPED out loud rather than passing silently.
for candidate in "${SPECS_SRC_DIR:-}" "$PROJECT_DIR/../specs"; do
  [ -n "$candidate" ] && [ -f "$candidate/cli/specs/main.go" ] || continue
  if (cd "$candidate/cli" && CGO_ENABLED=0 go build -trimpath -o /usr/local/bin/specs ./specs) >/dev/null 2>&1; then
    log "specs CLI ready (built from $candidate)"
  else
    log "WARNING: could not build the specs CLI from $candidate"
  fi
  break
done
command -v specs >/dev/null 2>&1 || log "note: no specs CLI — attach brettcallaghan/specs to check the corpus"

# --- 5. strata CLI --------------------------------------------------------------
# The Go types and JSON Schema under internal/ are generated from
# schemas/*.strata by scripts/generate-schemas.sh. strata-lang/strata is
# private, so the installer needs STRATA_ACCESS_TOKEN; without it the
# `schemas` check SKIPs out loud. It installs the release pinned in
# schemas/strata.version, and is a no-op when that is current.
if [ -n "${STRATA_ACCESS_TOKEN:-}" ]; then
  if scripts/install-strata-cli.sh "$(tr -d '[:space:]' <schemas/strata.version)" >/dev/null 2>&1; then
    log "strata CLI ready: $(strata --version 2>/dev/null || echo installed)"
  else
    log "WARNING: could not install strata — run scripts/install-strata-cli.sh to see why"
  fi
else
  log "note: no STRATA_ACCESS_TOKEN — strata not installed; generated schemas cannot be checked"
fi

# --- 6. Fail loudly if the tracker is unusable -----------------------------------
# Last, so everything else is provisioned first. A session that cannot write
# the tracker would lose every bead it closes.
if [ -n "${BB_DOLT_FATAL:-}" ]; then
  log ""
  log "ERROR: the beads tracker is not usable in this session — $BB_DOLT_FATAL"
  log ""
  log "  This repository's tracker is the Dolt database on the beads/data"
  log "  branch of origin. There is no JSONL fallback by design. Do not"
  log "  create or close beads in this session: the work would be lost when"
  log "  the container is reclaimed."
  log ""
  log "  Check that origin carries the branch:"
  log "    git ls-remote origin 'refs/heads/beads/*'"
  log "  then re-run ./scripts/beads-setup.sh."
  log ""
  exit 1
fi

log "ready."
