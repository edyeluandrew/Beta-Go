import { api } from '../../shared/api/client'
import type { KitchenOrder, LedgerEntry, MenuItem, OpeningHour, PoolAccount, VendorShop, VendorWorkspace } from './types'

export const getWorkspace = () => api<VendorWorkspace>('/api/v1/vendor/shop')

export const updateShop = (body: {
  description: string
  logoUrl: string
  shopPhotoUrl: string
  deliveryFeeUgx: number
  isOpen: boolean
  acceptingOrders: boolean
}) => api<{ vendor: VendorShop }>('/api/v1/vendor/shop', { method: 'PATCH', body: JSON.stringify(body) })

export const saveHours = (hours: OpeningHour[]) =>
  api<void>('/api/v1/vendor/hours', { method: 'PUT', body: JSON.stringify({ hours }) })

export const addCategory = (name: string) =>
  api<{ category: { id: string; name: string } }>('/api/v1/vendor/categories', {
    method: 'POST',
    body: JSON.stringify({ name }),
  })

export const addItem = (body: { name: string; description: string; priceUgx: number; categoryId?: string; photoUrl?: string }) =>
  api<{ item: MenuItem }>('/api/v1/vendor/items', { method: 'POST', body: JSON.stringify(body) })

export const updateItem = (id: string, body: { name: string; description: string; priceUgx: number; available: boolean; photoUrl?: string }) =>
  api<{ item: MenuItem }>(`/api/v1/vendor/items/${id}`, { method: 'PATCH', body: JSON.stringify(body) })

export const acceptInvite = (token: string) =>
  api<{ accepted: boolean }>('/api/v1/vendor/invitations/accept', { method: 'POST', body: JSON.stringify({ token }) })

export const listKitchen = () => api<{ orders: KitchenOrder[] }>('/api/v1/vendor/orders')

export const getKitchenOrder = (id: string) => api<{ order: KitchenOrder }>(`/api/v1/vendor/orders/${id}`)

export const kitchenAction = (id: string, action: string, body?: Record<string, string>) =>
  api<{ order: KitchenOrder }>(`/api/v1/vendor/orders/${id}/${action}`, {
    method: 'POST',
    body: body ? JSON.stringify(body) : undefined,
  })

export const getPool = () => api<{ account: PoolAccount }>('/api/v1/vendor/commission')

export const getLedger = () => api<{ entries: LedgerEntry[] }>('/api/v1/vendor/commission/ledger')

export const startTopUp = (body: { amountUgx: number; provider: string; phone: string; idempotencyKey: string }) =>
  api<{ topUp: { id: string; status: string; amountUgx: number } }>('/api/v1/vendor/commission/top-ups', {
    method: 'POST',
    body: JSON.stringify(body),
  })
