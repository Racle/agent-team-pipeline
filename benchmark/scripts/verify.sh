#!/usr/bin/env bash
# verify.sh <task> <workdir> [gates-file] -- strict deterministic completion gate. Exit 0 = PASS, 1 = FAIL.
# Gates come from meta.env GATES (space separated, lowercase). Unset = the v1 default for the synthetic fixture:
#   tests vet tsc build hidden
# Every listed gate always runs (none short-circuits); `hidden` always runs last because it copies the hidden tests in.
#   tests    project's own tests: go test -race ./... (api) + vitest run (web)            [synthetic fixture only]
#   vet      go vet ./... (api)                                                            [synthetic fixture only]
#   build    go build ./... (api) + npx vite build (web)                                   [synthetic fixture only]
#   tsc      npx tsc --noEmit; real repos: no NEW normalised errors vs tasks-private/baseline/<repo>.tsc.norm
#   gobuild  go build ./...                                    (real Go repos)
#   govet    go vet ./pkg/...; no NEW findings vs baseline/<repo>.vet.norm        (real Go repos)
#   gofmt    gofmt -l .; no NEW unformatted files vs baseline/<repo>.gofmt.norm   (real Go repos)
#   nodetest the repo's existing node tests (meta.env NODETEST_CMD, default `npm run test:voucher-export`); the
#            failure count may not exceed the baseline's (`# fail N` in baseline/<repo>.txt)
#   hidden   the task's hidden tests:
#              synthetic fixture: hidden/api/* -> api/, hidden/web/* -> web/src/hidden/ (go -run TestHidden / vitest)
#              real repos: hidden/ (and hidden-turns/01..NN/) mirror the workdir layout; *_test.go are run per package with
#              `go test -run TestHidden`, *.test.ts with `node --require <NODE_REGISTER> --test` (meta.env, default
#              test/hidden/register.cjs relative to the app dir, shipped inside hidden/).
# Env: GATES (overrides meta.env), HIDDEN_TURN=N (hidden gate only runs/copies hidden-turns/01..N, then removes them again
# so the agent never sees them between turns), GO_TEST_FLAGS (meta.env; -race is always on for the synthetic fixture).
# [gates-file] receives one KEY=VALUE per line, 1 = pass, 0 = fail (e.g. HIDDEN=1 GOVET=0 ...).
# shellcheck disable=SC2317 # gate commands are invoked indirectly
set -euo pipefail
BENCH="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck disable=SC1091
source "$BENCH/scripts/lib.sh"
task="${1:?usage: verify.sh <task> <workdir> [gates-file]}"
wd="${2:?usage: verify.sh <task> <workdir> [gates-file]}"
gates_file="${3:-}"
tdir="$(task_dir "$task")" || {
  echo "bad task" >&2
  exit 2
}
[[ -d "$wd" ]] || {
  echo "bad workdir" >&2
  exit 2
}
hidden="$tdir/hidden"
GATES_ENV="${GATES:-}" # an explicit GATES env var (e.g. from verify-turn-NN.sh) overrides meta.env
FIXTURE="" GATES="" GO_TEST_FLAGS="" NODETEST_CMD="npm run test:voucher-export" NODE_REGISTER="test/hidden/register.cjs"
# shellcheck disable=SC1091
source "$tdir/meta.env"
[[ -n "$GATES_ENV" ]] && GATES="$GATES_ENV"
export NODE_OPTIONS="--max-old-space-size=12288" # big rio apps OOM tsc at the default heap
GO_FLAGS="-race ${GO_TEST_FLAGS:-}"

