import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import App from './App'

const note = { id: 1, title: 'First', body: 'Hello', created_at: '', updated_at: '' }

function json(body: unknown, status = 200) {
  return Promise.resolve(new Response(status === 204 ? null : JSON.stringify(body), { status }))
}

beforeEach(() => {
  vi.stubGlobal(
    'fetch',
    vi.fn(() => json([note]))
  )
})
afterEach(() => vi.unstubAllGlobals())

it('lists notes from the API', async () => {
  render(<App />)
  expect(await screen.findByText('First')).toBeInTheDocument()
  expect(screen.getByText('Hello')).toBeInTheDocument()
})

it('creates a note via the form', async () => {
  const fetchMock = vi.fn((url: string, init?: RequestInit) => (init?.method === 'POST' ? json(note, 201) : json([note])))
  vi.stubGlobal('fetch', fetchMock)
  render(<App />)
  await userEvent.type(screen.getByLabelText('Title'), 'T')
  await userEvent.click(screen.getByRole('button', { name: 'Add note' }))
  await waitFor(() => expect(fetchMock).toHaveBeenCalledWith('/api/notes', expect.objectContaining({ method: 'POST' })))
})

it('deletes a note', async () => {
  const fetchMock = vi.fn((url: string, init?: RequestInit) => (init?.method === 'DELETE' ? json(null, 204) : json([note])))
  vi.stubGlobal('fetch', fetchMock)
  render(<App />)
  await userEvent.click(await screen.findByRole('button', { name: 'Delete' }))
  await waitFor(() => expect(fetchMock).toHaveBeenCalledWith('/api/notes/1', expect.objectContaining({ method: 'DELETE' })))
})
