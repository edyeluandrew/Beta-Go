import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { homeFor, useAuth } from '../../shared/auth/AuthProvider'
import { ErrorText, Field, inputClass, PrimaryButton } from '../../shared/ui/widgets'
import { clearJoinProfile, destinationFor, readJoinProfile, saveJoinProfile, saveNextPath } from './afterSignIn'
import { GoogleButton } from './GoogleButton'

export function JoinPage() {
  const { user, ready, signInWithGoogle, register, updateProfile } = useAuth()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const [name, setName] = useState('')
  const [phone, setPhone] = useState('')
  const [area, setArea] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [problems, setProblems] = useState<FieldProblems>({})
  const pending = useRef(false)
  const nameRef = useRef<HTMLInputElement>(null)
  const phoneRef = useRef<HTMLInputElement>(null)
  const areaRef = useRef<HTMLInputElement>(null)
  const emailRef = useRef<HTMLInputElement>(null)
  const passwordRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const next = params.get('next')
    if (next) saveNextPath(next)
    const saved = readJoinProfile()
    if (saved) {
      setName(saved.name)
      setPhone(saved.phone)
      setArea(saved.area)
    }
  }, [params])

  useEffect(() => {
    if (ready && user && !pending.current) navigate(homeFor(user), { replace: true })
  }, [navigate, ready, user])

  const edit = (field: keyof FieldProblems, value: string) => {
    if (field === 'name') setName(value)
    if (field === 'phone') setPhone(value)
    if (field === 'area') setArea(value)
    if (field === 'email') setEmail(value)
    if (field === 'password') setPassword(value)
    setProblems((current) => ({ ...current, [field]: undefined }))
  }

  const focusFirst = (next: FieldProblems) => {
    const first = next.name ? nameRef : next.phone ? phoneRef : next.area ? areaRef : next.email ? emailRef : next.password ? passwordRef : null
    first?.current?.focus()
  }

  const createWithPassword = () => {
    const next = accountProblems(name, phone, area, email, password)
    setProblems(next)
    setError('')
    if (next.name || next.phone || next.area || next.email || next.password) {
      focusFirst(next)
      return
    }
    pending.current = true
    setBusy(true)
    void register({
      name: name.trim(),
      phone: phone.trim(),
      area: area.trim(),
      email: email.trim(),
      password,
    })
      .then((signedIn) => {
        clearJoinProfile()
        navigate(destinationFor(signedIn), { replace: true })
      })
      .catch((err: Error) => {
        pending.current = false
        setError(err.message)
      })
      .finally(() => setBusy(false))
  }

  return (
    <main className="grid min-h-screen place-items-center bg-[radial-gradient(circle_at_top_left,#f4c48a_0%,transparent_28%),radial-gradient(circle_at_bottom_right,#9ad4b8_0%,transparent_32%),#f6efe4] px-4 py-6">
      <section className="w-full max-w-[440px] rounded-[20px] border border-line bg-card px-6 pb-6 pt-7 shadow-[0_16px_40px_rgb(26_20_16/8%)]">
        <p className="mb-2 text-xs font-semibold uppercase tracking-[0.14em] text-forest">New customer</p>
        <h1 className="text-[2.6rem] leading-none">Create an account</h1>
        <p className="mt-3 text-muted">Two ways. Use your email if you have no Google account. Use Google if you already have one. Either way we need a name, phone, and area.</p>
        <form
          className="mt-5 grid gap-3"
          onSubmit={(event) => {
            event.preventDefault()
            createWithPassword()
          }}
        >
          <Field label="Your name" error={problems.name}>
            <input ref={nameRef} className={fieldClass(problems.name)} value={name} onChange={(event) => edit('name', event.target.value)} placeholder="As we should call you" autoComplete="name" aria-invalid={Boolean(problems.name)} />
          </Field>
          <Field label="Phone" error={problems.phone}>
            <input ref={phoneRef} className={fieldClass(problems.phone)} value={phone} onChange={(event) => edit('phone', event.target.value)} placeholder="0772 000 111" inputMode="tel" autoComplete="tel" aria-invalid={Boolean(problems.phone)} />
          </Field>
          <Field label="Area in Kabale" error={problems.area}>
            <input ref={areaRef} className={fieldClass(problems.area)} value={area} onChange={(event) => edit('area', event.target.value)} placeholder="Makanga, Central, Kirigime…" autoComplete="address-level2" aria-invalid={Boolean(problems.area)} />
          </Field>
          <Field label="Email" error={problems.email}>
            <input ref={emailRef} className={fieldClass(problems.email)} value={email} onChange={(event) => edit('email', event.target.value)} placeholder="you@email.com" autoComplete="email" inputMode="email" aria-invalid={Boolean(problems.email)} />
          </Field>
          <Field label="Password" error={problems.password}>
            <input ref={passwordRef} className={fieldClass(problems.password)} value={password} onChange={(event) => edit('password', event.target.value)} type="password" autoComplete="new-password" placeholder="At least 8 characters" aria-invalid={Boolean(problems.password)} />
          </Field>
          <PrimaryButton type="submit" className="mt-2 w-full" disabled={busy}>
            {busy ? 'Creating…' : 'Create account'}
          </PrimaryButton>
        </form>
        <p className="mt-5 text-center text-sm text-muted">or continue with Google</p>
        <GoogleButton
          onToken={(idToken) => {
            const next = profileProblems(name, phone, area)
            setProblems(next)
            setError('')
            if (next.name || next.phone || next.area) {
              focusFirst(next)
              return
            }
            saveJoinProfile({ name: name.trim(), phone: phone.trim(), area: area.trim() })
            pending.current = true
            void signInWithGoogle(idToken)
              .then((signedIn) => updateProfile({ name: name.trim(), phone: phone.trim(), area: area.trim() }).then(() => signedIn))
              .then((signedIn) => {
                clearJoinProfile()
                navigate(destinationFor(signedIn), { replace: true })
              })
              .catch((err: Error) => {
                pending.current = false
                setError(err.message)
              })
          }}
          onError={setError}
        />
        <ErrorText>{error}</ErrorText>
        <p className="mt-5 text-sm text-muted">
          Already have an account?{' '}
          <Link className="text-forest" to="/signin">
            Sign in
          </Link>
          {' · '}
          <Link className="text-forest" to="/c">
            Keep browsing
          </Link>
        </p>
      </section>
    </main>
  )
}

