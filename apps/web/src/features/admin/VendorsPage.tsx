import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ApiError } from '../../shared/api/client'
import { prettyStatus } from '../../shared/ui/money'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Field, Notice, PrimaryButton, inputClass } from '../../shared/ui/widgets'
import { createVendor, listAdminVendors } from './api'
import type { AdminVendor } from './types'

export function VendorsPage() {
  const navigate = useNavigate()
  const [vendors, setVendors] = useState<AdminVendor[] | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [businessName, setBusinessName] = useState('')
  const [contactName, setContactName] = useState('')
  const [contactEmail, setContactEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [address, setAddress] = useState('Kabale')

  const load = async () => {
    const result = await listAdminVendors()
    setVendors(result.vendors)
  }

  useEffect(() => {
    let active = true
    load().catch((err: unknown) => {
      if (active) setError(err instanceof ApiError ? err.message : 'Vendors could not be loaded.')
    })
    return () => {
      active = false
    }
  }, [])

  return (
    <Shell title="Kitchens" lede="An admin opens the account. The vendor builds the menu after they accept the Google invite.">
      <form
        className="mt-6 grid gap-3 rounded-[20px] border border-line bg-card p-6"
        onSubmit={(event) => {
          event.preventDefault()
          setBusy(true)
          setError('')
          createVendor({ businessName, contactName, contactEmail, phone, address })
            .then((result) => navigate(`/a/vendors/${result.vendor.id}`))
            .catch((err: unknown) => setError(err instanceof ApiError ? err.message : 'The kitchen was not created.'))
            .finally(() => setBusy(false))
        }}
      >
        <Field label="Business name">
          <input className={inputClass} value={businessName} onChange={(event) => setBusinessName(event.target.value)} required />
        </Field>
        <Field label="Contact name">
          <input className={inputClass} value={contactName} onChange={(event) => setContactName(event.target.value)} required />
        </Field>
        <Field label="Google email">
          <input className={inputClass} type="email" value={contactEmail} onChange={(event) => setContactEmail(event.target.value)} required />
        </Field>
        <Field label="Phone">
          <input className={inputClass} value={phone} onChange={(event) => setPhone(event.target.value)} required />
        </Field>
        <Field label="Address">
          <input className={inputClass} value={address} onChange={(event) => setAddress(event.target.value)} required />
        </Field>
        <PrimaryButton type="submit" disabled={busy}>
          Create kitchen
        </PrimaryButton>
      </form>
      {vendors === null && !error && <Notice>Loading kitchens…</Notice>}
      {vendors?.length === 0 && <Notice>No kitchens yet. Create the first one above.</Notice>}
      {vendors && vendors.length > 0 && (
        <ul className="mt-5 grid gap-2.5">
          {vendors.map((vendor) => (
            <li key={vendor.id}>
              <Link className="block rounded-2xl border border-line bg-card p-4 text-ink no-underline" to={`/a/vendors/${vendor.id}`}>
                <strong>{vendor.businessName}</strong>
                <p className="mt-1 text-sm text-muted">
                  {prettyStatus(vendor.status)} · {vendor.contactEmail}
                </p>
                <p className="mt-2 text-sm font-semibold text-forest">
                  {vendor.status === 'PENDING_APPROVAL' ? 'Open this kitchen to approve it →' : 'Open kitchen →'}
                </p>
              </Link>
            </li>
          ))}
        </ul>
      )}
      <ErrorText>{error}</ErrorText>
    </Shell>
  )
}
