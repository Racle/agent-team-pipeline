#!/usr/bin/env bash
# prep-repos.sh [short...] -- pin + snapshot the real repos used by v2 tasks (read-only on the source repos).
# For each repo: pin HEAD sha -> tasks-private/repos.env, `git archive <sha>` (tracked files only, so no .env,
# no local changes) into ~/.cache/bench-repos/<dir>@<sha>/, run `npm ci` / `go mod download` once, and capture the
# baseline gate state + timings into tasks-private/baseline/<short>.txt (and normalised *.norm files that verify.sh
# compares against for "no new errors"). Initialised git submodules are archived at their checked-out sha.
# Env: REPOS_LIST (default tasks-private/repos.list), BENCH_CACHE (default ~/.cache/bench-repos).
set -uo pipefail
BENCH="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CACHE="${BENCH_CACHE:-$HOME/.cache/bench-repos}"
OUT="$BENCH/tasks-private"
mkdir -p "$OUT/baseline" "$CACHE"
touch "$OUT/repos.env"

# Repo list: tasks-private/repos.list (gitignored), one "<alias> <absolute path>" per line; '#' comments allowed.
LIST="${REPOS_LIST:-$OUT/repos.list}"
if [[ ! -f $LIST ]]; then echo "missing $LIST (format: <alias> <absolute path> per line)" >&2; exit 1; fi
REPOS=()
while read -r alias path _; do
  [[ -z $alias || $alias == \#* ]] && continue
  REPOS+=("$alias $path")
done <"$LIST"
want=("$@")

# norm: strip line/col numbers so unrelated edits do not shift baseline lines
norm() { sed -E 's/:[0-9]+(:[0-9]+)?:/:/; s/\([0-9]+,[0-9]+\)//' | sort -u; }
# timed <file> <label> <cmd...>: run, log output + elapsed seconds + exit code into <file>
timed() {
  local f="$1" label="$2" s rc
  shift 2
  s=$(date +%s)
  echo "### $label: $*" >>"$f"
  if [[ -n "${RAW:-}" ]]; then "$@" 2>&1 | tee "$RAW" >>"$f"; rc=${PIPESTATUS[0]}; else "$@" >>"$f" 2>&1; rc=$?; fi
  echo "### $label: exit=$rc elapsed=$(($(date +%s) - s))s" >>"$f"
  return "$rc"
}

for entry in "${REPOS[@]}"; do
  read -r short src <<<"$entry"
  if [[ -f $src/go.mod ]]; then kind=go; else kind=node; fi
  if [[ ${#want[@]} -gt 0 ]] && [[ ! " ${want[*]} " == *" $short "* ]]; then continue; fi
  dir="$(basename "$src")"
  sha="$(git -C "$src" rev-parse HEAD)"
  key="$(tr "[:lower:]" "[:upper:]" <<<"$short")"
  grep -v "^$key=" "$OUT/repos.env" >"$OUT/repos.env.tmp" || true
  echo "$key=$dir@$sha" >>"$OUT/repos.env.tmp"
  mv "$OUT/repos.env.tmp" "$OUT/repos.env"
  dst="$CACHE/$dir@$sha"
  log="$OUT/baseline/$short.txt"
  echo "[$short] $dir@$sha"
  t0=$(date +%s)
  if [[ ! -d "$dst" ]]; then
    mkdir -p "$dst.tmp" && git -C "$src" archive "$sha" | tar -x -C "$dst.tmp"
    # initialised submodules: git archive skips them; archive the checked-out commit
    while read -r _ spath; do
      ssha="$(git -C "$src/$spath" rev-parse HEAD 2>/dev/null)" || continue
      mkdir -p "$dst.tmp/$spath" && git -C "$src/$spath" archive "$ssha" | tar -x -C "$dst.tmp/$spath"
    done < <(git -C "$src" submodule status 2>/dev/null | grep -v '^-' | awk '{print $1, $2}')
    find "$dst.tmp" \( -name ".env" -o -name ".env.*" \) -delete # never keep env files
    mv "$dst.tmp" "$dst"
  fi
  : >"$log"
  echo "repo=$dir sha=$sha archive_s=$(($(date +%s) - t0))" >>"$log"
  cd "$dst" || continue
  if [[ $kind == go ]]; then
    timed "$log" "go mod download" go mod download
    timed "$log" "go build ./..." go build ./...
    RAW="$OUT/baseline/$short.vet.raw" timed "$log" "go vet ./pkg/..." go vet ./pkg/...
    RAW="$OUT/baseline/$short.gofmt.raw" timed "$log" "gofmt -l" gofmt -l .
    norm <"$OUT/baseline/$short.vet.raw" >"$OUT/baseline/$short.vet.norm"
    norm <"$OUT/baseline/$short.gofmt.raw" >"$OUT/baseline/$short.gofmt.norm"
    timed "$log" "go test ./pkg/..." go test -count=1 ./pkg/...
  else
    # plain npm ci hits peer-dep ERESOLVE in both apps; --legacy-peer-deps is the fallback
    if [[ ! -d node_modules ]]; then
      timed "$log" "npm ci" npm ci --no-audit --no-fund || timed "$log" "npm ci --legacy-peer-deps" npm ci --legacy-peer-deps --no-audit --no-fund
    fi
    export NODE_OPTIONS=--max-old-space-size=12288 # hmapp tsc OOMs at the default heap
    RAW="$OUT/baseline/$short.tsc.raw" timed "$log" "tsc --noEmit" npx tsc --noEmit
    norm <"$OUT/baseline/$short.tsc.raw" >"$OUT/baseline/$short.tsc.norm"
    if grep -q '"test:voucher-export"' package.json; then timed "$log" "npm run test:voucher-export" npm run test:voucher-export; fi
  fi
  echo "[$short] done in $(($(date +%s) - t0))s"
  grep '^### .*exit=' "$log" | sed 's/^/  /'
done
