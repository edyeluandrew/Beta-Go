import { useEffect, useState } from 'react'
import { ApiError } from '../../shared/api/client'
import { ugx } from '../../shared/ui/money'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Field, GhostButton, Notice, PrimaryButton, inputClass } from '../../shared/ui/widgets'
import { addCategory, addItem, getWorkspace, updateItem } from './api'
import type { VendorWorkspace } from './types'

export function VendorMenuPage() {
  const [shop, setShop] = useState<VendorWorkspace | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [categoryName, setCategoryName] = useState('')
  const [itemName, setItemName] = useState('')
  const [itemDesc, setItemDesc] = useState('')
  const [itemPrice, setItemPrice] = useState('')
  const [itemPhoto, setItemPhoto] = useState('')
  const [categoryId, setCategoryId] = useState('')

  const load = async () => {
    const result = await getWorkspace()
    setShop(result)
    setCategoryId((current) => current || result.categories[0]?.id || '')
  }

  useEffect(() => {
    let active = true
    load().catch((err: unknown) => {
      if (active) setError(err instanceof ApiError ? err.message : 'The menu could not be loaded.')
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
      <Shell title="Menu" lede="Loading…">
        <Notice>Loading…</Notice>
      </Shell>
    )
  }
  if (!shop) {
    return (
      <Shell title="Menu" lede="Accept an invite first if you do not have a kitchen yet.">
        <ErrorText>{error}</ErrorText>
      </Shell>
    )
  }

  return (
    <Shell title="Menu" lede="Dishes customers can order. Hide one when it is sold out.">
      <section className="mt-6 rounded-[20px] border border-line bg-card p-6">
        <form
          className="grid gap-3"
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
          className="mt-6 grid gap-3"
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
      </section>

      <ul className="mt-5 grid gap-3 sm:grid-cols-2">
        {shop.items.map((item) => (
          <li key={item.id} className="overflow-hidden rounded-2xl border border-line bg-card">
            {item.photoUrl ? (
              <img src={item.photoUrl} alt={item.name} referrerPolicy="no-referrer" className="h-36 w-full object-cover" />
            ) : (
              <div className="h-36 w-full bg-line" />
            )}
            <div className="flex items-center justify-between gap-3 p-4">
              <div>
                <strong>{item.name}</strong>
                <p className="mt-1 text-sm text-muted">
                  {ugx(item.priceUgx)} · {item.available ? 'Showing' : 'Hidden'}
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
      <ErrorText>{error}</ErrorText>
    </Shell>
  )
}
