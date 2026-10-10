import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { readCart, saveCart, saveNextPath } from '../auth/afterSignIn'
import { ApiError } from '../../shared/api/client'
import { useAuth } from '../../shared/auth/AuthProvider'
import { ugx, weekdayName } from '../../shared/ui/money'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Field, Notice, PrimaryButton, inputClass } from '../../shared/ui/widgets'
import { createOrder, getShop } from './api'
import type { PublicShop } from './types'

export function ShopPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const [shop, setShop] = useState<PublicShop | null>(null)
  const [qty, setQty] = useState<Record<string, number>>({})
  const [fulfilment, setFulfilment] = useState<'collection' | 'delivery'>('collection')
  const [address, setAddress] = useState('')
  const [phone, setPhone] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let active = true
    getShop(id)
      .then((result) => {
        if (active) setShop(result)
      })
      .catch((err: unknown) => {
        if (active) setError(err instanceof ApiError ? err.message : 'This kitchen could not be loaded.')
      })
    return () => {
      active = false
    }
  }, [id])

  useEffect(() => {
    if (!id) return
    const saved = readCart(id)
    if (Object.keys(saved).length > 0) setQty(saved)
  }, [id])

  useEffect(() => {
    if (user?.phone && !phone) setPhone(user.phone)
    if (user?.area && !address) setAddress(user.area)
  }, [address, phone, user])

  const lines = useMemo(
    () =>
      Object.entries(qty)
        .filter(([, quantity]) => quantity > 0)
        .map(([itemId, quantity]) => ({ itemId, quantity })),
    [qty],
  )
  const foodTotal = useMemo(() => {
    if (!shop) return 0
    return lines.reduce((sum, line) => {
      const item = shop.items.find((candidate) => candidate.id === line.itemId)
      return sum + (item ? item.priceUgx * line.quantity : 0)
    }, 0)
  }, [lines, shop])
  const deliveryFee = fulfilment === 'delivery' && shop ? shop.vendor.deliveryFeeUgx : 0

  const placeOrder = async () => {
    if (!shop || lines.length === 0) return
    if (!user) {
      saveCart(shop.vendor.id, qty)
      saveNextPath(`/c/shops/${shop.vendor.id}`)
      navigate(`/join?next=/c/shops/${shop.vendor.id}`)
      return
    }
    setBusy(true)
    setError('')
    try {
      const result = await createOrder({
        idempotencyKey: crypto.randomUUID(),
        vendorId: shop.vendor.id,
        fulfilment,
        address: fulfilment === 'delivery' ? address : '',
        phone: fulfilment === 'delivery' ? phone : '',
        items: lines,
      })
      navigate(`/c/orders/${result.order.id}`)
    } catch (err: unknown) {
      setError(err instanceof ApiError ? err.message : 'The order did not go through.')
    } finally {
      setBusy(false)
    }
  }

  if (!shop && !error) {
    return (
      <Shell title="Kitchen" lede="Loading the menu…">
        <Notice>Loading…</Notice>
      </Shell>
    )
  }
  if (!shop) {
    return (
      <Shell title="Kitchen" lede="This shop is not on the public list.">
        <ErrorText>{error}</ErrorText>
      </Shell>
    )
  }

  return (
    <Shell title={shop.vendor.businessName} lede={shop.vendor.description || shop.vendor.address}>
      {shop.vendor.shopPhotoUrl ? (
        <img
          src={shop.vendor.shopPhotoUrl}
          alt=""
          referrerPolicy="no-referrer"
          className="mt-5 h-48 w-full rounded-[20px] object-cover"
        />
      ) : null}
      <p className="mt-2 text-sm text-muted">{shop.vendor.address}</p>
      {shop.hours.length > 0 && (
        <ul className="mt-4 grid grid-cols-[repeat(auto-fit,minmax(140px,1fr))] gap-2 text-sm text-muted">
          {shop.hours.map((day) => (
            <li key={day.weekday}>
              {weekdayName(day.weekday)}
              {day.closed ? ' closed' : ` ${day.opensAt}–${day.closesAt}`}
            </li>
          ))}
        </ul>
      )}
      <ul className="mt-5 grid gap-3 sm:grid-cols-2">
        {[...shop.items]
          .sort((left, right) => Number(Boolean(right.photoUrl)) - Number(Boolean(left.photoUrl)))
          .map((item) => (
          <li key={item.id} className="overflow-hidden rounded-2xl border border-line bg-card">
            {item.photoUrl ? (
              <img src={item.photoUrl} alt={item.name} referrerPolicy="no-referrer" className="h-52 w-full object-cover" />
            ) : (
              <div className="flex h-52 w-full items-center justify-center bg-line text-sm text-muted">Photo coming</div>
            )}
            <div className="flex gap-3 p-4">
            <div>
              <strong>{item.name}</strong>
              <p className="mt-1 text-muted">{item.description}</p>
              <p className="mt-1 text-sm text-muted">{ugx(item.priceUgx)}</p>
            </div>
            {item.available ? (
              <div className="ml-auto flex items-center gap-2">
                <PrimaryButton type="button" className="min-w-8 px-2.5 py-1.5" onClick={() => setQty((current) => ({ ...current, [item.id]: Math.max(0, (current[item.id] ?? 0) - 1) }))}>
                  −
                </PrimaryButton>
                <span>{qty[item.id] ?? 0}</span>
                <PrimaryButton type="button" className="min-w-8 px-2.5 py-1.5" onClick={() => setQty((current) => ({ ...current, [item.id]: (current[item.id] ?? 0) + 1 }))}>
                  +
                </PrimaryButton>
              </div>
            ) : (
              <p className="ml-auto text-sm text-muted">Unavailable</p>
            )}
            </div>
          </li>
        ))}
      </ul>
      <form
        className="mt-6 grid w-full max-w-[440px] gap-3 rounded-[20px] border border-line bg-card p-6"
        onSubmit={(event) => {
          event.preventDefault()
          void placeOrder()
        }}
      >
        <Field label="How you get the food">
          <select className={inputClass} value={fulfilment} onChange={(event) => setFulfilment(event.target.value as 'collection' | 'delivery')}>
            <option value="collection">Collect it</option>
            <option value="delivery">Vendor delivery</option>
          </select>
        </Field>
        {fulfilment === 'delivery' && (
          <>
            <Field label="Delivery address">
              <input className={inputClass} value={address} onChange={(event) => setAddress(event.target.value)} required />
            </Field>
            <Field label="Phone">
              <input className={inputClass} value={phone} onChange={(event) => setPhone(event.target.value)} required />
            </Field>
          </>
        )}
        <p>
          Food {ugx(foodTotal)}
          {deliveryFee > 0 ? ` + delivery ${ugx(deliveryFee)}` : ''}
        </p>
        <p className="text-sm text-muted">
          {user
            ? 'Minimum order is UGX 5,000. The server checks the price again.'
            : 'You can pick food now. Create an account on the next step to place the order.'}
        </p>
        <ErrorText>{error}</ErrorText>
        <PrimaryButton type="submit" disabled={busy || lines.length === 0}>
          {busy ? 'Placing…' : user ? 'Place order' : 'Create account to order'}
        </PrimaryButton>
      </form>
    </Shell>
  )
}
