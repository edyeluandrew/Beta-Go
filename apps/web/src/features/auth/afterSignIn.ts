import { homeFor, type User } from '../../shared/auth/roles'

const JOIN_KEY = 'bg_join_profile'
const NEXT_KEY = 'bg_next'

export type JoinProfile = {
  name: string
  phone: string
  area: string
}

export function saveJoinProfile(profile: JoinProfile) {
  sessionStorage.setItem(JOIN_KEY, JSON.stringify(profile))
}

export function readJoinProfile(): JoinProfile | null {
  const raw = sessionStorage.getItem(JOIN_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw) as JoinProfile
  } catch {
    return null
  }
}

export function clearJoinProfile() {
  sessionStorage.removeItem(JOIN_KEY)
}

export function saveNextPath(path: string) {
  if (path.startsWith('/') && !path.startsWith('//')) sessionStorage.setItem(NEXT_KEY, path)
}

export function takeNextPath() {
  const path = sessionStorage.getItem(NEXT_KEY)
  sessionStorage.removeItem(NEXT_KEY)
  return path && path.startsWith('/') && !path.startsWith('//') ? path : ''
}

export function destinationFor(user: User) {
  return takeNextPath() || homeFor(user)
}

export const cartKey = (shopId: string) => `bg_cart_${shopId}`

export function saveCart(shopId: string, qty: Record<string, number>) {
  sessionStorage.setItem(cartKey(shopId), JSON.stringify(qty))
}

export function readCart(shopId: string): Record<string, number> {
  const raw = sessionStorage.getItem(cartKey(shopId))
  if (!raw) return {}
  try {
    return JSON.parse(raw) as Record<string, number>
  } catch {
    return {}
  }
}
