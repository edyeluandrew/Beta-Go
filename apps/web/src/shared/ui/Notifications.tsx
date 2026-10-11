import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Bell } from 'lucide-react'
import { api } from '../api/client'

type Note = {
  id: string
  title: string
  body: string
  href: string
  read: boolean
  createdAt: string
}

type Page = { items: Note[]; unread: number }

export function Notifications() {
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const [items, setItems] = useState<Note[]>([])
  const [unread, setUnread] = useState(0)

  useEffect(() => {
    let stopped = false
    let timer = 0
    let socket: WebSocket | null = null

    const load = async () => {
      try {
        const page = await api<Page>('/api/v1/notifications')
        if (!stopped) {
          setItems(page.items ?? [])
          setUnread(page.unread ?? 0)
        }
      } catch {
        // The bell stays quiet when the list cannot be loaded.
      }
    }

    const connect = async () => {
      if (stopped) return
      try {
        const ticket = await api<{ url: string; ticket: string }>('/api/v1/notifications/ticket', { method: 'POST' })
        if (stopped) return
        socket = new WebSocket(`${ticket.url}?ticket=${encodeURIComponent(ticket.ticket)}`)
        socket.onmessage = (event) => {
          const note = JSON.parse(String(event.data)) as Note
          if (!note?.id) return
          setItems((current) => [note, ...current.filter((item) => item.id !== note.id)].slice(0, 30))
          setUnread((count) => count + 1)
        }
        socket.onclose = () => {
          if (!stopped) timer = window.setTimeout(() => void connect(), 4000)
        }
      } catch {
        if (!stopped) timer = window.setTimeout(() => void connect(), 8000)
      }
    }

    void load()
    void connect()
    return () => {
      stopped = true
      window.clearTimeout(timer)
      socket?.close()
    }
  }, [])

  const openNote = async (note: Note) => {
    setOpen(false)
    if (!note.read) {
      setItems((current) => current.map((item) => (item.id === note.id ? { ...item, read: true } : item)))
      setUnread((count) => Math.max(0, count - 1))
      try {
        await api(`/api/v1/notifications/${note.id}/read`, { method: 'POST' })
      } catch {
        // The note still opens if marking it read fails.
      }
    }
    if (note.href) navigate(note.href)
  }

  const label = unread > 0 ? `${unread} unread notifications` : 'Notifications'

  return (
    <div className="relative">
      <button
        type="button"
        className="relative grid h-11 w-11 place-items-center rounded-full text-ink hover:bg-leaf"
        aria-label={label}
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
      >
        <Bell aria-hidden className="h-5 w-5" />
        {unread > 0 ? (
          <span className="absolute right-1 top-1 grid h-5 min-w-5 place-items-center rounded-full bg-forest px-1 text-[11px] font-semibold text-white">
            {unread > 9 ? '9+' : unread}
          </span>
        ) : null}
      </button>
      {open ? (
        <div className="absolute right-0 z-50 mt-2 w-[min(320px,80vw)] rounded-2xl border border-line bg-card p-2 shadow-lg">
          <p className="px-2 py-1 text-sm font-semibold">Notifications</p>
          {items.length === 0 ? (
            <p className="px-2 py-3 text-sm text-muted">Nothing new yet.</p>
          ) : (
            <ul className="grid max-h-80 gap-1 overflow-auto">
              {items.map((note) => (
                <li key={note.id}>
                  <button type="button" className="w-full rounded-xl px-2 py-2 text-left hover:bg-leaf" onClick={() => void openNote(note)}>
                    <span className="flex items-center gap-2">
                      {!note.read ? <span className="h-2 w-2 shrink-0 rounded-full bg-forest" /> : null}
                      <span className="text-sm font-semibold">{note.title}</span>
                    </span>
                    {note.body ? <span className="mt-1 block text-sm text-muted">{note.body}</span> : null}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      ) : null}
    </div>
  )
}
