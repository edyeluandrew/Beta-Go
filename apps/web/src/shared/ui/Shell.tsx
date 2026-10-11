import { NavLink, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthProvider'
import { canManageVendors, canTouchMoney, isStaff, isSuperAdmin } from '../auth/roles'
import { ProfileMark } from './ProfileMark'
import { GhostButton } from './widgets'

const linkClass = ({ isActive }: { isActive: boolean }) =>
  `rounded-full px-2.5 py-1.5 no-underline ${isActive ? 'bg-leaf text-forest-dark' : 'text-muted hover:bg-leaf hover:text-forest-dark'}`

export function Shell({ title, lede, children }: { title: string; lede: string; children?: React.ReactNode }) {
  const { user, signOut } = useAuth()
  const navigate = useNavigate()
  const roles = user?.roles ?? []
  const home = !user ? '/' : isStaff(roles) ? '/a' : roles.includes('vendor') ? '/v' : '/c'

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-10 flex flex-wrap items-center gap-4 border-b border-line bg-card/90 px-5 py-4 backdrop-blur">
        <NavLink className="font-display text-[1.4rem] text-ink no-underline" to={home}>
          Beta-Go
        </NavLink>
        <nav className="flex flex-1 flex-wrap gap-2">
          <NavLink to="/" end className={linkClass}>
            Home
          </NavLink>
          <NavLink to="/c" end className={linkClass}>
            Kitchens
          </NavLink>
          {roles.includes('customer') && (
            <>
              <NavLink to="/c/account" className={linkClass}>
                Dashboard
              </NavLink>
              <NavLink to="/c/orders" className={linkClass}>
                Orders
              </NavLink>
            </>
          )}
          {roles.includes('vendor') && (
            <>
              <NavLink to="/v" end className={linkClass}>
                Shop
              </NavLink>
              <NavLink to="/v/orders" className={linkClass}>
                Kitchen
              </NavLink>
              <NavLink to="/v/pool" className={linkClass}>
                Pool
              </NavLink>
            </>
          )}
          {isStaff(roles) && (
            <>
              <NavLink to="/a" end className={linkClass}>
                Admin
              </NavLink>
              {canManageVendors(roles) && (
                <NavLink to="/a/vendors" className={linkClass}>
                  Admin kitchens
                </NavLink>
              )}
              {canTouchMoney(roles) && (
                <NavLink to="/a/money" className={linkClass}>
                  Money
                </NavLink>
              )}
              {isSuperAdmin(roles) && (
                <NavLink to="/a/launch" className={linkClass}>
                  Launch
                </NavLink>
              )}
            </>
          )}
          {user && !roles.includes('vendor') && (
            <NavLink to="/invite" className={linkClass}>
              Invite
            </NavLink>
          )}
        </nav>
        <div className="ml-auto flex items-center gap-2.5">
          {user ? (
            <>
              {roles.includes('customer') ? (
                <NavLink to="/c/profile" aria-label="Profile" title="Profile" className="rounded-full no-underline">
                  <ProfileMark user={user} />
                </NavLink>
              ) : (
                <span className="max-w-36 truncate text-sm">{user.name}</span>
              )}
              <GhostButton
                type="button"
                onClick={() => {
                  void signOut().then(() => navigate('/'))
                }}
              >
                Sign out
              </GhostButton>
            </>
          ) : (
            <>
              <NavLink to="/signin" className={linkClass}>
                Sign in
              </NavLink>
              <NavLink to="/join" className="rounded-full bg-forest px-3 py-1.5 text-white no-underline">
                Create account
              </NavLink>
            </>
          )}
        </div>
      </header>
      <main className="mx-auto mb-12 mt-7 w-[min(760px,calc(100%-32px))]">
        <h1 className="mb-2 text-[2.4rem] leading-tight">{title}</h1>
        <p className="text-muted">{lede}</p>
        {children}
      </main>
    </div>
  )
}
