import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ApiError } from '../../shared/api/client'
import { useAuth } from '../../shared/auth/AuthProvider'
import { canTouchMoney } from '../../shared/auth/roles'
import { prettyStatus } from '../../shared/ui/money'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Field, GhostButton, Notice, PrimaryButton, inputClass } from '../../shared/ui/widgets'
import { getAdminVendor, inviteVendor, setDestination, vendorStatus, verifyDestination } from './api'
import type { AdminVendor, Destination } from './types'

export function VendorDetailPage() {
  const { id = '' } = useParams()
  const { user } = useAuth()
  const [vendor, setVendor] = useState<AdminVendor | null>(null)
  const [destination, setDest] = useState<Destination | null>(null)
  const [missing, setMissing] = useState<string[]>([])
  const [token, setToken] = useState('')
  const [provider, setProvider] = useState('mtn')
  const [account, setAccount] = useState('')
  const [reason, setReason] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const load = async () => {
    const result = await getAdminVendor(id)
    setVendor(result.vendor)
    setDest(result.destination ?? null)
    setMissing(result.missing ?? [])
  }

  useEffect(() => {
    let active = true
    load().catch((err: unknown) => {
      if (active) setError(err instanceof ApiError ? err.message : 'This kitchen could not be loaded.')
    })
    return () => {
      active = false
    }
  }, [id])

  const run = async (action: () => Promise<void>) => {
    setBusy(true)
    setError('')
    try {
      await action()
      await load()
    } catch (err: unknown) {
      setError(err instanceof ApiError ? err.message : 'That kitchen action did not finish.')
    } finally {
      setBusy(false)
    }
  }

  if (!vendor && !error) {
    return (
      <Shell title="Kitchen" lede="Loading…">
        <Notice>Loading…</Notice>
      </Shell>
    )
  }
  if (!vendor) {
    return (
      <Shell title="Kitchen" lede="This kitchen was not found.">
        <ErrorText>{error}</ErrorText>
      </Shell>
    )
  }

  return (
    <Shell title={vendor.businessName} lede={`${prettyStatus(vendor.status)} · ${vendor.contactEmail} · ${vendor.address}`}>
      <p className="mt-2 text-sm text-muted">
        {vendor.contactName} · {vendor.phone}
        {vendor.shopPhotoUrl ? ' · shop photo set' : ' · no shop photo yet'}
      </p>

      {vendor.status === 'PENDING_APPROVAL' && (
        <section id="approve" className="mt-6 rounded-[20px] border border-forest bg-leaf p-6">
          <h2 className="text-2xl text-forest-dark">Approve this kitchen</h2>
          {missing.length > 0 ? (
            <ul className="mt-3 list-disc pl-5 text-sm text-forest-dark">
              {missing.map((gap) => (
                <li key={gap}>Still needed: {gap}</li>
              ))}
            </ul>
          ) : (
            <p className="mt-2 text-sm text-forest-dark">Checks are complete. Approve puts it on the marketplace if the pool is at least UGX 10,000.</p>
          )}
          <PrimaryButton className="mt-4" type="button" disabled={busy} onClick={() => void run(async () => { await vendorStatus(vendor.id, 'approve') })}>
            Approve kitchen
          </PrimaryButton>
        </section>
      )}

      <section className="mt-6 rounded-[20px] border border-line bg-card p-6">
        <h2 className="text-2xl">Invite</h2>
        <p className="mt-2 text-sm text-muted">
          This invite only works for <strong className="text-ink">{vendor.contactEmail}</strong>. That person must be signed in with that Google account, then open Invite and paste the token.
        </p>
        <PrimaryButton
          className="mt-3"
          type="button"
          disabled={busy}
          onClick={() =>
            void run(async () => {
              const invited = await inviteVendor(vendor.id)
              setToken(invited.token)
            })
          }
        >
          Create invite
        </PrimaryButton>
        {token && (
          <p className="mt-3 break-all rounded-xl bg-leaf p-3 text-forest-dark">
            {token}
          </p>
        )}
      </section>

      <form
        className="mt-6 grid gap-3 rounded-[20px] border border-line bg-card p-6"
        onSubmit={(event) => {
          event.preventDefault()
          void run(async () => {
            await setDestination(vendor.id, provider, account)
          })
        }}
      >
        <h2 className="text-2xl">Payment destination</h2>
        <p className="text-sm text-muted">
          {destination ? `${destination.provider} ${destination.account} · ${prettyStatus(destination.status)}` : 'Not set. Beta-Go never stores a PIN.'}
        </p>
        <Field label="Provider">
          <select className={inputClass} value={provider} onChange={(event) => setProvider(event.target.value)}>
            <option value="mtn">MTN</option>
            <option value="airtel">Airtel</option>
          </select>
        </Field>
        <Field label="Account number">
          <input className={inputClass} value={account} onChange={(event) => setAccount(event.target.value)} required />
        </Field>
        <div className="flex flex-wrap gap-2">
          <PrimaryButton type="submit" disabled={busy}>
            Save destination
          </PrimaryButton>
          <GhostButton type="button" disabled={busy} onClick={() => void run(async () => { await verifyDestination(vendor.id) })}>
            Mark verified
          </GhostButton>
        </div>
      </form>

      <section className="mt-6 rounded-[20px] border border-line bg-card p-6">
        <h2 className="text-2xl">Other status changes</h2>
        <Field label="Reason for reject, pause, or suspend">
          <input className={inputClass} value={reason} onChange={(event) => setReason(event.target.value)} />
        </Field>
        <div className="mt-3 flex flex-wrap gap-2">
          {vendor.status !== 'PENDING_APPROVAL' && (
            <PrimaryButton type="button" disabled={busy} onClick={() => void run(async () => { await vendorStatus(vendor.id, 'approve') })}>
              Approve
            </PrimaryButton>
          )}
          <GhostButton type="button" disabled={busy} onClick={() => void run(async () => { await vendorStatus(vendor.id, 'reject', reason) })}>
            Reject
          </GhostButton>
          <GhostButton type="button" disabled={busy} onClick={() => void run(async () => { await vendorStatus(vendor.id, 'pause', reason) })}>
            Pause
          </GhostButton>
          <GhostButton type="button" disabled={busy} onClick={() => void run(async () => { await vendorStatus(vendor.id, 'suspend', reason) })}>
            Suspend
          </GhostButton>
          <GhostButton type="button" disabled={busy} onClick={() => void run(async () => { await vendorStatus(vendor.id, 'reactivate') })}>
            Reactivate
          </GhostButton>
        </div>
        {missing.length > 0 ? (
          <ul className="mt-3 list-disc pl-5 text-sm text-muted">
            {missing.map((gap) => (
              <li key={gap}>Still needed: {gap}</li>
            ))}
          </ul>
        ) : (
          <p className="mt-3 text-sm text-muted">Go-live checks are complete. A pool under UGX 10,000 still leaves the shop inactive after approve.</p>
        )}
      </section>

      {canTouchMoney(user?.roles ?? []) && (
        <Link className="mt-6 block rounded-2xl border border-line bg-card p-4 text-ink no-underline" to={`/a/money/${vendor.id}`}>
          <strong>Open the commission pool</strong>
          <Notice>Finance and super admin only. Support cannot adjust money.</Notice>
        </Link>
      )}
      <ErrorText>{error}</ErrorText>
    </Shell>
  )
}