# repo layout: "short:dir" pairs (dir relative to the workdir; "." for a single repo)
repos=()
if [[ "$FIXTURE" == repo:* ]]; then
  IFS=, read -r -a shorts <<<"${FIXTURE#repo:}"
  for s in "${shorts[@]}"; do
    if [[ ${#shorts[@]} -eq 1 ]]; then d="."; elif [[ "$s" == *api ]]; then d="api"; else d="app"; fi
    repos+=("$s:$d")
  done
  [[ -n "$GATES" ]] || GATES="gobuild govet gofmt hidden"
else
  [[ -n "$GATES" ]] || GATES="tests vet tsc build hidden"
fi

declare -A gate
order=()
# gate <name> <cmd...> -- run in the current shell dir, record 1/0 and print the result.
gate() {
  local name="$1"
  shift
  echo "== gate $name"
  if "$@"; then gate[$name]=1; else gate[$name]=0; fi
  order+=("$name")
  echo "gate $name: $([[ ${gate[$name]} == 1 ]] && echo PASS || echo FAIL)"
}
api() { (cd "$wd/api" && "$@"); }
web() { (cd "$wd/web" && "$@"); }

# --- synthetic fixture gates (v1) ---
# shellcheck disable=SC2086 # GO_FLAGS is intentionally word-split
tests_cmd() {
  local ok=0
  api timeout 300 go test -count=1 $GO_FLAGS ./... || ok=1
  web timeout 300 npx vitest run || ok=1
  return "$ok"
}
build_cmd() {
  local ok=0
  api go build ./... || ok=1
  web timeout 300 npx vite build || ok=1
  return "$ok"
}
# shellcheck disable=SC2086
hidden_synthetic() {
  local ok=0 f
  if compgen -G "$hidden/api/*" >/dev/null; then
    for f in "$hidden"/api/*; do rm -f "$wd/api/$(basename "$f")"; done
    cp "$hidden"/api/* "$wd/api/"
    api timeout 300 go test -count=1 $GO_FLAGS -run 'TestHidden' ./... || ok=1
  fi
  if compgen -G "$hidden/web/*" >/dev/null; then
    mkdir -p "$wd/web/src/hidden"
    for f in "$hidden"/web/*; do rm -f "$wd/web/src/hidden/$(basename "$f")"; done
    cp "$hidden"/web/* "$wd/web/src/hidden/"
    web timeout 300 npx vitest run src/hidden || ok=1
  fi
  return "$ok"
}

# --- real repo gates ---
# for_repos <kind: api|app> <fn short dir>: run fn for every repo of that kind; fails if any fails
for_repos() {
  local kind="$1" fn="$2" ok=0 e s d
  shift 2
  for e in "${repos[@]}"; do
    s="${e%%:*}" d="${e#*:}"
    [[ "$s" == *"$kind" ]] || continue
    "$fn" "$s" "$wd/$d" "$@" || ok=1
  done
  return "$ok"
}
# no_new <label> <current-output-file> <baseline-norm-file>: fail when a normalised line is not in the baseline
no_new() {
  local label="$1" cur="$2" base="$3" new
  [[ -f "$base" ]] || base=/dev/null # baseline absent = treated as empty
  new="$(normalize <"$cur" | comm -13 "$base" - | grep . || true)"
  if [[ -n "$new" ]]; then
    echo "NEW $label findings vs baseline:"
    echo "$new" | head -40
    return 1
  fi
  echo "$label: no new findings vs baseline"
}
gobuild_one() { (cd "$2" && go build ./...); }
govet_one() {
  local out
  out="$(mktemp)"
  (cd "$2" && go vet ./pkg/... >"$out" 2>&1) || true
  no_new "go vet" "$out" "$BENCH/tasks-private/baseline/$1.vet.norm"
  local rc=$?
  rm -f "$out"
  return "$rc"
}
gofmt_one() {
  local out
  out="$(mktemp)"
  (cd "$2" && gofmt -l . >"$out" 2>&1) || true
  no_new "gofmt" "$out" "$BENCH/tasks-private/baseline/$1.gofmt.norm"
  local rc=$?
  rm -f "$out"
  return "$rc"
}
tsc_one() {
  local out
  out="$(mktemp)"
  (cd "$2" && timeout 600 npx tsc --noEmit >"$out" 2>&1) || true
  no_new "tsc" "$out" "$BENCH/tasks-private/baseline/$1.tsc.norm"
  local rc=$?
  rm -f "$out"
  return "$rc"
}
nodetest_one() {
  local out base cur
  out="$(mktemp)"
  (cd "$2" && timeout 600 bash -c "$NODETEST_CMD" >"$out" 2>&1) || true
  cur="$(grep -m1 '^# fail' "$out" | awk '{print $3}')"
  base="$(grep -m1 '^# fail' "$BENCH/tasks-private/baseline/$1.txt" | awk '{print $3}')"
  rm -f "$out"
  echo "existing node tests: failures now=${cur:-?} baseline=${base:-0}"
  [[ -n "$cur" && "$cur" -le "${base:-0}" ]]
}

# hidden tests for real repos; files mirror the workdir layout
hidden_repos() {
  local ok=0 f rel e s d dir base turn="${HIDDEN_TURN:-}" root srcs=() copied=() pkgs=() tests=()
  [[ -d "$hidden" ]] && srcs+=("$hidden")
  for root in "$tdir"/hidden-turns/*/; do
    [[ -d "$root" ]] || continue
    if [[ -z "$turn" || $((10#$(basename "$root"))) -le $((10#$turn)) ]]; then srcs+=("${root%/}"); fi
  done
  for root in "${srcs[@]}"; do
    while IFS= read -r -d '' f; do
      rel="${f#"$root"/}"
      mkdir -p "$wd/$(dirname "$rel")"
      cp "$f" "$wd/$rel"
      copied+=("$wd/$rel")
    done < <(find "$root" -type f -print0)
  done
  for e in "${repos[@]}"; do
    s="${e%%:*}" d="${e#*:}"
    if [[ "$s" == *api ]]; then
      mapfile -t pkgs < <(for f in "${copied[@]}"; do
        if [[ "$f" == *_test.go && ( "$d" == . || "$f" == "$wd/$d/"* ) ]]; then dirname "$f"; fi
      done | sort -u)
      base="$wd"
      [[ "$d" == . ]] || base="$wd/$d"
      for dir in "${pkgs[@]}"; do
        echo "-- go test ./${dir#"$base/"}"
        (cd "$base" && timeout 300 go test -count=1 -run 'TestHidden' "./${dir#"$base/"}" 2>&1 | sed 's/^/   /'; exit "${PIPESTATUS[0]}") || ok=1
      done
    else
      mapfile -t tests < <(for f in "${copied[@]}"; do
        if [[ ( "$f" == *.test.ts || "$f" == *.test.tsx ) && ( "$d" == . || "$f" == "$wd/$d/"* ) ]]; then echo "${f#"$wd/$d/"}"; fi
      done)
      if [[ ${#tests[@]} -gt 0 ]]; then
        echo "-- node --test ${#tests[@]} file(s)"
        (cd "$wd/$d" && timeout 300 node --require "./$NODE_REGISTER" --test "${tests[@]}" 2>&1 | tail -60; exit "${PIPESTATUS[0]}") || ok=1
      fi
    fi
  done
  if [[ -n "$turn" ]]; then rm -f "${copied[@]}"; fi # keep hidden tests away from the agent between turns
  return "$ok"
}
hidden_cmd() { if [[ -n "$FIXTURE" ]]; then hidden_repos; else hidden_synthetic; fi; }

for g in $GATES; do
  case "$g" in
  tests) gate TESTS tests_cmd ;;
  vet) gate VET api go vet ./... ;;
  tsc) if [[ -n "$FIXTURE" ]]; then gate TSC for_repos app tsc_one; else gate TSC web npx tsc --noEmit; fi ;;
  build) gate BUILD build_cmd ;;
  gobuild) gate GOBUILD for_repos api gobuild_one ;;
  govet) gate GOVET for_repos api govet_one ;;
  gofmt) gate GOFMT for_repos api gofmt_one ;;
  nodetest) gate NODETEST for_repos app nodetest_one ;;
  hidden) ;;
  *)
    echo "unknown gate: $g" >&2
    exit 2
    ;;
  esac
done
[[ " $GATES " == *" hidden "* ]] && gate HIDDEN hidden_cmd # last: copies hidden tests into the workdir

rc=0
line=""
for k in HIDDEN "${order[@]}"; do
  [[ -n "${gate[$k]:-}" && " $line" != *" $k="* ]] || continue
  line+="$k=${gate[$k]} "
  [[ ${gate[$k]} == 1 ]] || rc=1
done
echo "gates: $line"
if [[ -n "$gates_file" ]]; then echo "$line" | tr -s ' ' '\n' | sed '/^$/d' >"$gates_file"; fi
if [[ $rc -eq 0 ]]; then echo "RESULT: PASS"; else echo "RESULT: FAIL"; fi
exit $rc
