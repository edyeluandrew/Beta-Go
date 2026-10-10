import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ApiError } from '../../shared/api/client'
import { ugx } from '../../shared/ui/money'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Field, GhostButton, Notice, PrimaryButton, inputClass } from '../../shared/ui/widgets'
import { approveAdjustment, getAdminPool, requestAdjustment } from './api'
import type { Adjustment, PoolAccount } from './types'

export function MoneyPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const [vendorId, setVendorId] = useState(id)
  const [account, setAccount] = useState<PoolAccount | null>(null)
  const [pending, setPending] = useState<Adjustment | null>(null)
  const [approveId, setApproveId] = useState('')
  const [amount, setAmount] = useState('10000')
  const [reason, setReason] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!id) return
    let active = true
    getAdminPool(id)
      .then((result) => {
        if (active) setAccount(result.account)
      })
      .catch((err: unknown) => {
        if (active) setError(err instanceof ApiError ? err.message : 'The pool could not be loaded.')
      })
    return () => {
      active = false
    }
  }, [id])

  const load = async (target: string) => {
    const result = await getAdminPool(target)
    setAccount(result.account)
    navigate(`/a/money/${target}`, { replace: true })
  }

  const run = async (action: () => Promise<void>) => {
    setBusy(true)
    setError('')
    try {
      await action()
    } catch (err: unknown) {
      setError(err instanceof ApiError ? err.message : 'That money action did not finish.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Shell title="Commission pool" lede="Food payments never credit this pool. Support cannot open these routes. The server blocks it.">
      <form
        className="mt-6 grid gap-3 rounded-[20px] border border-line bg-card p-6"
        onSubmit={(event) => {
          event.preventDefault()
          void run(async () => {
            await load(vendorId.trim())
          })
        }}
      >
        <Field label="Vendor id">
          <input className={inputClass} value={vendorId} onChange={(event) => setVendorId(event.target.value)} required />
        </Field>
        <PrimaryButton type="submit" disabled={busy}>
          Load pool
        </PrimaryButton>
      </form>

      {account && (
        <div className="mt-5 grid gap-2.5 sm:grid-cols-2">
          <div className="rounded-2xl border border-line bg-card p-4">
            <p className="text-sm text-muted">Available</p>
            <p className="font-display text-3xl">{ugx(account.availableUgx)}</p>
          </div>
          <div className="rounded-2xl border border-line bg-card p-4">
            <p className="text-sm text-muted">Reserved</p>
            <p className="font-display text-3xl">{ugx(account.reservedUgx)}</p>
          </div>
        </div>
      )}
      {account?.lowBalance && <Notice>This kitchen is under the UGX 20,000 warning.</Notice>}

      <form
        className="mt-6 grid gap-3 rounded-[20px] border border-line bg-card p-6"
        onSubmit={(event) => {
          event.preventDefault()
          void run(async () => {
            const result = await requestAdjustment(vendorId.trim(), Number(amount), reason)
            setAccount(result.account)
            setPending(result.adjustment)
            if (result.adjustment.status !== 'APPLIED') setApproveId(result.adjustment.id)
          })
        }}
      >
        <h2 className="text-2xl">Adjustment</h2>
        <Field label="Amount (negative withdraws)">
          <input className={inputClass} type="number" value={amount} onChange={(event) => setAmount(event.target.value)} required />
        </Field>
        <Field label="Reason">
          <input className={inputClass} value={reason} onChange={(event) => setReason(event.target.value)} required />
        </Field>
        <p className="text-sm text-muted">Above UGX 50,000 a second finance or super admin must approve. You cannot approve your own request.</p>
        <PrimaryButton type="submit" disabled={busy || !vendorId.trim()}>
          Request adjustment
        </PrimaryButton>
      </form>

      {pending && (
        <Notice>
          Last adjustment {pending.id} is {pending.status} for {ugx(pending.amountUgx)}.
        </Notice>
      )}

      <form
        className="mt-6 grid gap-3 rounded-[20px] border border-line bg-card p-6"
        onSubmit={(event) => {
          event.preventDefault()
          void run(async () => {
            const result = await approveAdjustment(approveId.trim())
            setAccount(result.account)
            setPending(result.adjustment)
          })
        }}
      >
        <h2 className="text-2xl">Second approval</h2>
        <Field label="Adjustment id">
          <input className={inputClass} value={approveId} onChange={(event) => setApproveId(event.target.value)} required />
        </Field>
        <GhostButton type="submit" disabled={busy}>
          Approve adjustment
        </GhostButton>
      </form>
      <ErrorText>{error}</ErrorText>
    </Shell>
  )
}
