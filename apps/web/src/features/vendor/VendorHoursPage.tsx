import { useEffect, useState } from 'react'
import { ApiError } from '../../shared/api/client'
import { weekdayName } from '../../shared/ui/money'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Notice, PrimaryButton, inputClass } from '../../shared/ui/widgets'
import { getWorkspace, saveHours } from './api'
import type { OpeningHour } from './types'

const emptyHours = (): OpeningHour[] =>
  [0, 1, 2, 3, 4, 5, 6].map((weekday) => ({
    weekday,
    opensAt: '08:00',
    closesAt: '21:00',
    closed: weekday === 0,
  }))

export function VendorHoursPage() {
  const [ready, setReady] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [hours, setHours] = useState<OpeningHour[]>(emptyHours())

  useEffect(() => {
    let active = true
    getWorkspace()
      .then((result) => {
        if (!active) return
        setHours(result.hours.length === 7 ? result.hours : emptyHours())
        setReady(true)
      })
      .catch((err: unknown) => {
        if (active) setError(err instanceof ApiError ? err.message : 'Hours could not be loaded.')
      })
    return () => {
      active = false
    }
  }, [])

  if (!ready && !error) {
    return (
      <Shell title="Hours" lede="Loading…">
        <Notice>Loading…</Notice>
      </Shell>
    )
  }
  if (!ready) {
    return (
      <Shell title="Hours" lede="Accept an invite first if you do not have a kitchen yet.">
        <ErrorText>{error}</ErrorText>
      </Shell>
    )
  }

  return (
    <Shell title="Hours" lede="Customers can order only inside these times, in Kampala.">
      <form
        className="mt-6 grid gap-3 rounded-[20px] border border-line bg-card p-6"
        onSubmit={(event) => {
          event.preventDefault()
          setBusy(true)
          setError('')
          saveHours(hours)
            .catch((err: unknown) => setError(err instanceof ApiError ? err.message : 'Those hours did not save.'))
            .finally(() => setBusy(false))
        }}
      >
        {hours.map((day, index) => (
          <div key={day.weekday} className="grid items-center gap-2 sm:grid-cols-[7rem_1fr_1fr_auto]">
            <span className="text-sm">{weekdayName(day.weekday)}</span>
            <input
              className={inputClass}
              type="time"
              aria-label={`${weekdayName(day.weekday)} opens`}
              value={day.opensAt}
              disabled={day.closed}
              onChange={(event) => setHours((current) => current.map((row, rowIndex) => (rowIndex === index ? { ...row, opensAt: event.target.value } : row)))}
            />
            <input
              className={inputClass}
              type="time"
              aria-label={`${weekdayName(day.weekday)} closes`}
              value={day.closesAt}
              disabled={day.closed}
              onChange={(event) => setHours((current) => current.map((row, rowIndex) => (rowIndex === index ? { ...row, closesAt: event.target.value } : row)))}
            />
            <label className="flex min-h-11 items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={day.closed}
                onChange={(event) => setHours((current) => current.map((row, rowIndex) => (rowIndex === index ? { ...row, closed: event.target.checked } : row)))}
              />
              Closed
            </label>
          </div>
        ))}
        <PrimaryButton type="submit" disabled={busy || !ready}>
          Save hours
        </PrimaryButton>
      </form>
      <ErrorText>{error}</ErrorText>
    </Shell>
  )
}
