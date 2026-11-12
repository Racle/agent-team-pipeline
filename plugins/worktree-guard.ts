/**
 * worktree-guard: an OpenCode (V2) server plugin that stops two agent sessions
 * from editing the same git checkout at the same time.
 *
 * It hooks permission evaluation for the `edit` action (the edit, write and
 * patch tools). For each target file it finds the checkout root
 * (`git rev-parse --show-toplevel`; every worktree has its own) and remembers
 * which sessions edited that checkout. An edit is denied while another of
 * those sessions is running, unless one session is an ancestor of the other (a
 * parent waiting on its own subagent). Parallel sibling subagents are denied.
 *
 * State is in memory only, so a server restart clears it and there are no
 * stale lock files. Shell commands that write files are not seen. Any lookup
 * error fails open: the guard never blocks because of its own failure.
 */
import { execFileSync } from 'node:child_process'
import { existsSync } from 'node:fs'
import { dirname, isAbsolute, resolve } from 'node:path'

type Evaluation = {
  readonly sessionID: string
  readonly action: string
  readonly resources: readonly string[]
  effect: 'allow' | 'ask' | 'deny'
  message?: string
}

type SessionInfo = {
  id: string
  parentID?: string
  title?: string
  location: { directory: string }
  time: { busy?: number; idle?: number }
}

/** True while the session is working on a turn: it went busy after it last went idle. */
export const isBusy = (s: SessionInfo): boolean => !!s.time.busy && (!s.time.idle || s.time.busy > s.time.idle)

/** Unwraps client responses that may come as the value or as `{ data }`. */
const unwrap = <T>(r: unknown): T => ((r as { data?: T })?.data ?? r) as T

/**
 * Returns the ID of a running session that edited the same checkout and is not
 * `self`, an ancestor of `self`, or a descendant of `self`; otherwise undefined.
 * `editors` maps each editor session to its ancestor IDs.
 */
export function findConflict(self: string, selfAncestors: ReadonlySet<string>, editors: ReadonlyMap<string, ReadonlySet<string>>, running: ReadonlySet<string>): string | undefined {
  for (const [other, otherAncestors] of editors) {
    if (other === self || !running.has(other)) continue
    if (selfAncestors.has(other) || otherAncestors.has(self)) continue
    return other
  }
  return undefined
}

const rootCache = new Map<string, string | null>()

/** Git checkout root containing `file` (which may not exist yet), or null outside git. */
function checkoutRoot(file: string): string | null {
  let dir = dirname(file)
  while (!existsSync(dir) && dirname(dir) !== dir) dir = dirname(dir)
  if (!rootCache.has(dir)) {
    let root: string | null = null
    try {
      root = execFileSync('git', ['-C', dir, 'rev-parse', '--show-toplevel'], {
        encoding: 'utf8',
        stdio: ['ignore', 'pipe', 'ignore'],
      }).trim()
    } catch {
      // Not a git checkout: nothing to guard.
    }
    rootCache.set(dir, root)
  }
  return rootCache.get(dir) ?? null
}

// Module scope, not per setup(): OpenCode runs one plugin instance per location,
// and sessions in different subdirectories of one checkout must share this state.
/** checkout root -> (editor session -> its ancestor IDs) */
const editorsByRoot = new Map<string, Map<string, Set<string>>>()
const sessions = new Map<string, SessionInfo>()

export default {
  id: 'worktree-guard',
  async setup(ctx: any) {
    /** Session info; parents are cached, `fresh` refetches (location changes on session_move). */
    const getSession = async (id: string, fresh = false): Promise<SessionInfo> => {
      let s = fresh ? undefined : sessions.get(id)
      if (!s) {
        s = unwrap<SessionInfo>(await ctx.session.get({ sessionID: id }))
        sessions.set(id, s)
      }
      return s
    }

    const ancestorsOf = async (id: string): Promise<Set<string>> => {
      const out = new Set<string>()
      for (let s = await getSession(id); s.parentID && !out.has(s.parentID); s = await getSession(s.parentID)) {
        out.add(s.parentID)
      }
      return out
    }

    await ctx.permission.hook('evaluate', async (event: Evaluation) => {
      if (event.action !== 'edit' || event.effect === 'deny') return
      try {
        const self = await getSession(event.sessionID, true)
        const roots = new Set<string>()
        for (const r of event.resources) {
          const root = checkoutRoot(isAbsolute(r) ? r : resolve(self.location.directory, r))
          if (root) roots.add(root)
        }
        if (roots.size === 0) return

        const selfAncestors = await ancestorsOf(self.id)
        for (const root of roots) {
          const editors = editorsByRoot.get(root) ?? new Map<string, Set<string>>()
          const running = new Set<string>()
          for (const id of editors.keys()) {
            if (id === self.id) continue
            const s = await getSession(id, true).catch(() => undefined)
            if (!s)
              editors.delete(id) // deleted session
            else if (isBusy(s)) running.add(id)
          }
          const other = findConflict(self.id, selfAncestors, editors, running)
          if (other) {
            const title = sessions.get(other)?.title
            event.effect = 'deny'
            event.message =
              `worktree-guard: session ${other}${title ? ` ("${title}")` : ''} is running and editing ${root}. ` +
              "Don't edit this checkout in parallel: create a worktree, move this session into it (session_move), " +
              'run `git switch -c <task>`, then retry. Or stop and tell the user to wait for the other session.'
            return
          }
        }
        // A session edits one checkout at a time; forget older ones (e.g. after session_move to a worktree).
        for (const [root, editors] of editorsByRoot) if (!roots.has(root)) editors.delete(self.id)
        for (const root of roots) {
          const editors = editorsByRoot.get(root) ?? new Map<string, Set<string>>()
          editors.set(self.id, selfAncestors)
          editorsByRoot.set(root, editors)
        }
      } catch (err) {
        console.error('worktree-guard: check skipped:', err)
      }
    })
  },
}
