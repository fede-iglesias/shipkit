#!/usr/bin/env bash
# Pre-commit go vet runner.
# Receives staged .go files as arguments from lefthook (glob: "*.go").
# Detects their nearest go.mod ancestor and runs `go vet ./...` in each
# unique module. Any vet output is treated as FAIL, matching the behavior
# of the release workflow CI in .github/workflows/.
#
# Historical: v0.2.1 shipped with `deps.FS = deps.FS` in
# lifecycle/install/plan_test.go:245; local `go vet` was lax, CI release
# workflow rejected the tag, forcing retract + v0.2.2 bump.

set -uo pipefail

if [ "$#" -eq 0 ]; then
  exit 0
fi

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root" || exit 2

modules=""
for f in "$@"; do
  dir="$(dirname "$f")"
  while :; do
    if [ -f "$dir/go.mod" ]; then
      case " $modules " in
        *" $dir "*) ;;
        *) modules="$modules $dir" ;;
      esac
      break
    fi
    if [ "$dir" = "." ] || [ "$dir" = "/" ]; then
      break
    fi
    dir="$(dirname "$dir")"
  done
done

trimmed="$(echo "$modules" | tr -s ' ')"
if [ -z "${trimmed// /}" ]; then
  exit 0
fi

failed=0
for m in $modules; do
  printf '==> go vet ./... in %s\n' "$m"
  if ! ( cd "$m" && go vet ./... ); then
    failed=1
  fi
done

if [ "$failed" -ne 0 ]; then
  printf '\nFAIL: go vet found issues. Release workflows enforce vet strictly; fix locally before commit.\n' >&2
  exit 1
fi

exit 0
