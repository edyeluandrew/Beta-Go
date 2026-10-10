import { useState } from 'react'
import { Link } from 'react-router-dom'
import { ApiError } from '../../shared/api/client'
import { useAuth } from '../../shared/auth/AuthProvider'
import { canManageVendors, canTouchMoney, isSuperAdmin } from '../../shared/auth/roles'
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
    <Shell title="Admin" lede="You open kitchen accounts. Vendors build the menu. The server still checks every role.">
      <div className="mt-5 grid gap-2.5">
        {canManageVendors(roles) && (
          <Link className="rounded-2xl border border-line bg-card p-4 text-ink no-underline" to="/a/vendors">
            <strong>Kitchens</strong>
            <Notice>Create a vendor, send the Google invite, set the MTN or Airtel destination, then approve.</Notice>
          </Link>
        )}
        {canTouchMoney(roles) && (
          <Link className="rounded-2xl border border-line bg-card p-4 text-ink no-underline" to="/a/money">
            <strong>Money</strong>
            <Notice>Pool balances and adjustments. Support cannot open this. Above UGX 50,000 needs a second approver.</Notice>
          </Link>
        )}
        {isSuperAdmin(roles) && (
          <Link className="rounded-2xl border border-line bg-card p-4 text-ink no-underline" to="/a/launch">
            <strong>Launch</strong>
            <Notice>Security flags, audit, Neon backup check, and grant support or finance.</Notice>
          </Link>
        )}
        {!canManageVendors(roles) && !canTouchMoney(roles) && (
          <Notice>This account does not have a staff role that can use admin tools.</Notice>
        )}
      </div>

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
