import type { ReactNode } from 'react'
import { Navigate, Outlet } from 'react-router-dom'
import { useAuth } from '../shared/auth/AuthProvider'
import { hasRole } from '../shared/auth/roles'
import { Shell } from '../shared/ui/Shell'
import { Notice } from '../shared/ui/widgets'

export function RequireAuth() {
  const { user, ready } = useAuth()
  if (!ready) return <p className="px-5 py-12 text-center text-muted">Checking your session…</p>
  if (!user) return <Navigate to="/signin" replace />
  return <Outlet />
}

export function RequireRole({ roles, children }: { roles: string[]; children: ReactNode }) {
  const { user } = useAuth()
  if (!user || !hasRole(user.roles, roles)) {
    return <Shell title="No access" lede="This workspace belongs to a different role.">
      <Notice>The server still checks the role on every request. This screen is only a hide.</Notice>
    </Shell>
  }
  return children
}
