import { api } from '../../shared/api/client'
import type {
  Adjustment,
  AdminVendor,
  AuditEntry,
  BackupCheck,
  Destination,
  PoolAccount,
  SecurityReport,
} from './types'

export const listAdminVendors = () => api<{ vendors: AdminVendor[] }>('/api/v1/admin/vendors')

export const getAdminVendor = (id: string) =>
  api<{ vendor: AdminVendor; destination?: Destination; missing?: string[] }>(`/api/v1/admin/vendors/${id}`)

export const createVendor = (body: {
  businessName: string
  contactName: string
  contactEmail: string
  phone: string
  address: string
}) => api<{ vendor: AdminVendor }>('/api/v1/admin/vendors', { method: 'POST', body: JSON.stringify(body) })

export const inviteVendor = (id: string) =>
  api<{ token: string; expiresAt: string }>(`/api/v1/admin/vendors/${id}/invitations`, { method: 'POST' })

export const setDestination = (id: string, provider: string, account: string) =>
  api<{ destination: Destination }>(`/api/v1/admin/vendors/${id}/destination`, {
    method: 'POST',
    body: JSON.stringify({ provider, account }),
  })

export const verifyDestination = (id: string) =>
  api<{ destination: Destination }>(`/api/v1/admin/vendors/${id}/destination/verify`, { method: 'POST' })

export const vendorStatus = (id: string, action: string, reason = '') =>
  api<{ vendor: AdminVendor }>(`/api/v1/admin/vendors/${id}/${action}`, {
    method: 'POST',
    body: JSON.stringify({ reason }),
  })

export const getAdminPool = (id: string) => api<{ account: PoolAccount }>(`/api/v1/admin/vendors/${id}/commission`)

export const requestAdjustment = (id: string, amountUgx: number, reason: string) =>
  api<{ adjustment: Adjustment; account: PoolAccount }>(`/api/v1/admin/vendors/${id}/commission/adjustments`, {
    method: 'POST',
    body: JSON.stringify({ amountUgx, reason }),
  })

export const approveAdjustment = (id: string) =>
  api<{ adjustment: Adjustment; account: PoolAccount }>(`/api/v1/admin/commission/adjustments/${id}/approve`, {
    method: 'POST',
  })

export const reconcilePayments = () =>
  api<{ checked: number; paid: number; expired: number; refundPending: number; unchanged: number }>(
    '/api/v1/admin/payments/reconcile',
    { method: 'POST' },
  )

export const reconcileOrders = () => api<{ status: string }>('/api/v1/admin/orders/reconcile', { method: 'POST' })

export const resolveDispute = (orderId: string, outcome: string, reason: string) =>
  api<{ order: { id: string; status: string } }>(`/api/v1/admin/orders/${orderId}/disputes/resolve`, {
    method: 'POST',
    body: JSON.stringify({ outcome, reason }),
  })

export const launchSecurity = () => api<SecurityReport>('/api/v1/admin/launch/security')
export const launchAudit = () => api<{ items: AuditEntry[] }>('/api/v1/admin/launch/audit')
export const launchBackups = () => api<{ items: BackupCheck[] }>('/api/v1/admin/launch/backups')
export const recordBackup = (note: string) =>
  api<BackupCheck>('/api/v1/admin/launch/backups', { method: 'POST', body: JSON.stringify({ note }) })
export const grantStaff = (email: string, role: string) =>
  api<{ id: string; email: string; roles: string[] }>('/api/v1/admin/launch/staff', {
    method: 'POST',
    body: JSON.stringify({ email, role }),
  })
