import type { Note } from './api'

interface Props {
  notes: Note[]
  onDelete: (id: number) => void
}

export default function NotesList({ notes, onDelete }: Props) {
  return (
    <ul>
      {notes.map(n => (
        <li key={n.id}>
          <strong>{n.title}</strong>
          <p>{n.body}</p>
          <button onClick={() => onDelete(n.id)}>Delete</button>
        </li>
      ))}
    </ul>
  )
}
