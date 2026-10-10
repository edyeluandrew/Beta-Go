import { useEffect, useState } from 'react'
import { ApiError } from '../../shared/api/client'
import { ugx } from '../../shared/ui/money'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Field, Notice, PrimaryButton, inputClass } from '../../shared/ui/widgets'
import { getLedger, getPool, startTopUp } from './api'
import type { LedgerEntry, PoolAccount } from './types'

export function VendorPoolPage() {
  const [account, setAccount] = useState<PoolAccount | null>(null)
  const [entries, setEntries] = useState<LedgerEntry[]>([])
  const [amount, setAmount] = useState('20000')
  const [provider, setProvider] = useState('mtn')
  const [phone, setPhone] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const load = async () => {
    const [pool, ledger] = await Promise.all([getPool(), getLedger()])
    setAccount(pool.account)
    setEntries(ledger.entries)
  }

  useEffect(() => {
    let active = true
    load().catch((err: unknown) => {
      if (active) setError(err instanceof ApiError ? err.message : 'The pool could not be loaded.')
    })
    return () => {
      active = false
    }
  }, [])

  return (
    <Shell title="Commission pool" lede="Food payments never credit this pool. Below UGX 10,000 available the shop goes inactive.">
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
      {account?.lowBalance && <Notice>Balance is under the UGX 20,000 warning. Top up before the floor hides the shop.</Notice>}
      <form
        className="mt-6 grid w-full max-w-[440px] gap-3 rounded-[20px] border border-line bg-card p-6"
        onSubmit={(event) => {
          event.preventDefault()
          setBusy(true)
          setError('')
          startTopUp({
            amountUgx: Number(amount),
            provider,
            phone,
            idempotencyKey: crypto.randomUUID(),
          })
            .then(() => load())
            .catch((err: unknown) => setError(err instanceof ApiError ? err.message : 'Top-up did not start.'))
            .finally(() => setBusy(false))
        }}
      >
        <Field label="Amount (UGX 10,000 to 1,000,000)">
          <input className={inputClass} type="number" min="10000" max="1000000" value={amount} onChange={(event) => setAmount(event.target.value)} />
        </Field>
        <Field label="Provider">
          <select className={inputClass} value={provider} onChange={(event) => setProvider(event.target.value)}>
            <option value="mtn">MTN</option>
            <option value="airtel">Airtel</option>
          </select>
        </Field>
        <Field label="Phone">
          <input className={inputClass} value={phone} onChange={(event) => setPhone(event.target.value)} required />
        </Field>
        <p className="text-sm text-muted">A screenshot never credits the pool. Only a signed provider notice does.</p>
        <PrimaryButton type="submit" disabled={busy}>
          Start top-up
        </PrimaryButton>
      </form>
      {entries.length > 0 && (
        <ul className="mt-5 grid gap-2.5">
          {entries.map((entry) => (
            <li key={entry.id} className="rounded-2xl border border-line bg-card p-4">
              <strong>{entry.operation}</strong>
              <p className="mt-1 text-sm text-muted">
                {entry.direction} {ugx(entry.amountUgx)} · {entry.stream}
              </p>
            </li>
          ))}
        </ul>
      )}
      <ErrorText>{error}</ErrorText>
    </Shell>
  )
}
