#!/usr/bin/env bash
# validate-tasks.sh [task-glob] -- sanity-check the hidden tests/gates, no LLM calls. For every task in tasks/ and
# tasks-private/: the fresh workdir (incl. setup.patch, i.e. the planted state) must FAIL verify.sh, and
# <task>/reference.patch (applied with `git apply` on top) must PASS verify.sh. Multi-turn tasks have one cumulative
# reference.patch for the final state; there every verify-turn-NN.sh must also PASS. Exit 1 on any violation.
set -uo pipefail
BENCH="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck disable=SC1091
source "$BENCH/scripts/lib.sh"
pattern="${1:-*}"
bad=0
for d in $(all_task_dirs); do
  task="$(basename "$d")"
  # shellcheck disable=SC2053
  [[ "$task" == $pattern ]] || continue
  wd="$("$BENCH/scripts/reset.sh" "$task" V0)" || {
    echo "$task: reset failed"
    bad=1
    continue
  }
  if "$BENCH/scripts/verify.sh" "$task" "$wd" >"/tmp/validate-base-$task.log" 2>&1; then
    echo "$task: BASELINE PASSED (expected FAIL)"
    bad=1
  else echo "$task: baseline FAIL (ok)  [$(grep '^gates:' "/tmp/validate-base-$task.log")]"; fi
  rm -rf "$wd"
  wd="$("$BENCH/scripts/reset.sh" "$task" V0)" || continue
  if ! git -C "$wd" apply --whitespace=nowarn "$d/reference.patch"; then
    echo "$task: reference.patch does not apply"
    bad=1
  else
    # per-turn checks first: they copy hidden tests in and out, the final verify leaves them in place
    for v in "$d"/verify-turn-*.sh; do
      [[ -x "$v" ]] || continue
      if TASK="$task" BENCH="$BENCH" "$v" "$wd" >"/tmp/validate-$task-$(basename "$v").log" 2>&1; then echo "$task: $(basename "$v") PASS (ok)"; else
        echo "$task: $(basename "$v") FAILED on the reference (see /tmp/validate-$task-$(basename "$v").log)"
        bad=1
      fi
    done
    if "$BENCH/scripts/verify.sh" "$task" "$wd" >"/tmp/validate-$task.log" 2>&1; then echo "$task: reference PASS (ok)"; else
      echo "$task: REFERENCE FAILED (see /tmp/validate-$task.log)"
      bad=1
    fi
  fi
  rm -rf "$wd"
done
exit $bad
