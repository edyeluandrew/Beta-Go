import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { ApiError } from '../../shared/api/client'
import { prettyStatus, ugx } from '../../shared/ui/money'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Notice } from '../../shared/ui/widgets'
import { listKitchen } from './api'
import type { KitchenOrder } from './types'

export function VendorOrdersPage() {
  const [orders, setOrders] = useState<KitchenOrder[] | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true
    listKitchen()
      .then((result) => {
        if (active) setOrders(result.orders)
      })
      .catch((err: unknown) => {
        if (active) setError(err instanceof ApiError ? err.message : 'Kitchen orders could not be loaded.')
      })
    return () => {
      active = false
    }
  }, [])

  return (
    <Shell title="Kitchen" lede="Paid orders wait here. Accepting is a separate step from the customer paying.">
      <ErrorText>{error}</ErrorText>
      {orders === null && !error && <Notice>Loading orders…</Notice>}
      {orders?.length === 0 && <Notice>No kitchen orders yet.</Notice>}
      {orders && orders.length > 0 && (
        <ul className="mt-5 grid gap-2.5">
          {orders.map((order) => (
            <li key={order.id}>
              <Link className="block rounded-2xl border border-line bg-card p-4 text-ink no-underline" to={`/v/orders/${order.id}`}>
                <strong className="capitalize">{prettyStatus(order.status)}</strong>
                <p className="mt-1 text-sm text-muted">
                  {ugx(order.totalUgx)} · {order.fulfilment}
                  {order.cancelRequested ? ' · cancel requested' : ''}
                </p>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Shell>
  )
}
