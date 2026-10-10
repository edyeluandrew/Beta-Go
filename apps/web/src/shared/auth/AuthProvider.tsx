import { createContext, useContext, useEffect, useMemo, useState } from 'react'
import { api } from '../api/client'
import { homeFor, type User } from './roles'

type AuthState = {
  user: User | null
  ready: boolean
  refresh: () => Promise<User>
  signInWithGoogle: (idToken: string) => Promise<User>
  updateProfile: (body: { name: string; phone: string; area: string }) => Promise<User>
  signOut: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [ready, setReady] = useState(false)

  useEffect(() => {
    let active = true
    api<{ user: User }>('/api/v1/me')
      .then((result) => {
        if (active) setUser(result.user)
      })
      .catch(() => {
        if (active) setUser(null)
      })
      .finally(() => {
        if (active) setReady(true)
      })
    return () => {
      active = false
    }
  }, [])

  const value = useMemo<AuthState>(
    () => ({
      user,
      ready,
      async refresh() {
        const result = await api<{ user: User }>('/api/v1/me')
        setUser(result.user)
        return result.user
      },
      async signInWithGoogle(idToken: string) {
        const result = await api<{ user: User }>('/api/v1/auth/google', {
          method: 'POST',
          body: JSON.stringify({ idToken }),
        })
        setUser(result.user)
        return result.user
      },
      async updateProfile(body) {
        const result = await api<{ user: User }>('/api/v1/me', {
          method: 'PATCH',
          body: JSON.stringify(body),
        })
        setUser(result.user)
        return result.user
      },
      async signOut() {
        await api<void>('/api/v1/auth/logout', { method: 'POST' })
        setUser(null)
      },
    }),
    [ready, user],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const value = useContext(AuthContext)
  if (!value) {
    throw new Error('useAuth must be used inside AuthProvider')
  }
  return value
}

export { homeFor }
