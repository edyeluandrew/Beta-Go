import { api } from '../../shared/api/client'
import type { CustomerOrder, Payment, PublicShop, PublicVendor } from './types'

export function listVendors() {
  return api<{ vendors: PublicVendor[] }>('/api/v1/vendors')
}

export function getShop(id: string) {
  return api<PublicShop>(`/api/v1/vendors/${id}`)
}

export function listOrders() {
  return api<{ orders: CustomerOrder[] }>('/api/v1/orders')
}

export function getOrder(id: string) {
  return api<{ order: CustomerOrder }>(`/api/v1/orders/${id}`)
}

export function createOrder(body: {
  idempotencyKey: string
  vendorId: string
  fulfilment: 'collection' | 'delivery'
  address: string
  phone: string
  items: { itemId: string; quantity: number }[]
}) {
  return api<{ order: CustomerOrder }>('/api/v1/orders', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function cancelOrder(id: string) {
  return api<{ order: CustomerOrder }>(`/api/v1/orders/${id}/cancel`, { method: 'POST' })
}

export function confirmOrder(id: string) {
  return api<{ order: CustomerOrder }>(`/api/v1/orders/${id}/confirm`, { method: 'POST' })
}

export function openDispute(id: string, reason: string) {
  return api<{ order: CustomerOrder }>(`/api/v1/orders/${id}/disputes`, {
    method: 'POST',
    body: JSON.stringify({ reason }),
  })
}

export function startPayment(orderId: string, provider: 'mtn' | 'airtel', phone: string) {
  return api<{ payment: Payment }>(`/api/v1/orders/${orderId}/payment`, {
    method: 'POST',
    body: JSON.stringify({ provider, phone }),
  })
}

export function getPayment(orderId: string) {
  return api<{ payment: Payment }>(`/api/v1/orders/${orderId}/payment`)
}
