import { useState, type FormEvent } from 'react'
import type { NoteInput } from './api'

interface Props {
  onSubmit: (input: NoteInput) => Promise<void> | void
}

export default function NoteForm({ onSubmit }: Props) {
  const [title, setTitle] = useState('')
  const [body, setBody] = useState('')

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    await onSubmit({ title, body })
    setTitle('')
    setBody('')
  }

  return (
    <form onSubmit={handleSubmit}>
      <label>
        Title
        <input value={title} onChange={e => setTitle(e.target.value)} />
      </label>
      <label>
        Body
        <textarea value={body} onChange={e => setBody(e.target.value)} />
      </label>
      <button type='submit'>Add note</button>
    </form>
  )
}
