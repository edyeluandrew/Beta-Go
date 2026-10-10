import { useEffect, useState } from 'react'
import { ApiError } from '../../shared/api/client'
import { ugx, weekdayName } from '../../shared/ui/money'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Field, GhostButton, Notice, PrimaryButton, inputClass } from '../../shared/ui/widgets'
import { addCategory, addItem, getWorkspace, saveHours, updateItem, updateShop } from './api'
import type { OpeningHour, VendorWorkspace } from './types'

const emptyHours = (): OpeningHour[] =>
  [0, 1, 2, 3, 4, 5, 6].map((weekday) => ({
    weekday,
    opensAt: '08:00',
    closesAt: '21:00',
    closed: weekday === 0,
  }))

export function VendorHome() {
  const [shop, setShop] = useState<VendorWorkspace | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [description, setDescription] = useState('')
  const [photo, setPhoto] = useState('')
  const [logo, setLogo] = useState('')
  const [fee, setFee] = useState('0')
  const [isOpen, setIsOpen] = useState(true)
  const [accepting, setAccepting] = useState(true)
  const [hours, setHours] = useState<OpeningHour[]>(emptyHours())
  const [categoryName, setCategoryName] = useState('')
  const [itemName, setItemName] = useState('')
  const [itemDesc, setItemDesc] = useState('')
  const [itemPrice, setItemPrice] = useState('')
  const [itemPhoto, setItemPhoto] = useState('')
  const [categoryId, setCategoryId] = useState('')

  const load = async () => {
    const result = await getWorkspace()
    setShop(result)
    setDescription(result.vendor.description)
    setPhoto(result.vendor.shopPhotoUrl)
    setLogo(result.vendor.logoUrl)
    setFee(String(result.vendor.deliveryFeeUgx))
    setIsOpen(result.vendor.isOpen)
    setAccepting(result.vendor.acceptingOrders)
    setHours(result.hours.length === 7 ? result.hours : emptyHours())
    if (!categoryId && result.categories[0]) setCategoryId(result.categories[0].id)
  }

  useEffect(() => {
    let active = true
    load().catch((err: unknown) => {
      if (active) setError(err instanceof ApiError ? err.message : 'Your shop could not be loaded.')
    })
    return () => {
      active = false
    }
  }, [])

  const run = async (action: () => Promise<void>) => {
    setBusy(true)
    setError('')
    try {
      await action()
      await load()
    } catch (err: unknown) {
      setError(err instanceof ApiError ? err.message : 'That change did not finish.')
    } finally {
      setBusy(false)
    }
  }

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

  return (
    <Shell title={shop.vendor.businessName} lede={`${shop.vendor.address}. Status ${shop.vendor.status.replaceAll('_', ' ')}.`}>
      <form
        className="mt-6 grid gap-3 rounded-[20px] border border-line bg-card p-6"
        onSubmit={(event) => {
          event.preventDefault()
          void run(async () => {
            await updateShop({
              description,
              logoUrl: logo,
              shopPhotoUrl: photo,
              deliveryFeeUgx: Number(fee) || 0,
              isOpen,
              acceptingOrders: accepting,
            })
          })
        }}
      >
        <Field label="Description">
          <textarea className={inputClass} value={description} onChange={(event) => setDescription(event.target.value)} />
        </Field>
        <Field label="Shop photo URL">
          <input className={inputClass} value={photo} onChange={(event) => setPhoto(event.target.value)} />
        </Field>
        <Field label="Logo URL">
          <input className={inputClass} value={logo} onChange={(event) => setLogo(event.target.value)} />
        </Field>
        <Field label="Vendor delivery fee (UGX)">
          <input className={inputClass} type="number" min="0" value={fee} onChange={(event) => setFee(event.target.value)} />
        </Field>
        <label className="flex items-center gap-2 text-sm text-muted">
          <input type="checkbox" checked={isOpen} onChange={(event) => setIsOpen(event.target.checked)} />
          Shop is open
        </label>
        <label className="flex items-center gap-2 text-sm text-muted">
          <input type="checkbox" checked={accepting} onChange={(event) => setAccepting(event.target.checked)} />
          Accepting new orders
        </label>
        <PrimaryButton type="submit" disabled={busy}>
          Save shop
        </PrimaryButton>
      </form>

      <form
        className="mt-6 grid gap-3 rounded-[20px] border border-line bg-card p-6"
        onSubmit={(event) => {
          event.preventDefault()
          void run(async () => {
            await saveHours(hours)
          })
        }}
      >
        <h2 className="text-2xl">Hours</h2>
        {hours.map((day, index) => (
          <div key={day.weekday} className="grid items-center gap-2 sm:grid-cols-[7rem_1fr_1fr_auto]">
            <span className="text-sm text-muted">{weekdayName(day.weekday)}</span>
            <input
              className={inputClass}
              type="time"
              value={day.opensAt}
              disabled={day.closed}
              onChange={(event) => setHours((current) => current.map((row, rowIndex) => (rowIndex === index ? { ...row, opensAt: event.target.value } : row)))}
            />
            <input
              className={inputClass}
              type="time"
              value={day.closesAt}
              disabled={day.closed}
              onChange={(event) => setHours((current) => current.map((row, rowIndex) => (rowIndex === index ? { ...row, closesAt: event.target.value } : row)))}
            />
            <label className="text-sm text-muted">
              <input
                type="checkbox"
                checked={day.closed}
                onChange={(event) => setHours((current) => current.map((row, rowIndex) => (rowIndex === index ? { ...row, closed: event.target.checked } : row)))}
              />{' '}
              Closed
            </label>
          </div>
        ))}
        <PrimaryButton type="submit" disabled={busy}>
          Save hours
        </PrimaryButton>
      </form>

      <section className="mt-6 rounded-[20px] border border-line bg-card p-6">
        <h2 className="text-2xl">Menu</h2>
        <form
          className="mt-3 grid gap-3"
          onSubmit={(event) => {
            event.preventDefault()
            void run(async () => {
              await addCategory(categoryName)
              setCategoryName('')
            })
          }}
        >
          <Field label="New category">
            <input className={inputClass} value={categoryName} onChange={(event) => setCategoryName(event.target.value)} />
          </Field>
          <PrimaryButton type="submit" disabled={busy || !categoryName.trim()}>
            Add category
          </PrimaryButton>
        </form>
        <form
          className="mt-4 grid gap-3"
          onSubmit={(event) => {
            event.preventDefault()
            void run(async () => {
              await addItem({
                name: itemName,
                description: itemDesc,
                priceUgx: Number(itemPrice),
                categoryId: categoryId || undefined,
                photoUrl: itemPhoto,
              })
              setItemName('')
              setItemDesc('')
              setItemPrice('')
              setItemPhoto('')
            })
          }}
        >
          {shop.categories.length > 0 && (
            <Field label="Category">
              <select className={inputClass} value={categoryId} onChange={(event) => setCategoryId(event.target.value)}>
                {shop.categories.map((category) => (
                  <option key={category.id} value={category.id}>
                    {category.name}
                  </option>
                ))}
              </select>
            </Field>
          )}
          <Field label="Item name">
            <input className={inputClass} value={itemName} onChange={(event) => setItemName(event.target.value)} required />
          </Field>
          <Field label="Description">
            <input className={inputClass} value={itemDesc} onChange={(event) => setItemDesc(event.target.value)} />
          </Field>
          <Field label="Price (UGX)">
            <input className={inputClass} type="number" min="1" value={itemPrice} onChange={(event) => setItemPrice(event.target.value)} required />
          </Field>
          <Field label="Food photo URL">
            <input className={inputClass} value={itemPhoto} onChange={(event) => setItemPhoto(event.target.value)} placeholder="Clear photo of the dish" />
          </Field>
          <PrimaryButton type="submit" disabled={busy}>
            Add item
          </PrimaryButton>
        </form>
        <ul className="mt-5 grid gap-3 sm:grid-cols-2">
          {shop.items.map((item) => (
            <li key={item.id} className="overflow-hidden rounded-2xl border border-line">
              {item.photoUrl ? (
                <img src={item.photoUrl} alt={item.name} referrerPolicy="no-referrer" className="h-36 w-full object-cover" />
              ) : (
                <div className="h-36 w-full bg-line" />
              )}
              <div className="flex items-center justify-between gap-3 p-4">
              <div>
                <strong>{item.name}</strong>
                <p className="mt-1 text-sm text-muted">
                  {ugx(item.priceUgx)} · {item.available ? 'On the menu' : 'Hidden'}
                </p>
              </div>
              <GhostButton
                type="button"
                disabled={busy}
                onClick={() =>
                  void run(async () => {
                    await updateItem(item.id, {
                      name: item.name,
                      description: item.description,
                      priceUgx: item.priceUgx,
                      available: !item.available,
                      photoUrl: item.photoUrl,
                    })
                  })
                }
              >
                {item.available ? 'Hide' : 'Show'}
              </GhostButton>
              </div>
            </li>
          ))}
        </ul>
      </section>
      <ErrorText>{error}</ErrorText>
    </Shell>
  )
}
