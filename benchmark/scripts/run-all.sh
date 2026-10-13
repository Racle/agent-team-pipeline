#!/usr/bin/env bash
# run-all.sh [--reps N] [--rep-start N] [--tasks glob] [--modes "A B C V0 V1:sonnet V2:sonnet"] [--stop-on-broken]
# Runs matching tasks (tasks/ and tasks-private/) in the given modes (<mode>:<alias> = model override, labelled <mode>-<alias>),
# reps rep-start..reps, then prints the report. Mode order rotates per (task, rep) to spread time-of-day / rate-limit effects.
# Guards: BENCH_BUDGET (USD, default 90) = max summed est_usd of this batch, checked before every run (abort when reached);
# --stop-on-broken skips later reps of a task when every mode failed it in rep 1 with the same failed gates (the task or
# its gate is probably broken, not the agent); BENCH_RUN_MAX_USD (default 8): a single run above it prints a COST-ABORT
# note and the remaining reps of that task x mode are skipped (a running opencode cannot be priced mid-run).
set -euo pipefail
BENCH="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck disable=SC1091
source "$BENCH/scripts/lib.sh"
reps=3
stop_on_broken=0
rep_start=1
modes_arg='A B C'
pattern='*'
while [[ $# -gt 0 ]]; do
  case "$1" in
  --reps)
    reps="${2:?}"
    shift 2
    ;;
  --rep-start)
    rep_start="${2:?}"
    shift 2
    ;;
  --modes)
    modes_arg="${2:?}"
    shift 2
    ;;
  --tasks)
    pattern="${2:?}"
    shift 2
    ;;
  --stop-on-broken)
    stop_on_broken=1
    shift
    ;;
  -h | --help)
    sed -n '2,10p' "$0"
    exit 0
    ;;
  *)
    echo "unknown arg: $1" >&2
    exit 2
    ;;
  esac
done

tasks=()
for d in $(all_task_dirs); do
  name="$(basename "$d")"
  # shellcheck disable=SC2053
  [[ "$name" == $pattern ]] && tasks+=("$name")
done
[[ ${#tasks[@]} -gt 0 ]] || {
  echo "no tasks match '$pattern'" >&2
  exit 2
}

BENCH_BATCH="batch-$(date +%Y%m%d-%H%M%S)"
export BENCH_BATCH
BUDGET="${BENCH_BUDGET:-90}"
RUN_MAX="${BENCH_RUN_MAX_USD:-8}"
echo "batch: $BENCH_BATCH (budget \$$BUDGET, per-run note above \$$RUN_MAX)"
read -r -a modes <<<"$modes_arg"
for m in "${modes[@]}"; do [[ "$m" =~ ^([ABC]|V[0-9]|(C|V[0-9]):[A-Za-z0-9._/-]+)$ ]] || {
  echo "bad mode: $m" >&2
  exit 2
}; done

# spend <run_id|""> -- summed est_usd of the rows of this batch (or of one run)
spend() {
  [[ -f "$BENCH/results/results.csv" ]] || { echo 0; return; }
  awk -F, -v batch="$BENCH_BATCH" -v run="${1:-}" 'NR==1 { for (i = 1; i <= NF; i++) h[$i] = i; next }
    (run == "" ? $h["batch_id"] == batch : $h["run_id"] == run) { s += $h["est_usd"] } END { printf "%.4f\n", s }' "$BENCH/results/results.csv"
}
over() { awk -v a="$1" -v b="$2" 'BEGIN { exit !(a + 0 >= b + 0) }'; }
# failed_gates <run_id> -- space-joined names of failed gates ("" when none or when no gates were recorded)
failed_gates() {
  local f="$BENCH/results/runs/$1/gates.env"
  if [[ -f "$f" ]]; then grep '=0$' "$f" | cut -d= -f1 | sort | tr '\n' ' ' || true; fi
}

n=${#modes[@]}
i=0
declare -A skip
aborted=0
for task in "${tasks[@]}"; do
  for ((rep = rep_start; rep <= reps; rep++)); do
    order=()
    for ((k = 0; k < n; k++)); do order+=("${modes[(i + rep + k) % n]}"); done
    declare -A rep_fail=()
    for mode in "${order[@]}"; do
      [[ -n "${skip[$task:$mode]:-}" ]] && continue
      if over "$(spend "")" "$BUDGET"; then
        echo "BUDGET: batch spend $(spend "") >= \$$BUDGET; aborting before $task $mode rep $rep" >&2
        aborted=1
        break 3
      fi
      rm -f "$BENCH/results/.last-run"
      "$BENCH/scripts/run-task.sh" "$task" "$mode" "$rep" || echo "run failed: $task $mode $rep" >&2
      rid="$(cat "$BENCH/results/.last-run" 2>/dev/null || true)"
      [[ -n "$rid" ]] || continue
      rcost="$(spend "$rid")"
      echo "[$rid] est cost \$$rcost; batch total \$$(spend "")"
      if over "$rcost" "$RUN_MAX"; then
        echo "COST-ABORT note: $task $mode used \$$rcost (> \$$RUN_MAX); skipping its remaining reps" >&2
        skip[$task:$mode]=1
      fi
      rep_fail[$mode]="$(failed_gates "$rid")"
    done
    if [[ $stop_on_broken -eq 1 && $rep -eq $rep_start && $rep -lt $reps ]]; then
      common="" ok=1
      for mode in "${order[@]}"; do
        fg="${rep_fail[$mode]:-}"
        if [[ -z "$fg" ]] || { [[ -n "$common" ]] && [[ "$fg" != "$common" ]]; }; then ok=0; break; fi
        common="$fg"
      done
      if [[ $ok -eq 1 ]]; then
        echo "STOP-ON-BROKEN: every mode failed $task with the same gate(s) [$common]; skipping later reps" >&2
        break
      fi
    fi
  done
  i=$((i + 1))
done
[[ $aborted -eq 1 ]] && echo "NOTE: batch aborted by BENCH_BUDGET; report below covers the runs done so far" >&2

"$BENCH/bin/bench" report --in "$BENCH/results/results.csv" --batch "$BENCH_BATCH" --modes "$(IFS=,; echo "${modes[*]//:/-}")" | tee "$BENCH/results/report.md"
