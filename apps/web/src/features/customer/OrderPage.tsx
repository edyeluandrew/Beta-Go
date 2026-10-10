import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { ApiError } from '../../shared/api/client'
import { prettyStatus, ugx } from '../../shared/ui/money'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Field, GhostButton, Notice, PrimaryButton, inputClass } from '../../shared/ui/widgets'
import { cancelOrder, confirmOrder, getOrder, getPayment, openDispute, startPayment } from './api'
import type { CustomerOrder, Payment } from './types'

export function OrderPage() {
  const { id = '' } = useParams()
  const [order, setOrder] = useState<CustomerOrder | null>(null)
  const [payment, setPayment] = useState<Payment | null>(null)
  const [provider, setProvider] = useState<'mtn' | 'airtel'>('mtn')
  const [phone, setPhone] = useState('')
  const [reason, setReason] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const refresh = async () => {
    const result = await getOrder(id)
    setOrder(result.order)
    try {
      const paid = await getPayment(id)
      setPayment(paid.payment)
    } catch (err: unknown) {
      if (err instanceof ApiError && err.status === 404) {
        setPayment(null)
        return
      }
      throw err
    }
  }

  useEffect(() => {
    let active = true
    refresh().catch((err: unknown) => {
      if (active) setError(err instanceof ApiError ? err.message : 'This order could not be loaded.')
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
      await refresh()
    } catch (err: unknown) {
      setError(err instanceof ApiError ? err.message : 'That action did not finish.')
    } finally {
      setBusy(false)
    }
  }

  if (!order && !error) {
    return (
      <Shell title="Order" lede="Loading…">
        <Notice>Loading…</Notice>
      </Shell>
    )
  }
  if (!order) {
    return (
      <Shell title="Order" lede="This order was not found.">
        <ErrorText>{error}</ErrorText>
      </Shell>
    )
  }

  return (
    <Shell title={`Order ${prettyStatus(order.status)}`} lede="The server decides the price and whether payment succeeded.">
      <ul className="mt-5 grid gap-2.5">
        {order.items.map((item) => (
          <li key={item.itemId} className="rounded-2xl border border-line bg-card p-4">
            <strong>
              {item.quantity}× {item.name}
            </strong>
            <p className="mt-1 text-sm text-muted">{ugx(item.subtotalUgx)}</p>
          </li>
        ))}
      </ul>
      <p className="mt-4">
        Food {ugx(order.foodSubtotalUgx)}
        {order.deliveryFeeUgx > 0 ? ` + delivery ${ugx(order.deliveryFeeUgx)}` : ''} = {ugx(order.totalUgx)}
      </p>
      {order.pickupCode && <Notice>Pickup code {order.pickupCode}</Notice>}
      {order.cancelRequested && <Notice>Cancel requested. Waiting for the kitchen.</Notice>}
      {payment && (
        <p className="mt-2 text-sm text-muted">
          Payment {payment.status}
          {payment.provider ? ` · ${payment.provider}` : ''}
          {payment.phone ? ` · ${payment.phone}` : ''}
        </p>
      )}
      {order.status === 'PENDING_PAYMENT' && (
        <form
          className="mt-6 grid w-full max-w-[440px] gap-3 rounded-[20px] border border-line bg-card p-6"
          onSubmit={(event) => {
            event.preventDefault()
            void run(async () => {
              await startPayment(order.id, provider, phone)
            })
          }}
        >
          <Field label="Pay with">
            <select className={inputClass} value={provider} onChange={(event) => setProvider(event.target.value as 'mtn' | 'airtel')}>
              <option value="mtn">MTN MoMo</option>
              <option value="airtel">Airtel Money</option>
            </select>
          </Field>
          <Field label="Phone">
            <input className={inputClass} value={phone} onChange={(event) => setPhone(event.target.value)} required />
          </Field>
          <p className="text-sm text-muted">You pay the kitchen number. Live MTN and Airtel stay off until a provider test.</p>
          <PrimaryButton type="submit" disabled={busy}>
            Start payment
          </PrimaryButton>
        </form>
      )}
      {['PENDING_PAYMENT', 'AWAITING_VENDOR', 'CONFIRMED'].includes(order.status) && (
        <GhostButton className="mt-4" type="button" disabled={busy} onClick={() => void run(async () => { await cancelOrder(order.id) })}>
          Cancel order
        </GhostButton>
      )}
      {order.status === 'FULFILLED' && (
        <div className="mt-6 grid w-full max-w-[440px] gap-3 rounded-[20px] border border-line bg-card p-6">
          <PrimaryButton type="button" disabled={busy} onClick={() => void run(async () => { await confirmOrder(order.id) })}>
            Confirm you received it
          </PrimaryButton>
          <Field label="Dispute reason">
            <input className={inputClass} value={reason} onChange={(event) => setReason(event.target.value)} />
          </Field>
          <GhostButton type="button" disabled={busy || !reason.trim()} onClick={() => void run(async () => { await openDispute(order.id, reason) })}>
            Open a dispute
          </GhostButton>
        </div>
      )}
      <ErrorText>{error}</ErrorText>
    </Shell>
  )
}
