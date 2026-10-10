import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { ApiError } from '../../shared/api/client'
import { prettyStatus, ugx } from '../../shared/ui/money'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, GhostButton, Notice, PrimaryButton } from '../../shared/ui/widgets'
import { getKitchenOrder, kitchenAction } from './api'
import type { KitchenOrder } from './types'

const nextStep: Record<string, { action: string; label: string } | undefined> = {
  AWAITING_VENDOR: { action: 'accept', label: 'Accept and reserve commission' },
  CONFIRMED: { action: 'preparing', label: 'Start preparing' },
  PREPARING: { action: 'ready', label: 'Mark ready' },
  READY: { action: 'fulfilled', label: 'Mark fulfilled' },
}

export function VendorOrderPage() {
  const { id = '' } = useParams()
  const [order, setOrder] = useState<KitchenOrder | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const load = async () => {
    const result = await getKitchenOrder(id)
    setOrder(result.order)
  }

  useEffect(() => {
    let active = true
    load().catch((err: unknown) => {
      if (active) setError(err instanceof ApiError ? err.message : 'This order could not be loaded.')
    })
    return () => {
      active = false
    }
  }, [id])

  const run = async (action: string, body?: Record<string, string>) => {
    setBusy(true)
    setError('')
    try {
      await kitchenAction(id, action, body)
      await load()
    } catch (err: unknown) {
      setError(err instanceof ApiError ? err.message : 'That kitchen action did not finish.')
    } finally {
      setBusy(false)
    }
  }

  if (!order && !error) {
    return (
      <Shell title="Kitchen order" lede="Loading…">
        <Notice>Loading…</Notice>
      </Shell>
    )
  }
  if (!order) {
    return (
      <Shell title="Kitchen order" lede="This order was not found.">
        <ErrorText>{error}</ErrorText>
      </Shell>
    )
  }

  const step = nextStep[order.status]

  return (
    <Shell title={prettyStatus(order.status)} lede={`${ugx(order.totalUgx)} · ${order.fulfilment}`}>
      {order.pickupCode && <Notice>Pickup code {order.pickupCode}</Notice>}
      {order.cancelRequested && <Notice>The customer asked to cancel before you started cooking.</Notice>}
      <div className="mt-5 flex flex-wrap gap-2">
        {step && (
          <PrimaryButton type="button" disabled={busy} onClick={() => void run(step.action)}>
            {step.label}
          </PrimaryButton>
        )}
        {order.status === 'AWAITING_VENDOR' && (
          <GhostButton type="button" disabled={busy} onClick={() => void run('reject', { reason: 'VENDOR_UNABLE' })}>
            Reject
          </GhostButton>
        )}
        {order.cancelRequested && ['CONFIRMED'].includes(order.status) && (
          <GhostButton type="button" disabled={busy} onClick={() => void run('cancel-approve')}>
            Approve customer cancel
          </GhostButton>
        )}
        {['CONFIRMED', 'PREPARING'].includes(order.status) && (
          <GhostButton type="button" disabled={busy} onClick={() => void run('cancel')}>
            Cannot fulfil
          </GhostButton>
        )}
      </div>
      <ErrorText>{error}</ErrorText>
    </Shell>
  )
}
