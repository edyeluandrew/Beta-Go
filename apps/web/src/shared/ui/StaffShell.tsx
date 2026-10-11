import { useEffect, useState } from 'react'
import { NavLink, useLocation, useNavigate } from 'react-router-dom'
import { Flag, LayoutDashboard, LogOut, Menu, Store, Wallet, X } from 'lucide-react'
import { canManageVendors, canTouchMoney, isSuperAdmin, type User } from '../auth/roles'
import { ProfileMark } from './ProfileMark'

const itemClass = ({ isActive }: { isActive: boolean }) =>
  `flex min-h-11 items-center gap-3 rounded-xl px-3 text-sm no-underline ${isActive ? 'bg-leaf font-semibold text-forest-dark' : 'text-ink hover:bg-leaf'}`

export function StaffShell({
  user,
  signOut,
  title,
  lede,
  children,
}: {
  user: User
  signOut: () => Promise<void>
  title: string
  lede: string
  children?: React.ReactNode
}) {
  const navigate = useNavigate()
  const location = useLocation()
  const [open, setOpen] = useState(false)
  const roles = user.roles

  useEffect(() => {
    setOpen(false)
  }, [location.pathname])

  useEffect(() => {
    if (!open) return
    const previous = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false)
    }
    window.addEventListener('keydown', onKey)
    return () => {
      document.body.style.overflow = previous
      window.removeEventListener('keydown', onKey)
    }
  }, [open])

  const leave = () => {
    void signOut().then(() => navigate('/'))
  }

  return (
    <div className="min-h-screen md:grid md:grid-cols-[240px_minmax(0,1fr)]">
      {open ? (
        <button type="button" className="fixed inset-0 z-30 bg-ink/40 md:hidden" aria-label="Close menu" onClick={() => setOpen(false)} />
      ) : null}

      <aside
        className={`fixed inset-y-0 left-0 z-40 flex w-[min(280px,86vw)] flex-col border-r border-line bg-card px-4 py-5 transition-transform md:static md:z-auto md:w-auto md:translate-x-0 ${open ? 'translate-x-0' : '-translate-x-full'}`}
      >
        <div className="flex items-center justify-between">
          <NavLink className="font-display text-[1.6rem] text-ink no-underline" to="/a">
            Beta-Go
          </NavLink>
          <button type="button" className="grid h-11 w-11 place-items-center rounded-full text-ink md:hidden" aria-label="Close menu" onClick={() => setOpen(false)}>
            <X aria-hidden className="h-5 w-5" />
          </button>
        </div>

        <nav className="mt-8 grid gap-1" aria-label="Admin">
          <NavLink to="/a" end className={itemClass}>
            <LayoutDashboard aria-hidden className="h-5 w-5" />
            Dashboard
          </NavLink>
          {canManageVendors(roles) && (
            <NavLink to="/a/vendors" className={itemClass}>
              <Store aria-hidden className="h-5 w-5" />
              Kitchens
            </NavLink>
          )}
          {canTouchMoney(roles) && (
            <NavLink to="/a/money" className={itemClass}>
              <Wallet aria-hidden className="h-5 w-5" />
              Money
            </NavLink>
          )}
          {isSuperAdmin(roles) && (
            <NavLink to="/a/launch" className={itemClass}>
              <Flag aria-hidden className="h-5 w-5" />
              Launch
            </NavLink>
          )}
        </nav>

        <div className="mt-auto grid gap-1 border-t border-line pt-4">
          <NavLink to="/a/profile" className={itemClass} aria-label="Profile">
            <ProfileMark user={user} className="h-8 w-8" />
            Profile
          </NavLink>
          <button type="button" className="flex min-h-11 items-center gap-3 rounded-xl px-3 text-left text-sm text-ink hover:bg-leaf" onClick={leave}>
            <LogOut aria-hidden className="h-5 w-5" />
            Sign out
          </button>
        </div>
      </aside>

      <div className="min-w-0">
        <header className="sticky top-0 z-20 flex items-center gap-2 border-b border-line bg-card/95 px-3 py-2 backdrop-blur md:hidden">
          <button type="button" className="grid h-11 w-11 place-items-center rounded-full text-ink" aria-label="Open menu" onClick={() => setOpen(true)}>
            <Menu aria-hidden className="h-5 w-5" />
          </button>
          <span className="font-display text-xl">Beta-Go</span>
          <NavLink to="/a/profile" aria-label="Profile" className="ml-auto rounded-full no-underline">
            <ProfileMark user={user} />
          </NavLink>
        </header>

        <main className="mx-auto mb-16 mt-5 w-[min(880px,calc(100%-1.5rem))] sm:mt-8 sm:w-[min(880px,calc(100%-2.5rem))]">
          <h1 className="mb-2 text-[1.85rem] leading-tight sm:text-[2.4rem]">{title}</h1>
          <p className="text-muted">{lede}</p>
          {children}
        </main>
      </div>
    </div>
  )
}
