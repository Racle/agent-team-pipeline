export interface Note {
  id: number
  title: string
  body: string
  created_at: string
  updated_at: string
}

export interface NoteInput {
  title: string
  body: string
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init)
  if (!res.ok) {
    throw new Error(`Request failed: ${res.status}`)
  }
  return res.status === 204 ? (undefined as T) : ((await res.json()) as T)
}

export function listNotes(): Promise<Note[]> {
  return request<Note[]>('/api/notes')
}

export function createNote(input: NoteInput): Promise<Note> {
  return request<Note>('/api/notes', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export function deleteNote(id: number): Promise<void> {
  return request<void>(`/api/notes/${id}`, { method: 'DELETE' })
}
