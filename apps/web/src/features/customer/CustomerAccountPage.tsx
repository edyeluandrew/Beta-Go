import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { ApiError } from '../../shared/api/client'
import { useAuth } from '../../shared/auth/AuthProvider'
import { prettyStatus, ugx } from '../../shared/ui/money'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Field, Notice, PrimaryButton, inputClass } from '../../shared/ui/widgets'
import { listOrders, listVendors } from './api'
import type { CustomerOrder, PublicVendor } from './types'

export function CustomerAccountPage() {
  const { user, updateProfile } = useAuth()
  const [orders, setOrders] = useState<CustomerOrder[] | null>(null)
  const [vendors, setVendors] = useState<PublicVendor[]>([])
  const [name, setName] = useState(user?.name ?? '')
  const [phone, setPhone] = useState(user?.phone ?? '')
  const [area, setArea] = useState(user?.area ?? '')
  const [error, setError] = useState('')
  const [saved, setSaved] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    setName(user?.name ?? '')
    setPhone(user?.phone ?? '')
    setArea(user?.area ?? '')
  }, [user])

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
    <Shell title={`Hi ${user.name || 'there'}`} lede="This is your customer dashboard. Browse kitchens any time. An account is only required to place and follow an order.">
      {(!user.phone || !user.area) && (
        <Notice>Add a phone and area so a kitchen can reach you when you order.</Notice>
      )}
      <section className="mt-6 grid gap-3 rounded-[20px] border border-line bg-card p-6">
        <h2 className="text-2xl">Your details</h2>
        <p className="text-sm text-muted">{user.email}</p>
        <form
          className="grid gap-3"
          onSubmit={(event) => {
            event.preventDefault()
            setBusy(true)
            setError('')
            setSaved('')
            void updateProfile({ name: name.trim(), phone: phone.trim(), area: area.trim() })
              .then(() => setSaved('Saved.'))
              .catch((err: unknown) => setError(err instanceof ApiError ? err.message : 'Those details did not save.'))
              .finally(() => setBusy(false))
          }}
        >
          <Field label="Name">
            <input className={inputClass} value={name} onChange={(event) => setName(event.target.value)} required />
          </Field>
          <Field label="Phone">
            <input className={inputClass} value={phone} onChange={(event) => setPhone(event.target.value)} required />
          </Field>
          <Field label="Area in Kabale">
            <input className={inputClass} value={area} onChange={(event) => setArea(event.target.value)} required />
          </Field>
          <PrimaryButton type="submit" disabled={busy}>
            {busy ? 'Saving…' : 'Save details'}
          </PrimaryButton>
          {saved ? <p className="text-sm text-forest">{saved}</p> : null}
        </form>
      </section>

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
