import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { ApiError } from '../../shared/api/client'
import { prettyStatus, ugx } from '../../shared/ui/money'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Notice } from '../../shared/ui/widgets'
import { listOrders } from './api'
import type { CustomerOrder } from './types'

export function OrdersPage() {
  const [orders, setOrders] = useState<CustomerOrder[] | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true
    listOrders()
      .then((result) => {
        if (active) setOrders(result.orders)
      })
      .catch((err: unknown) => {
        if (active) setError(err instanceof ApiError ? err.message : 'Orders could not be loaded.')
      })
    return () => {
      active = false
    }
  }, [])

  return (
    <Shell title="Your orders" lede="Payment, kitchen acceptance, and completion stay as separate steps.">
      <ErrorText>{error}</ErrorText>
      {orders === null && !error && <Notice>Loading orders…</Notice>}
      {orders?.length === 0 && <Notice>No orders yet. Start from the marketplace.</Notice>}
      {orders && orders.length > 0 && (
        <ul className="mt-5 grid gap-2.5">
          {orders.map((order) => (
            <li key={order.id}>
              <Link className="block rounded-2xl border border-line bg-card p-4 text-ink no-underline" to={`/c/orders/${order.id}`}>
                <strong className="capitalize">{prettyStatus(order.status)}</strong>
                <p className="mt-1 text-muted">{order.items.map((item) => `${item.quantity}× ${item.name}`).join(', ')}</p>
                <p className="mt-1 text-sm text-muted">{ugx(order.totalUgx)}</p>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Shell>
  )
}
