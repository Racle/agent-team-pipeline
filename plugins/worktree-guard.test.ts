import assert from 'node:assert/strict'
import { test } from 'node:test'
import { findConflict, isBusy } from './worktree-guard.ts'

const none = new Set<string>()

test('no other editors: allowed', () => {
  assert.equal(findConflict('a', none, new Map(), new Set(['a'])), undefined)
})

test('another running editor of the same checkout: denied', () => {
  assert.equal(findConflict('b', none, new Map([['a', none]]), new Set(['a', 'b'])), 'a')
})

test('other editor finished (idle): allowed', () => {
  assert.equal(findConflict('b', none, new Map([['a', none]]), new Set(['b'])), undefined)
})

test('same session editing again: allowed', () => {
  assert.equal(findConflict('a', none, new Map([['a', none]]), new Set(['a'])), undefined)
})

test('subagent editing while its parent (an editor) waits: allowed', () => {
  assert.equal(findConflict('child', new Set(['parent']), new Map([['parent', none]]), new Set(['parent', 'child'])), undefined)
})

test('parent editing while its subagent (an editor) still runs: allowed', () => {
  const editors = new Map([['child', new Set(['parent'])]])
  assert.equal(findConflict('parent', none, editors, new Set(['parent', 'child'])), undefined)
})

test('parallel sibling subagents: denied', () => {
  const editors = new Map([['c1', new Set(['p'])]])
  assert.equal(findConflict('c2', new Set(['p']), editors, new Set(['p', 'c1', 'c2'])), 'c1')
})

const at = (busy?: number, idle?: number) => ({ id: 's', location: { directory: '/' }, time: { busy, idle } })

test('isBusy: busy after last idle, never busy, idle after busy', () => {
  assert.equal(isBusy(at(2, 1)), true)
  assert.equal(isBusy(at(2)), true)
  assert.equal(isBusy(at()), false)
  assert.equal(isBusy(at(1, 2)), false)
})