type FieldProblems = {
  name?: string
  phone?: string
  area?: string
  email?: string
  password?: string
}

function fieldClass(error?: string) {
  return error ? `${inputClass} border-danger` : inputClass
}

function profileProblems(name: string, phone: string, area: string): FieldProblems {
  const problems: FieldProblems = {}
  if (name.trim().length < 2) problems.name = 'Add the name we should call you.'
  else if (name.trim().length > 80) problems.name = 'Keep the name under 80 characters.'
  if (!ugandaMobile(phone)) problems.phone = 'Use an MTN or Airtel number, like 0772 000 111.'
  if (area.trim().length < 2) problems.area = 'Add your area in Kabale.'
  else if (area.trim().length > 80) problems.area = 'Keep the area under 80 characters.'
  return problems
}

function accountProblems(name: string, phone: string, area: string, email: string, password: string) {
  const problems = profileProblems(name, phone, area)
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) problems.email = 'Use a real email address.'
  if (password.length < 8) problems.password = 'Use a password of at least 8 characters.'
  else if (password.length > 72) problems.password = 'Use a shorter password.'
  return problems
}

function ugandaMobile(raw: string) {
  const digits = raw.replace(/\D/g, '')
  return (digits.length === 10 && digits.startsWith('07')) || (digits.length === 12 && digits.startsWith('2567'))
}
