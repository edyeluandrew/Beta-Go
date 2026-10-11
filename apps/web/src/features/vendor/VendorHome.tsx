import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { ApiError } from '../../shared/api/client'
import { prettyStatus, ugx, weekdayName } from '../../shared/ui/money'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Notice } from '../../shared/ui/widgets'
import { getPool, getWorkspace, listKitchen } from './api'
import type { KitchenOrder, PoolAccount, VendorWorkspace } from './types'

export function VendorHome() {
  const [shop, setShop] = useState<VendorWorkspace | null>(null)
  const [orders, setOrders] = useState<KitchenOrder[]>([])
  const [pool, setPool] = useState<PoolAccount | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true
    Promise.allSettled([getWorkspace(), listKitchen(), getPool()]).then(([workspace, kitchen, money]) => {
      if (!active) return
      if (workspace.status === 'rejected') {
        const err = workspace.reason
        setError(err instanceof ApiError ? err.message : 'Your shop could not be loaded.')
        return
      }
      setShop(workspace.value)
      if (kitchen.status === 'fulfilled') setOrders(kitchen.value.orders)
      if (money.status === 'fulfilled') setPool(money.value.account)
    })
    return () => {
      active = false
    }
  }, [])

  if (!shop && !error) {
    return (
      <Shell title="Your shop" lede="Loading…">
        <Notice>Loading…</Notice>
      </Shell>
    )
  }
  if (!shop) {
    return (
      <Shell title="Your shop" lede="Accept an invite first if you do not have a kitchen yet.">
        <ErrorText>{error}</ErrorText>
      </Shell>
    )
  }

  const now = kampalaNow()
  const today = shop.hours.find((day) => day.weekday === now.weekday)
  const waiting = orders.filter((order) => order.status === 'AWAITING_VENDOR')
  const showing = shop.items.filter((item) => item.available).length
  const headline = !shop.vendor.isOpen ? 'Closed' : shop.vendor.acceptingOrders ? 'Open and taking orders' : 'Open, but not taking new orders'
  const todayLine = !today || today.closed ? 'Closed today' : `${weekdayName(today.weekday)}, ${today.opensAt} to ${today.closesAt}`

  return (
    <Shell title={shop.vendor.businessName} lede="This is the state of the kitchen right now.">
      <section className="mt-6 rounded-[20px] border border-line bg-card p-6">
        <p className="text-sm text-muted">Right now</p>
        <h2 className="mt-1 text-3xl">{headline}</h2>
        <p className="mt-2 text-muted">{todayLine}</p>
        <p className="mt-1 text-sm text-muted">{shop.vendor.address}</p>
      </section>

      <section className="mt-4 grid gap-3 sm:grid-cols-3">
        <div className="rounded-[20px] border border-line bg-card p-5">
          <p className="text-sm text-muted">Waiting for you</p>
          <p className="mt-2 font-display text-4xl">{waiting.length}</p>
        </div>
        <div className="rounded-[20px] border border-line bg-card p-5">
          <p className="text-sm text-muted">Dishes showing</p>
          <p className="mt-2 font-display text-4xl">
            {showing}
            <span className="text-lg text-muted"> / {shop.items.length}</span>
          </p>
        </div>
        <div className="rounded-[20px] border border-line bg-card p-5">
          <p className="text-sm text-muted">Pool available</p>
          <p className="mt-2 font-display text-3xl">{pool ? ugx(pool.availableUgx) : '—'}</p>
        </div>
      </section>

      {pool?.lowBalance ? (
        <p className="mt-4 text-sm text-muted">
          The pool is low. Top it up on <Link className="text-forest" to="/v/pool">Pool</Link> before the shop stops taking orders.
        </p>
      ) : null}

      <section className="mt-6">
        <h2 className="text-2xl">Orders waiting</h2>
        {waiting.length === 0 ? (
          <p className="mt-3 text-sm text-muted">No paid order is waiting. New ones show up here.</p>
        ) : (
          <ul className="mt-3 grid gap-2.5">
            {waiting.map((order) => (
              <li key={order.id}>
                <Link className="block rounded-2xl border border-line bg-card p-4 text-ink no-underline" to={`/v/orders/${order.id}`}>
                  <strong className="capitalize">{prettyStatus(order.status)}</strong>
                  <p className="mt-1 text-sm text-muted">
                    {ugx(order.totalUgx)} · {order.fulfilment}
                  </p>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </section>
      <ErrorText>{error}</ErrorText>
    </Shell>
  )
}

function kampalaNow() {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: 'Africa/Kampala',
    weekday: 'short',
  }).formatToParts(new Date())
  const weekday = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'].indexOf(parts.find((part) => part.type === 'weekday')?.value ?? '')
  return { weekday }
}
