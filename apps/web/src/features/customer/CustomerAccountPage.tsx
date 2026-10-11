import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { ShoppingBag, Store } from 'lucide-react'
import { ApiError } from '../../shared/api/client'
import { useAuth } from '../../shared/auth/AuthProvider'
import { prettyStatus, ugx } from '../../shared/ui/money'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Notice } from '../../shared/ui/widgets'
import { listOrders, listVendors } from './api'
import type { CustomerOrder, PublicVendor } from './types'

export function CustomerAccountPage() {
  const { user } = useAuth()
  const [orders, setOrders] = useState<CustomerOrder[] | null>(null)
  const [vendors, setVendors] = useState<PublicVendor[]>([])
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
    listVendors()
      .then((result) => {
        if (active) setVendors(result.vendors)
      })
      .catch(() => undefined)
    return () => {
      active = false
    }
  }, [])

  if (!user) return null

  return (
    <Shell title={`Hi ${user.name || 'there'}`} lede="Recent orders and kitchens that are cooking.">
      <div className="mt-6 grid gap-3 sm:grid-cols-2">
        <Link className="flex items-center gap-3 rounded-2xl border border-line bg-card p-4 text-ink no-underline" to="/c/orders">
          <span className="grid h-11 w-11 place-items-center rounded-full bg-leaf text-forest">
            <ShoppingBag aria-hidden className="h-5 w-5" />
          </span>
          <span>
            <span className="block font-semibold">Orders</span>
            <span className="text-sm text-muted">{orders === null ? 'Loading…' : orders.length === 0 ? 'None yet' : `${orders.length} placed`}</span>
          </span>
        </Link>
        <Link className="flex items-center gap-3 rounded-2xl border border-line bg-card p-4 text-ink no-underline" to="/c">
          <span className="grid h-11 w-11 place-items-center rounded-full bg-leaf text-forest">
            <Store aria-hidden className="h-5 w-5" />
          </span>
          <span>
            <span className="block font-semibold">Kitchens</span>
            <span className="text-sm text-muted">Browse what is cooking</span>
          </span>
        </Link>
      </div>

      <section className="mt-8">
        <div className="flex items-end justify-between gap-3">
          <h2 className="text-2xl">Recent orders</h2>
          <Link className="text-sm text-forest" to="/c/orders">
            All orders
          </Link>
        </div>
        {orders === null && <Notice>Loading orders…</Notice>}
        {orders?.length === 0 && <Notice>No orders yet. Open a kitchen when you are hungry.</Notice>}
        {orders && orders.length > 0 && (
          <ul className="mt-4 grid gap-2.5">
            {orders.slice(0, 3).map((order) => (
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
      </section>

      <section className="mt-8">
        <div className="flex items-end justify-between gap-3">
          <h2 className="text-2xl">Kitchens</h2>
          <Link className="text-sm text-forest" to="/c">
            See all
          </Link>
        </div>
        <ul className="mt-4 grid gap-3 sm:grid-cols-2">
          {vendors.slice(0, 4).map((vendor) => (
            <li key={vendor.id}>
              <Link className="block overflow-hidden rounded-2xl border border-line bg-card text-ink no-underline" to={`/c/shops/${vendor.id}`}>
                {vendor.shopPhotoUrl ? (
                  <img src={vendor.shopPhotoUrl} alt="" referrerPolicy="no-referrer" className="h-28 w-full object-cover" />
                ) : (
                  <div className="h-28 w-full bg-line" />
                )}
                <div className="p-3">
                  <strong>{vendor.businessName}</strong>
                  <p className="mt-1 text-sm text-muted">{vendor.acceptingOrders ? 'Accepting orders' : 'Busy'}</p>
                </div>
              </Link>
            </li>
          ))}
        </ul>
      </section>
      <ErrorText>{error}</ErrorText>
    </Shell>
  )
}
