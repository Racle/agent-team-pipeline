#!/usr/bin/env bash
# run-task.sh <task> <A|B|C|V0|V1|V2>[:model] [rep] -- run one task in one mode, verify, and append to results/results.csv.
# Mode <C|V0|V1|V2>:<alias> = that mode with a model override; recorded as mode <mode>-<alias> (e.g. V1-sonnet).
# <alias> is opus|sonnet|haiku (-> github-copilot/claude-<alias>-5.5) or a full provider/model id; default opus.
# Mode A: old pipeline (team-captain, legacy-agents/). B: bench-solo (legacy-agents/). C: current agents/team-captain.md.
# V0: exact copy of agents/team-captain.md; V1: V0 + `task: allow` + a delegation rule; V2: V1 + cheaper explore model
# (variants/vN/). Tasks come from tasks/ or tasks-private/.
# Multi-turn tasks (<task>/turns/NN.md): turn 1 starts the session, later turns continue it with `opencode run --session`;
# one overall timeout; events go to events-NN.jsonl; <task>/verify-turn-NN.sh (arg: workdir; env TASK, BENCH) is run after
# each turn; TURNS / TURNS_PASSED are recorded; the verdict needs all final gates AND all turns passed.
# Env: BENCH_TIMEOUT (seconds, default 1800; 3600 for multi-turn), BENCH_KEEP_WORKDIR=1 to keep the workdir, BENCH_BATCH (batch id).
# Exit: 0 ok; 3 model has no price entry; 4 collect failed.
set -euo pipefail
BENCH="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck disable=SC1091
source "$BENCH/scripts/lib.sh"
task="${1:?usage: run-task.sh <task> <A|B|C|V0|V1|V2>[:model] [rep]}"
mode_arg="${2:?usage: run-task.sh <task> <A|B|C|V0|V1|V2>[:model] [rep]}"
rep="${3:-1}"
MODEL="github-copilot/claude-opus-5.5"
mode="${mode_arg%%:*}" # base mode: A, B, C, V0, V1 or V2
label="$mode"          # mode label recorded in the CSV and run id
if [[ "$mode_arg" == *:* ]]; then
  [[ "$mode" == C || "$mode" == V[0-9] ]] || {
    echo "model override is only supported for modes C, V0, V1, V2" >&2
    exit 2
  }
  malias="${mode_arg#*:}"
  [[ -n "$malias" ]] || {
    echo "empty model alias" >&2
    exit 2
  }
  case "$malias" in
  */*) MODEL="$malias" ;;
  *) MODEL="github-copilot/claude-$malias-5.5" ;;
  esac
  label="$mode-${malias//\//_}"
fi
SUFFIX="Do not ask questions or wait for approval; proceed. Commit locally when done; no git remote exists."

case "$mode" in A) agent=team-captain ;; B) agent=bench-solo ;; C | V[0-9]) agent=team-captain ;; *)
  echo "mode must be A, B, C, V0, V1 or V2" >&2
  exit 2
  ;;
esac
tdir="$(task_dir "$task")" || {
  echo "unknown task: $task" >&2
  exit 2
}
# shellcheck disable=SC1091
source "$tdir/meta.env"
: "${CATEGORY:?meta.env must define CATEGORY}"
# Isolation: do not inject opencode's Claude-Code-compatible system prompt (name found in the opencode 1.18.35 binary;
# its exact effect is unverified). Project .claude/skills stay visible on purpose.
export OPENCODE_DISABLE_CLAUDE_CODE_PROMPT=1

mkdir -p "$BENCH/bin" "$BENCH/results"
if [[ ! -x "$BENCH/bin/bench" || "$BENCH/cmd/bench/main.go" -nt "$BENCH/bin/bench" ]]; then
  (cd "$BENCH" && go build -o bin/bench ./cmd/bench)
fi

run_id="${task}-${label}-r${rep}-$(date +%Y%m%d-%H%M%S)"
out="$BENCH/results/runs/$run_id"
mkdir -p "$out"
wd="$("$BENCH/scripts/reset.sh" "$task" "$mode")"
echo "[$run_id] workdir: $wd"
# turns: tasks with turns/NN.md run as one session of several prompts; others use prompt.md
turn_files=()
if compgen -G "$tdir/turns/*.md" >/dev/null; then
  for f in "$tdir"/turns/*.md; do turn_files+=("$f"); done
else
  turn_files=("$tdir/prompt.md")
fi
nturns=${#turn_files[@]}
multi=0
[[ $nturns -gt 1 ]] && multi=1

# Isolation: an empty XDG_CONFIG_HOME hides the user's global opencode config (MCP servers such as
# Engram/Playwright, plugins, global agents/skills). Credentials live in XDG_DATA_HOME and are kept.
export XDG_CONFIG_HOME="$out/xdg-config"
mkdir -p "$XDG_CONFIG_HOME"

# shellcheck disable=SC2317 # invoked via trap
cleanup() {
  if [[ -n "${agent_pid:-}" ]]; then kill -TERM -- "-$agent_pid" 2>/dev/null || true; fi # whole process group
  if [[ "${BENCH_KEEP_WORKDIR:-0}" == 1 ]]; then echo "[$run_id] kept workdir $wd"; else rm -rf "$wd"; fi
}
trap cleanup EXIT
trap 'exit 130' INT TERM

start=$(date +%s)
timeout_total="${BENCH_TIMEOUT:-$([[ $multi -eq 1 ]] && echo 3600 || echo 1800)}"
deadline=$((start + timeout_total))
session=""
timed_out=0
turns_passed=0
rc=0
for ((t = 1; t <= nturns; t++)); do
  nn="$(printf '%02d' "$t")"
  ev="$out/events.jsonl"
  [[ $multi -eq 1 ]] && ev="$out/events-$nn.jsonl"
  prompt="$(cat "${turn_files[t - 1]}")

$SUFFIX"
  remaining=$((deadline - $(date +%s)))
  if [[ $remaining -le 0 ]]; then timed_out=1; break; fi
  sess_args=()
  [[ $t -gt 1 ]] && sess_args=(--session "$session")
  # setsid: own process group so cleanup can kill opencode's children; timeout -k escalates to SIGKILL.
  setsid timeout -k 30 "$remaining" opencode run \
    --dir "$wd" --agent "$agent" --model "$MODEL" --format json --title "bench-$run_id" "${sess_args[@]}" \
    "$prompt" >"$ev" 2>"$out/stderr-$nn.log" &
  agent_pid=$!
  rc=0
  wait "$agent_pid" || rc=$?
  kill -TERM -- "-$agent_pid" 2>/dev/null || true # reap stragglers
  agent_pid=
  if [[ $rc -eq 124 || $rc -eq 137 ]]; then
    timed_out=1
    echo "[$run_id] TIMEOUT in turn $t after $(($(date +%s) - start))s" >&2
  elif [[ $rc -ne 0 ]]; then
    echo "[$run_id] opencode exited $rc in turn $t (see $out/stderr-$nn.log)" >&2
  fi
  [[ $t -eq 1 ]] && session="$(grep -o '"sessionID":"ses_[A-Za-z0-9]*"' "$ev" | head -1 | cut -d'"' -f4 || true)"
  # per-turn check (hidden tests are copied in and removed again; see verify.sh HIDDEN_TURN)
  if [[ -x "$tdir/verify-turn-$nn.sh" ]]; then
    if TASK="$task" BENCH="$BENCH" "$tdir/verify-turn-$nn.sh" "$wd" >"$out/verify-turn-$nn.log" 2>&1; then
      turns_passed=$((turns_passed + 1))
      echo "[$run_id] turn $t: PASS"
    else echo "[$run_id] turn $t: FAIL"; fi
  fi
  [[ $timed_out -eq 1 ]] && break
  if [[ $t -eq 1 && $nturns -gt 1 && -z "$session" ]]; then
    echo "[$run_id] no session id after turn 1; cannot continue the session" >&2
    break
  fi
done
duration=$(($(date +%s) - start))
[[ $rc -ne 0 && $timed_out -eq 0 ]] && echo "[$run_id] last opencode exit code $rc" >&2

# Code-size metrics vs the baseline commit, BEFORE verify.sh copies the hidden tests in. Excludes node_modules,
# lockfiles, build output, .opencode and opencode.json. app = non-test source lines added; test = *_test.go/*.test.ts(x).
(
  cd "$wd"
  git add -A >/dev/null 2>&1 # stage so untracked files count; the workdir is disposable
  base="$(git rev-list --max-parents=0 HEAD | tail -1)"
  git diff --cached --numstat "$base" -- . ':(exclude)node_modules' ':(exclude)**/node_modules/**' \
    ':(exclude)*package-lock.json' ':(exclude)*yarn.lock' ':(exclude)*pnpm-lock.yaml' ':(exclude)*go.sum' \
    ':(exclude)dist' ':(exclude)**/dist/**' ':(exclude)build' ':(exclude)**/build/**' \
    ':(exclude).opencode' ':(exclude)opencode.json' |
    awk -F'\t' '{ a = ($1 == "-") ? 0 : $1; f++; t += a
      if ($3 ~ /(_test\.go|\.test\.tsx?)$/) test += a
      else if ($3 ~ /\.(go|tsx?|jsx?|css|html)$/) app += a }
      END { printf "FILES_CHANGED=%d\nAPP_ADDED=%d\nTEST_ADDED=%d\nTOTAL_ADDED=%d\n", f, app, test, t }' >"$out/metrics.env"
) 2>/dev/null || true

if [[ $multi -eq 1 ]]; then printf 'TURNS=%d\nTURNS_PASSED=%d\n' "$nturns" "$turns_passed" >>"$out/metrics.env"; fi
pass=false
if [[ $timed_out -eq 0 ]] && "$BENCH/scripts/verify.sh" "$task" "$wd" "$out/gates.env" >"$out/verify.log" 2>&1; then pass=true; fi
[[ $multi -eq 1 && $turns_passed -lt $nturns ]] && pass=false
[[ $timed_out -eq 1 ]] && echo "timeout: counted as FAIL" >"$out/verify.log"
echo "$run_id" >"$BENCH/results/.last-run"
echo "[$run_id] verdict: $([[ $pass == true ]] && echo PASS || echo FAIL)  (${duration}s)"
(
  cd "$wd" && git log --oneline -5 >"$out/git-log.txt" 2>&1
  git diff --stat HEAD~1 >"$out/diffstat.txt" 2>&1 || true
) || true

# Always record a row (sessionless runs get a placeholder, pass=false). Collect failure must not skip cleanup (trap).
[[ -n "$session" ]] || {
  echo "[$run_id] no session id found in events; recording placeholder row (see $out/events*.jsonl, $out/stderr-*.log)" >&2
  pass=false
}
set +e
"$BENCH/bin/bench" collect --session "$session" --task "$task" --category "$CATEGORY" --mode "$label" --rep "$rep" \
  --pass "$pass" --duration "$duration" --prices "$BENCH/prices.json" --out "$BENCH/results/results.csv" \
  --metrics-file "$out/metrics.env" --gates-file "$out/gates.env" --run-id "$run_id" --batch "${BENCH_BATCH:-$run_id}" --save-dir "$out/sessions" --strict 2>&1 | tee "$out/collect.log" >&2
crc=${PIPESTATUS[0]}
set -e
if grep -q "no price entry" "$out/collect.log"; then
  echo "[$run_id] ERROR: model key has no price entry in prices.json; costs are 0. Compare keys in $out/sessions/*.json with prices.json." >&2
  exit 3
fi
[[ $crc -eq 0 ]] || {
  echo "[$run_id] ERROR: collect failed (exit $crc)" >&2
  exit 4
}
exit 0
