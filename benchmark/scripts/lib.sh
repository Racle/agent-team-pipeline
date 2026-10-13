#!/usr/bin/env bash
# lib.sh -- helpers shared by the benchmark scripts (source it; needs $BENCH).
# task_dir <task> prints the task directory (tasks/ is tracked, tasks-private/ is gitignored); empty + rc 1 if unknown.
task_dir() {
  local t="$1"
  if [[ -d "$BENCH/tasks/$t" ]]; then echo "$BENCH/tasks/$t"; elif [[ -d "$BENCH/tasks-private/$t" ]]; then echo "$BENCH/tasks-private/$t"; else return 1; fi
}
# all_task_dirs prints every task directory (tasks/*/ then tasks-private/*/, only dirs that have meta.env).
all_task_dirs() {
  local d
  for d in "$BENCH"/tasks/*/ "$BENCH"/tasks-private/*/; do [[ -f "$d/meta.env" ]] && echo "${d%/}"; done
}
# repo_cache <short> prints the cached snapshot dir of a pinned repo (see prep-repos.sh), e.g. repo_cache hmapi.
repo_cache() {
  local key line
  key="$(tr '[:lower:]' '[:upper:]' <<<"$1")"
  line="$(grep "^$key=" "$BENCH/tasks-private/repos.env" 2>/dev/null | cut -d= -f2-)" || true
  [[ -n "$line" && -d "${BENCH_CACHE:-$HOME/.cache/bench-repos}/$line" ]] || return 1
  echo "${BENCH_CACHE:-$HOME/.cache/bench-repos}/$line"
}
# normalize <file-or-stdin>: strip line/col numbers so unrelated edits do not shift baseline error lines.
normalize() { sed -E 's/:[0-9]+(:[0-9]+)?:/:/; s/\([0-9]+,[0-9]+\)//' | sort -u; }
