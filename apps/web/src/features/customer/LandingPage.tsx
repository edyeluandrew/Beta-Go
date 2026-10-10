import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useAuth } from '../../shared/auth/AuthProvider'
import { homeFor } from '../../shared/auth/roles'
import { ugx } from '../../shared/ui/money'
import { listVendors } from './api'
import type { PublicVendor } from './types'

const steps = [
  { title: 'Look around', body: 'Open kitchens in Kabale. See chips, chicken, pork, burgers, pizza, and whatever else they are cooking today.' },
  { title: 'Create an account', body: 'When you are ready to order, tell us your name, phone, and area, then continue with Google.' },
  { title: 'Pay the kitchen', body: 'You pay that shop on MTN or Airtel. Beta-Go keeps the order moving from the kitchen to you.' },
]

export function LandingPage() {
  const { user } = useAuth()
  const [vendors, setVendors] = useState<PublicVendor[]>([])

  useEffect(() => {
    listVendors()
      .then((result) => setVendors(result.vendors))
      .catch(() => setVendors([]))
  }, [])

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-10 flex flex-wrap items-center gap-3 border-b border-line bg-card/90 px-5 py-4 backdrop-blur">
        <Link className="font-display text-[1.4rem] text-ink no-underline" to="/">
          Beta-Go
        </Link>
        <nav className="ml-auto flex flex-wrap items-center gap-2 text-sm">
          <Link className="rounded-full px-3 py-1.5 text-muted no-underline hover:bg-leaf hover:text-forest-dark" to="/c">
            Kitchens
          </Link>
          {user ? (
            <Link className="rounded-full bg-forest px-3 py-1.5 text-white no-underline" to={homeFor(user)}>
              Open dashboard
            </Link>
          ) : (
            <>
              <Link className="rounded-full px-3 py-1.5 text-muted no-underline hover:bg-leaf hover:text-forest-dark" to="/signin">
                Sign in
              </Link>
              <Link className="rounded-full bg-forest px-3 py-1.5 text-white no-underline" to="/join">
                Create account
              </Link>
            </>
          )}
        </nav>
      </header>

      <section className="mx-auto w-[min(960px,calc(100%-32px))] pb-16 pt-10">
        <p className="text-xs font-semibold uppercase tracking-[0.14em] text-forest">Kabale food marketplace</p>
        <h1 className="mt-3 max-w-3xl text-[3.2rem] leading-[1.05]">Order from kitchens already cooking chips, chicken, pork, burgers, and pizza.</h1>
        <p className="mt-4 max-w-2xl text-lg text-muted">
          You can walk the whole app first. Browse shops and menus with no account. Create one only when you want to place an order.
        </p>
        <div className="mt-6 flex flex-wrap gap-3">
          <Link className="rounded-full bg-forest px-5 py-2.5 text-white no-underline" to="/c">
            See kitchens
          </Link>
          <Link className="rounded-full border border-line bg-card px-5 py-2.5 text-ink no-underline" to="/join">
            Create an account
          </Link>
        </div>

        <ul className="mt-12 grid gap-3 md:grid-cols-3">
          {steps.map((step, index) => (
            <li key={step.title} className="rounded-[20px] border border-line bg-card p-5">
              <p className="text-xs font-semibold uppercase tracking-[0.14em] text-clay">Step {index + 1}</p>
              <h2 className="mt-2 text-2xl">{step.title}</h2>
              <p className="mt-2 text-muted">{step.body}</p>
            </li>
          ))}
        </ul>

        <h2 className="mt-14 text-3xl">Kitchens you can open now</h2>
        <p className="mt-2 text-muted">Photos and prices are live. No sign-in needed to look.</p>
        {vendors.length === 0 ? (
          <p className="mt-4 text-muted">No kitchen is taking orders right now. Come back shortly.</p>
        ) : (
          <ul className="mt-5 grid gap-4 md:grid-cols-2">
            {vendors.map((vendor) => (
              <li key={vendor.id}>
                <Link className="block overflow-hidden rounded-[20px] border border-line bg-card text-ink no-underline" to={`/c/shops/${vendor.id}`}>
                  {vendor.shopPhotoUrl ? (
                    <img src={vendor.shopPhotoUrl} alt="" referrerPolicy="no-referrer" className="h-44 w-full object-cover" />
                  ) : (
                    <div className="h-44 w-full bg-line" />
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
                            <img src={dish.photoUrl} alt={dish.name} referrerPolicy="no-referrer" className="h-16 w-full object-cover" />
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
      </section>
    </div>
  )
}
