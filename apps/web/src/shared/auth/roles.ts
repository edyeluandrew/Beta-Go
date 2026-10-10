export type User = {
  id: string
  email: string
  name: string
  pictureUrl: string
  phone?: string
  area?: string
  roles: string[]
}

export const staffRoles = ['support', 'finance', 'super_admin']

export function isStaff(roles: string[]) {
  return roles.some((role) => staffRoles.includes(role))
}

export const canManageVendors = (roles: string[]) => hasRole(roles, ['support', 'super_admin'])
export const canTouchMoney = (roles: string[]) => hasRole(roles, ['finance', 'super_admin'])
export const isSuperAdmin = (roles: string[]) => roles.includes('super_admin')

export function hasRole(roles: string[], wanted: string[]) {
  return wanted.some((role) => roles.includes(role))
}

export function homeFor(user: User) {
  if (isStaff(user.roles)) return '/a'
  if (user.roles.includes('vendor')) return '/v'
  if (user.roles.includes('customer')) return '/c/account'
  return '/'
}
