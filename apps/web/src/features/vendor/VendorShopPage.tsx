import { useEffect, useState } from 'react'
import { ApiError } from '../../shared/api/client'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Field, Notice, PrimaryButton, inputClass } from '../../shared/ui/widgets'
import { getWorkspace, updateShop } from './api'
import type { VendorWorkspace } from './types'

export function VendorShopPage() {
  const [shop, setShop] = useState<VendorWorkspace | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [description, setDescription] = useState('')
  const [photo, setPhoto] = useState('')
  const [logo, setLogo] = useState('')
  const [fee, setFee] = useState('0')
  const [isOpen, setIsOpen] = useState(true)
  const [accepting, setAccepting] = useState(true)

  const load = async () => {
    const result = await getWorkspace()
    setShop(result)
    setDescription(result.vendor.description)
    setPhoto(result.vendor.shopPhotoUrl)
    setLogo(result.vendor.logoUrl)
    setFee(String(result.vendor.deliveryFeeUgx))
    setIsOpen(result.vendor.isOpen)
    setAccepting(result.vendor.acceptingOrders)
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

  if (!shop && !error) {
    return (
      <Shell title="Shop" lede="Loading…">
        <Notice>Loading…</Notice>
      </Shell>
    )
  }
  if (!shop) {
    return (
      <Shell title="Shop" lede="Accept an invite first if you do not have a kitchen yet.">
        <ErrorText>{error}</ErrorText>
      </Shell>
    )
  }

  return (
    <Shell title="Shop" lede="The story, photos, and whether this kitchen is taking orders.">
      <form
        className="mt-6 grid gap-3 rounded-[20px] border border-line bg-card p-6"
        onSubmit={(event) => {
          event.preventDefault()
          setBusy(true)
          setError('')
          updateShop({
            description,
            logoUrl: logo,
            shopPhotoUrl: photo,
            deliveryFeeUgx: Number(fee) || 0,
            isOpen,
            acceptingOrders: accepting,
          })
            .then(() => load())
            .catch((err: unknown) => setError(err instanceof ApiError ? err.message : 'That change did not finish.'))
            .finally(() => setBusy(false))
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
        <Field label="Delivery fee (UGX)">
          <input className={inputClass} type="number" min="0" value={fee} onChange={(event) => setFee(event.target.value)} />
        </Field>
        <label className="flex min-h-11 items-center gap-2 text-sm">
          <input type="checkbox" checked={isOpen} onChange={(event) => setIsOpen(event.target.checked)} />
          Shop is open
        </label>
        <label className="flex min-h-11 items-center gap-2 text-sm">
          <input type="checkbox" checked={accepting} onChange={(event) => setAccepting(event.target.checked)} />
          Taking new orders
        </label>
        <PrimaryButton type="submit" disabled={busy}>
          Save shop
        </PrimaryButton>
      </form>
      <ErrorText>{error}</ErrorText>
    </Shell>
  )
}
