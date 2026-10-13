#!/usr/bin/env bash
# reset.sh <task> <A|B|C|V0|V1|V2> -- create a fresh, isolated workdir for one run and print its path.
# Task lookup: tasks/<task> then tasks-private/<task>. meta.env FIXTURE:
#   (unset)                 the synthetic notes fixture in fixture/ (api/ + web/)
#   repo:<short>[,<short>]  pinned snapshot(s) from ~/.cache/bench-repos (see prep-repos.sh). One repo is laid out at the
#                           workdir root; several are laid out as api/ (names ending in "api") and app/ ("app").
# <task>/setup.patch (optional, `git apply` from the workdir root) plants a bug/state BEFORE the single baseline commit,
# so the planted diff is invisible to the agent and to the code-size metrics.
set -euo pipefail
BENCH="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO="$(cd "$BENCH/.." && pwd)"
# shellcheck disable=SC1091
source "$BENCH/scripts/lib.sh"
task="${1:?usage: reset.sh <task> <A|B|C|V0|V1|V2>}"
mode="${2:?usage: reset.sh <task> <A|B|C|V0|V1|V2>}"
tdir="$(task_dir "$task")" || {
  echo "unknown task: $task" >&2
  exit 2
}
FIXTURE=""
# shellcheck disable=SC1091
source "$tdir/meta.env"

wd="$(mktemp -d "${TMPDIR:-/tmp}/bench-${task}-${mode}-XXXXXX")"

# copy_tree <src> <dst>: copy-on-write where supported; otherwise everything but node_modules is copied and
# node_modules is hard-linked (cp -al; fast, installs replace files rather than editing them in place).
copy_tree() {
  local src="$1" dst="$2"
  mkdir -p "$dst"
  if cp -a --reflink=always "$src/." "$dst/" 2>/dev/null; then return 0; fi
  rm -rf "$dst" && mkdir -p "$dst"
  (cd "$src" && tar cf - --exclude=./node_modules .) | tar xf - -C "$dst"
  if [[ -d "$src/node_modules" ]]; then
    cp -al "$src/node_modules" "$dst/node_modules" 2>/dev/null || cp -a --reflink=auto "$src/node_modules" "$dst/node_modules"
  fi
}

if [[ -z "$FIXTURE" ]]; then
  copy_tree "$BENCH/fixture" "$wd"
  if [[ ! -d "$wd/web/node_modules" ]]; then
    echo "fixture/web/node_modules missing; run: (cd $BENCH/fixture/web && npm ci)" >&2
    exit 2
  fi
  printf 'node_modules/\ndist/\n' >"$wd/.gitignore"
elif [[ "$FIXTURE" == repo:* ]]; then
  IFS=, read -r -a shorts <<<"${FIXTURE#repo:}"
  for s in "${shorts[@]}"; do
    cache="$(repo_cache "$s")" || {
      echo "repo '$s' not prepared (run scripts/prep-repos.sh)" >&2
      exit 2
    }
    if [[ ${#shorts[@]} -eq 1 ]]; then sub=""; elif [[ "$s" == *api ]]; then sub="api"; else sub="app"; fi
    copy_tree "$cache" "${wd}${sub:+/$sub}"
  done
  printf 'node_modules/\ndist/\nbuild/\n*.log\n' >"$wd/.gitignore"
else
  echo "bad FIXTURE: $FIXTURE" >&2
  exit 2
fi

if [[ -f "$tdir/setup.patch" ]]; then
  (cd "$wd" && git apply --whitespace=nowarn "$tdir/setup.patch") || {
    echo "setup.patch does not apply for $task" >&2
    exit 2
  }
fi

mkdir -p "$wd/.opencode/agents"
case "$mode" in
A) cp "$BENCH"/legacy-agents/team-*.md "$wd/.opencode/agents/" ;;
B) cp "$BENCH/legacy-agents/bench-solo.md" "$wd/.opencode/agents/" ;;
C) cp "$REPO/agents/team-captain.md" "$wd/.opencode/agents/" ;;
V[0-9]) cp "$BENCH/variants/${mode,,}/team-captain.md" "$wd/.opencode/agents/" ;;
*)
  echo "mode must be A, B, C, V0, V1 or V2" >&2
  exit 2
  ;;
esac

# permissive permissions, but no access to paths outside the workdir (hidden tests live in the repo); MCP servers are additionally dropped via XDG_CONFIG_HOME in run-task.sh
cat >"$wd/opencode.json" <<'JSON'
{
  "$schema": "https://opencode.ai/config.json",
  "permission": {
    "edit": "allow",
    "bash": "allow",
    "webfetch": "allow",
    "external_directory": "deny",
    "doom_loop": "allow"
  },
  "mcp": {},
  "autoupdate": false,
  "share": "disabled"
}
JSON
# variant config fragment (V2: cheaper explore model), deep-merged with jq
if [[ "$mode" == V* && -f "$BENCH/variants/${mode,,}/opencode.fragment.json" ]]; then
  jq -s '.[0] * .[1]' "$wd/opencode.json" "$BENCH/variants/${mode,,}/opencode.fragment.json" >"$wd/opencode.json.tmp"
  mv "$wd/opencode.json.tmp" "$wd/opencode.json"
fi

(
  cd "$wd"
  git init -q
  git config user.email bench@example.invalid
  git config user.name bench
  git add -A
  git commit -q -m "baseline fixture"
)
echo "$wd"
