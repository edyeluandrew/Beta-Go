import { useEffect, useState } from 'react'
import { ApiError } from '../../shared/api/client'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Field, Notice, PrimaryButton, inputClass } from '../../shared/ui/widgets'
import { grantStaff, launchAudit, launchBackups, launchSecurity, recordBackup } from './api'
import type { AuditEntry, BackupCheck, SecurityReport } from './types'

const flagLabel: Record<keyof SecurityReport, string> = {
  env: 'Environment',
  cookieSecure: 'Cookie secure',
  googleConfigured: 'Google configured',
  paymentProvider: 'Payment provider',
  paymentWebhookSet: 'Webhook secret set',
  sessionSecretSet: 'Session secret set',
  sessionSecretIsExample: 'Session secret is still the example',
  webOrigin: 'Web origin',
  sandboxBlocked: 'Sandbox blocked in production',
}

export function LaunchPage() {
  const [report, setReport] = useState<SecurityReport | null>(null)
  const [audit, setAudit] = useState<AuditEntry[]>([])
  const [backups, setBackups] = useState<BackupCheck[]>([])
  const [note, setNote] = useState('Neon recovery point checked')
  const [email, setEmail] = useState('')
  const [role, setRole] = useState('support')
  const [granted, setGranted] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const load = async () => {
    const [security, trail, checks] = await Promise.all([launchSecurity(), launchAudit(), launchBackups()])
    setReport(security)
    setAudit(trail.items)
    setBackups(checks.items)
  }

  useEffect(() => {
    let active = true
    load().catch((err: unknown) => {
      if (active) setError(err instanceof ApiError ? err.message : 'Launch data could not be loaded.')
    })
    return () => {
      active = false
    }
  }, [])

  const run = async (action: () => Promise<void>) => {
    setBusy(true)
    setError('')
    try {
      await action()
      await load()
    } catch (err: unknown) {
      setError(err instanceof ApiError ? err.message : 'That launch action did not finish.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Shell title="Launch" lede="Flags only. This page never shows DATABASE_URL, the session secret, or the webhook secret.">
      {report && (
        <ul className="mt-5 grid gap-2.5">
          {(Object.keys(flagLabel) as (keyof SecurityReport)[]).map((key) => (
            <li key={key} className="rounded-2xl border border-line bg-card p-4">
              <strong>{flagLabel[key]}</strong>
              <p className="mt-1 text-sm text-muted">{String(report[key])}</p>
            </li>
          ))}
        </ul>
      )}

      <form
        className="mt-6 grid gap-3 rounded-[20px] border border-line bg-card p-6"
        onSubmit={(event) => {
          event.preventDefault()
          void run(async () => {
            await recordBackup(note)
          })
        }}
      >
        <h2 className="text-2xl">Backup check</h2>
        <p className="text-sm text-muted">Confirm Neon point-in-time recovery in the console, then record it. This does not dump the database.</p>
        <Field label="Note">
          <input className={inputClass} value={note} onChange={(event) => setNote(event.target.value)} />
        </Field>
        <PrimaryButton type="submit" disabled={busy}>
          Record backup check
        </PrimaryButton>
        {backups[0] && <Notice>Last check: {backups[0].note}</Notice>}
      </form>

      <form
        className="mt-6 grid gap-3 rounded-[20px] border border-line bg-card p-6"
        onSubmit={(event) => {
          event.preventDefault()
          void run(async () => {
            const result = await grantStaff(email.trim(), role)
            setGranted(`${result.email} now has ${result.roles.join(', ')}. They must sign in again.`)
            setEmail('')
          })
        }}
      >
        <h2 className="text-2xl">Grant staff</h2>
        <p className="text-sm text-muted">Support or finance only. Not vendor. Not super admin. The person must already have signed in with Google.</p>
        <Field label="Google email">
          <input className={inputClass} type="email" value={email} onChange={(event) => setEmail(event.target.value)} required />
        </Field>
        <Field label="Role">
          <select className={inputClass} value={role} onChange={(event) => setRole(event.target.value)}>
            <option value="support">support</option>
            <option value="finance">finance</option>
          </select>
        </Field>
        <PrimaryButton type="submit" disabled={busy}>
          Grant role
        </PrimaryButton>
        {granted && <Notice>{granted}</Notice>}
      </form>

      {audit.length > 0 && (
        <ul className="mt-5 grid gap-2.5">
          {audit.map((entry) => (
            <li key={entry.id} className="rounded-2xl border border-line bg-card p-4">
              <strong>{entry.action}</strong>
              <p className="mt-1 text-sm text-muted">{entry.reason || '—'} · {new Date(entry.createdAt).toLocaleString()}</p>
            </li>
          ))}
        </ul>
      )}
      <ErrorText>{error}</ErrorText>
    </Shell>
  )
}
