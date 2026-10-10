import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { ApiError } from '../../shared/api/client'
import { useAuth } from '../../shared/auth/AuthProvider'
import { ugx } from '../../shared/ui/money'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Notice } from '../../shared/ui/widgets'
import { listVendors } from './api'
import type { PublicVendor } from './types'

export function CustomerHome() {
  const { user } = useAuth()
  const [vendors, setVendors] = useState<PublicVendor[] | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true
    listVendors()
      .then((result) => {
        if (active) setVendors(result.vendors)
      })
      .catch((err: unknown) => {
        if (active) setError(err instanceof ApiError ? err.message : 'Kitchens could not be loaded.')
      })
    return () => {
      active = false
    }
  }, [])

  return (
    <Shell
      title="Kitchens in Kabale"
      lede={
        user
          ? 'One kitchen per order. Prices come from the server.'
          : 'Look through the menus first. Create an account only when you are ready to place an order.'
      }
    >
      <ErrorText>{error}</ErrorText>
      {vendors === null && !error && <Notice>Loading kitchens…</Notice>}
      {vendors?.length === 0 && <Notice>No kitchen is taking orders right now. An admin has to approve a shop first.</Notice>}
      {vendors && vendors.length > 0 && (
        <ul className="mt-5 grid gap-3">
          {vendors.map((vendor) => (
            <li key={vendor.id}>
              <Link className="block overflow-hidden rounded-2xl border border-line bg-card text-ink no-underline" to={`/c/shops/${vendor.id}`}>
                {vendor.shopPhotoUrl ? (
                  <img src={vendor.shopPhotoUrl} alt="" referrerPolicy="no-referrer" className="h-40 w-full object-cover" />
                ) : (
                  <div className="h-40 w-full bg-line" />
                )}
                <div className="p-4">
                  <strong>{vendor.businessName}</strong>
                  <p className="mt-1 text-muted">{vendor.description || vendor.address}</p>
                  <p className="mt-1 text-sm text-muted">
                    {vendor.acceptingOrders ? 'Accepting orders' : 'Busy'}
                    {vendor.deliveryFeeUgx > 0 ? ` · Delivery ${ugx(vendor.deliveryFeeUgx)}` : ' · Collection'}
                  </p>
                  {vendor.dishes && vendor.dishes.length > 0 && (
                    <ul className="mt-3 grid grid-cols-5 gap-2">
                      {vendor.dishes.map((dish) => (
                        <li key={dish.id} className="overflow-hidden rounded-xl">
                          <img src={dish.photoUrl} alt={dish.name} referrerPolicy="no-referrer" className="h-20 w-full object-cover" />
                          <p className="mt-1 truncate text-xs text-muted">{dish.name}</p>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Shell>
  )
}
