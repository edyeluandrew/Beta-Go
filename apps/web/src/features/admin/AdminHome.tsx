import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, ApiError } from '../../shared/api/client'
import { useAuth } from '../../shared/auth/AuthProvider'
import { canManageVendors } from '../../shared/auth/roles'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText } from '../../shared/ui/widgets'

type Overview = {
  activeKitchens: number
  ordersWaiting: number
  openDisputes: number
}

type Note = {
  id: string
  title: string
  body: string
  href: string
  createdAt: string
}

const cardClass = 'rounded-[20px] border border-line bg-card p-5 text-ink no-underline'

export function AdminHome() {
  const { user } = useAuth()
  const roles = user?.roles ?? []
  const [overview, setOverview] = useState<Overview | null>(null)
  const [notes, setNotes] = useState<Note[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    let stopped = false
    const load = async () => {
      try {
        const [counts, page] = await Promise.all([
          api<Overview>('/api/v1/admin/overview'),
          api<{ items: Note[] }>('/api/v1/notifications'),
        ])
        if (!stopped) {
          setOverview(counts)
          setNotes((page.items ?? []).slice(0, 5))
        }
      } catch (err: unknown) {
        if (!stopped) setError(err instanceof ApiError ? err.message : 'The dashboard could not be loaded.')
      }
    }
    void load()
    return () => {
      stopped = true
    }
  }, [])

  return (
    <Shell title={`Hi ${user?.name || 'there'}`} lede="Kitchens that are live, orders waiting on a cook, and disputes still open.">
      <ErrorText>{error}</ErrorText>
      <section className="mt-6 grid gap-3 sm:grid-cols-3">
        {canManageVendors(roles) ? (
          <Link className={cardClass} to="/a/vendors">
            <p className="text-sm text-muted">Kitchens cooking</p>
            <p className="mt-2 font-display text-4xl">{overview?.activeKitchens ?? '—'}</p>
          </Link>
        ) : (
          <div className={cardClass}>
            <p className="text-sm text-muted">Kitchens cooking</p>
            <p className="mt-2 font-display text-4xl">{overview?.activeKitchens ?? '—'}</p>
          </div>
        )}
        <Link className={cardClass} to="/a/reconcile">
          <p className="text-sm text-muted">Orders waiting</p>
          <p className="mt-2 font-display text-4xl">{overview?.ordersWaiting ?? '—'}</p>
        </Link>
        <Link className={cardClass} to="/a/reconcile">
          <p className="text-sm text-muted">Open disputes</p>
          <p className="mt-2 font-display text-4xl">{overview?.openDisputes ?? '—'}</p>
        </Link>
      </section>
      <section className="mt-6 rounded-[20px] border border-line bg-card p-6">
        <h2 className="text-2xl">Latest</h2>
        {notes.length === 0 ? (
          <p className="mt-3 text-sm text-muted">Quiet for now. New orders, payments, and disputes show up here.</p>
        ) : (
          <ul className="mt-3 grid gap-3">
            {notes.map((note) => (
              <li key={note.id}>
                {note.href ? (
                  <Link className="block text-ink no-underline" to={note.href}>
                    <span className="font-semibold">{note.title}</span>
                    {note.body ? <span className="mt-1 block text-sm text-muted">{note.body}</span> : null}
                  </Link>
                ) : (
                  <div>
                    <span className="font-semibold">{note.title}</span>
                    {note.body ? <span className="mt-1 block text-sm text-muted">{note.body}</span> : null}
                  </div>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>
    </Shell>
  )
}
