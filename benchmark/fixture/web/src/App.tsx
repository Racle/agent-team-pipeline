import { useCallback, useEffect, useState } from 'react'
import { createNote, deleteNote, listNotes, type Note, type NoteInput } from './api'
import NoteForm from './NoteForm'
import NotesList from './NotesList'

export default function App() {
  const [notes, setNotes] = useState<Note[]>([])

  const refresh = useCallback(async () => {
    setNotes(await listNotes())
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  async function handleCreate(input: NoteInput) {
    await createNote(input)
    await refresh()
  }

  async function handleDelete(id: number) {
    await deleteNote(id)
    await refresh()
  }

  return (
    <main>
      <h1>Notes</h1>
      <NoteForm onSubmit={handleCreate} />
      <NotesList notes={notes} onDelete={handleDelete} />
    </main>
  )
}
