import { useEffect, useState } from 'react'
import { BadgeCheck, Mail, MapPin, Phone, UserRound } from 'lucide-react'
import { ApiError } from '../../shared/api/client'
import { useAuth } from '../../shared/auth/AuthProvider'
import { isSuperAdmin } from '../../shared/auth/roles'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Field, PrimaryButton, inputClass } from '../../shared/ui/widgets'
import { ProfileMark } from '../../shared/ui/ProfileMark'

export function AdminProfilePage() {
  const { user, updateProfile } = useAuth()
  const [name, setName] = useState(user?.name ?? '')
  const [phone, setPhone] = useState(user?.phone ?? '')
  const [area, setArea] = useState(user?.area ?? '')
  const [problems, setProblems] = useState<{ name?: string; phone?: string; area?: string }>({})
  const [error, setError] = useState('')
  const [saved, setSaved] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    setName(user?.name ?? '')
    setPhone(user?.phone ?? '')
    setArea(user?.area ?? '')
  }, [user])

  if (!user) return null

  const rows = [
    { icon: UserRound, label: 'Name', value: user.name || 'Not added yet' },
    { icon: Phone, label: 'Phone', value: user.phone || 'Not added yet' },
    { icon: MapPin, label: 'Area', value: user.area || 'Not added yet' },
    { icon: Mail, label: 'Email', value: user.email },
    { icon: BadgeCheck, label: 'Role', value: staffLabel(user.roles) },
  ]

  return (
    <Shell title="Profile" lede="The name, phone, area, and email for this staff account live here.">
      <section className="mt-5 rounded-[20px] border border-line bg-card p-6">
        <div className="flex items-center gap-4">
          <ProfileMark user={user} className="h-16 w-16" />
          <div>
            <h2 className="text-2xl">{user.name || 'Your profile'}</h2>
            <p className="text-sm text-muted">{staffLabel(user.roles)}</p>
          </div>
        </div>
        <ul className="mt-5 grid gap-3">
          {rows.map((row) => (
            <li key={row.label} className="flex items-start gap-3 rounded-2xl bg-cream px-3 py-3">
              <row.icon aria-hidden className="mt-0.5 h-5 w-5 shrink-0 text-forest" />
              <div className="min-w-0">
                <p className="text-xs font-semibold uppercase tracking-[0.12em] text-muted">{row.label}</p>
                <p className="mt-0.5 break-all">{row.value}</p>
              </div>
            </li>
          ))}
        </ul>
      </section>

      <section className="mt-6 rounded-[20px] border border-line bg-card p-6">
        <h2 className="text-2xl">Update details</h2>
        <form
          className="mt-4 grid gap-3"
          onSubmit={(event) => {
            event.preventDefault()
            const next = detailProblems(name, phone, area)
            setProblems(next)
            setError('')
            setSaved('')
            if (next.name || next.phone || next.area) return
            setBusy(true)
            void updateProfile({ name: name.trim(), phone: phone.trim(), area: area.trim() })
              .then(() => setSaved('Saved.'))
              .catch((err: unknown) => setError(err instanceof ApiError ? err.message : 'Those details did not save.'))
              .finally(() => setBusy(false))
          }}
        >
          <Field label="Name" error={problems.name}>
            <input className={fieldClass(problems.name)} value={name} onChange={(event) => setName(event.target.value)} autoComplete="name" />
          </Field>
          <Field label="Phone" error={problems.phone}>
            <input className={fieldClass(problems.phone)} value={phone} onChange={(event) => setPhone(event.target.value)} inputMode="tel" autoComplete="tel" />
          </Field>
          <Field label="Area in Kabale" error={problems.area}>
            <input className={fieldClass(problems.area)} value={area} onChange={(event) => setArea(event.target.value)} autoComplete="address-level2" />
          </Field>
          <PrimaryButton type="submit" disabled={busy}>
            {busy ? 'Saving…' : 'Save details'}
          </PrimaryButton>
          {saved ? <p className="text-sm text-forest">{saved}</p> : null}
        </form>
        <ErrorText>{error}</ErrorText>
        {isSuperAdmin(user.roles) ? <p className="mt-4 text-sm text-muted">Super admin is granted from the bootstrap email. It is not chosen on this form.</p> : null}
      </section>
    </Shell>
  )
}

function staffLabel(roles: string[]) {
  if (roles.includes('super_admin')) return 'Super admin'
  if (roles.includes('finance') && roles.includes('support')) return 'Finance and support'
  if (roles.includes('finance')) return 'Finance'
  if (roles.includes('support')) return 'Support'
  return 'Staff'
}

function fieldClass(error?: string) {
  return error ? `${inputClass} border-danger` : inputClass
}

function detailProblems(name: string, phone: string, area: string) {
  const problems: { name?: string; phone?: string; area?: string } = {}
  if (name.trim().length < 2) problems.name = 'Add the name we should call you.'
  const digits = phone.replace(/\D/g, '')
  const ok = (digits.length === 10 && digits.startsWith('07')) || (digits.length === 12 && digits.startsWith('2567'))
  if (phone.trim() && !ok) problems.phone = 'Use an MTN or Airtel number, like 0772 000 111.'
  if (area.trim() && area.trim().length < 2) problems.area = 'Add your area in Kabale.'
  return problems
}
