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
