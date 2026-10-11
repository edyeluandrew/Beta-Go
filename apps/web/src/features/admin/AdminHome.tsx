import { useState } from 'react'
import { ApiError } from '../../shared/api/client'
import { useAuth } from '../../shared/auth/AuthProvider'
import { canTouchMoney } from '../../shared/auth/roles'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Field, GhostButton, Notice, PrimaryButton, inputClass } from '../../shared/ui/widgets'
import { reconcileOrders, reconcilePayments, resolveDispute } from './api'

export function AdminHome() {
  const { user } = useAuth()
  const roles = user?.roles ?? []
  const [orderId, setOrderId] = useState('')
  const [outcome, setOutcome] = useState('COMPLETED')
  const [reason, setReason] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const run = async (action: () => Promise<string>) => {
    setBusy(true)
    setError('')
    setMessage('')
    try {
      setMessage(await action())
    } catch (err: unknown) {
      setError(err instanceof ApiError ? err.message : 'That admin action did not finish.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Shell title={`Hi ${user?.name || 'there'}`} lede="Kitchens, money, and launch sit in the sidebar. Profile keeps the details for this account.">
      <section className="mt-6 rounded-[20px] border border-line bg-card p-6">
        <h2 className="text-2xl">Reconcile</h2>
        <p className="mt-2 text-sm text-muted">Uncertain money and late kitchen steps get reconciled, never ignored.</p>
        <div className="mt-4 flex flex-wrap gap-2">
          {canTouchMoney(roles) && (
            <PrimaryButton
              type="button"
              disabled={busy}
              onClick={() =>
                void run(async () => {
                  const result = await reconcilePayments()
                  return `Payments checked ${result.checked}, paid ${result.paid}, expired ${result.expired}.`
                })
              }
            >
              Reconcile payments
            </PrimaryButton>
          )}
          <GhostButton
            type="button"
            disabled={busy}
            onClick={() =>
              void run(async () => {
                await reconcileOrders()
                return 'Order clocks were checked.'
              })
            }
          >
            Reconcile orders
          </GhostButton>
        </div>
        <form
          className="mt-4 grid gap-3"
          onSubmit={(event) => {
            event.preventDefault()
            void run(async () => {
              const result = await resolveDispute(orderId.trim(), outcome, reason)
              return `Dispute closed. Order is ${result.order.status}.`
            })
          }}
        >
          <Field label="Disputed order id">
            <input className={inputClass} value={orderId} onChange={(event) => setOrderId(event.target.value)} required />
          </Field>
          <Field label="Outcome">
            <select className={inputClass} value={outcome} onChange={(event) => setOutcome(event.target.value)}>
              <option value="COMPLETED">Completed</option>
              <option value="CANCELLED">Cancelled</option>
            </select>
          </Field>
          <Field label="Reason">
            <input className={inputClass} value={reason} onChange={(event) => setReason(event.target.value)} required />
          </Field>
          <PrimaryButton type="submit" disabled={busy}>
            Resolve dispute
          </PrimaryButton>
        </form>
        {message && <Notice>{message}</Notice>}
        <ErrorText>{error}</ErrorText>
      </section>
    </Shell>
  )
}
