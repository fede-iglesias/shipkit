#!/usr/bin/env bash
# coverage-gate.sh - Asserts 100% statement coverage for a Go module.
#
# Usage:
#   bash .github/scripts/coverage-gate.sh <coverage-profile> <module-path>
#
# Arguments:
#   coverage-profile  Path to the coverage.out file produced by 'go test -coverprofile'.
#                     Pass /dev/null or an empty file to accept a vacuous pass.
#   module-path       Directory path of the module being tested (used in messages only).
#
# Environment:
#   EXEMPT=1          Skip coverage check entirely. Used for example/ modules that
#                     contain integration skeletons rather than library code.
#
# File-level exemptions:
#   The gate filters out lines belonging to files that match the
#   COVERAGE_SKIP_PATTERNS list (default: cosign_sigstore_real.go). These files
#   make TUF + Rekor network calls and cannot be unit-tested without live
#   Sigstore infrastructure; their absence from the profile is intentional and
#   documented in lifecycle/update/CHANGELOG.md (v0.2.4). To exempt additional
#   files, extend COVERAGE_SKIP_PATTERNS (one regex per line).
#
# Exit codes:
#   0 - coverage gate passed (or vacuous/exempt)
#   1 - coverage below 100.0% (with gap message on stderr)
#
# Examples:
#   bash .github/scripts/coverage-gate.sh coverage.out frontmatter
#   bash .github/scripts/coverage-gate.sh /dev/null example/shipkit-example
#   EXEMPT=1 bash .github/scripts/coverage-gate.sh coverage.out example/shipkit-example

set -euo pipefail

PROFILE="${1:?usage: coverage-gate.sh <coverage-profile> <module-path>}"
MODULE="${2:?usage: coverage-gate.sh <coverage-profile> <module-path>}"

# Patterns of files exempt from the per-file coverage check. Each line is an
# extended regex matched against the file path that appears in coverage.out
# entries ("module/path/file.go:line.col,line.col stmts hits"). The pattern is
# anchored to "/" boundaries by the awk filter below.
COVERAGE_SKIP_PATTERNS="${COVERAGE_SKIP_PATTERNS:-cosign_sigstore_real\.go}"

# Exempt modules (e.g. example/) skip the gate entirely.
if [ "${EXEMPT:-0}" = "1" ]; then
  echo "coverage exempt for module: $MODULE"
  exit 0
fi

# Vacuous pass: profile is /dev/null or an empty or missing file.
# This handles the B0 bootstrap phase where no library packages exist yet.
if [ "$PROFILE" = "/dev/null" ] || [ ! -s "$PROFILE" ]; then
  echo "vacuous coverage pass for module: $MODULE (no statements to measure)"
  exit 0
fi

# Build a filtered profile that omits lines from files matching
# COVERAGE_SKIP_PATTERNS. The "mode:" header is preserved so `go tool cover`
# still parses the result. Files containing TUF + Rekor calls cannot be
# unit-tested offline; see the script header for the documented exemption.
FILTERED_PROFILE="$(mktemp -t coverage-filtered.XXXXXX)"
trap 'rm -f "$FILTERED_PROFILE"' EXIT

awk -v patterns="$COVERAGE_SKIP_PATTERNS" '
  BEGIN {
    n = split(patterns, pat, "\n")
  }
  NR == 1 { print; next }
  {
    file = $1
    # Strip the trailing ":line.col,line.col" portion before matching so the
    # pattern can match the basename of the source file.
    sub(/:[0-9]+\.[0-9]+,[0-9]+\.[0-9]+$/, "", file)
    for (i = 1; i <= n; i++) {
      if (pat[i] != "" && file ~ pat[i]) {
        next
      }
    }
    print
  }
' "$PROFILE" > "$FILTERED_PROFILE"

PROFILE="$FILTERED_PROFILE"

# Compute total statement coverage via go tool cover.
PCT=$(go tool cover -func="$PROFILE" \
  | awk '/^total:/ { print $3 }' \
  | tr -d '%')

if [ -z "$PCT" ]; then
  echo "vacuous coverage pass for module: $MODULE (coverage profile has no statements)"
  exit 0
fi

echo "$MODULE coverage: $PCT%"

# Require exactly 100.0 - shipkit library modules must be fully covered.
# Network-call functions use the ErrNotConfigured sentinel pattern so they
# can be tested without real network access (see sigstoreRealVerify pattern).
if [ "$PCT" != "100.0" ]; then
  echo "ERROR: coverage gate FAILED for $MODULE" >&2
  echo "  got:      $PCT%" >&2
  echo "  required: 100.0%" >&2
  echo "" >&2
  echo "Uncovered functions:" >&2
  go tool cover -func="$PROFILE" | awk '$3 != "100.0%" && !/^total:/ { print "  " $0 }' >&2
  echo "" >&2
  echo "Move any untestable network-call functions to the consumer cmd/ layer" >&2
  echo "using the ErrNotConfigured sentinel pattern (sigstoreRealVerify style)." >&2
  exit 1
fi

echo "coverage gate PASSED for $MODULE: 100.0%"
