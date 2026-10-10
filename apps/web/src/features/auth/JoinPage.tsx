import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { homeFor, useAuth } from '../../shared/auth/AuthProvider'
import { ErrorText, Field, inputClass } from '../../shared/ui/widgets'
import { clearJoinProfile, destinationFor, readJoinProfile, saveJoinProfile, saveNextPath } from './afterSignIn'
import { GoogleButton } from './GoogleButton'

export function JoinPage() {
  const { user, ready, signInWithGoogle, updateProfile } = useAuth()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const [name, setName] = useState('')
  const [phone, setPhone] = useState('')
  const [area, setArea] = useState('')
  const [error, setError] = useState('')
  const pending = useRef(false)

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

  const remember = () => saveJoinProfile({ name: name.trim(), phone: phone.trim(), area: area.trim() })

  return (
    <main className="grid min-h-screen place-items-center bg-[radial-gradient(circle_at_top_left,#f4c48a_0%,transparent_28%),radial-gradient(circle_at_bottom_right,#9ad4b8_0%,transparent_32%),#f6efe4] px-4 py-6">
      <section className="w-full max-w-[440px] rounded-[20px] border border-line bg-card px-6 pb-6 pt-7 shadow-[0_16px_40px_rgb(26_20_16/8%)]">
        <p className="mb-2 text-xs font-semibold uppercase tracking-[0.14em] text-forest">New customer</p>
        <h1 className="text-[2.6rem] leading-none">Create an account</h1>
        <p className="mt-3 text-muted">Tell us how to reach you, then finish with Google. You only need this when you place an order.</p>
        <form className="mt-5 grid gap-3" onSubmit={(event) => event.preventDefault()}>
          <Field label="Your name">
            <input className={inputClass} value={name} onChange={(event) => setName(event.target.value)} placeholder="As we should call you" required />
          </Field>
          <Field label="Phone">
            <input className={inputClass} value={phone} onChange={(event) => setPhone(event.target.value)} placeholder="MTN or Airtel number" required />
          </Field>
          <Field label="Area in Kabale">
            <input className={inputClass} value={area} onChange={(event) => setArea(event.target.value)} placeholder="Makanga, Central, Kirigime…" required />
          </Field>
        </form>
        <p className="mt-4 text-sm text-muted">Google proves the account. We keep the phone and area for delivery and the kitchen.</p>
        <GoogleButton
          onToken={(idToken) => {
            if (!name.trim() || !phone.trim() || !area.trim()) {
              setError('Add your name, phone, and area first.')
              return
            }
            remember()
            pending.current = true
            void signInWithGoogle(idToken)
              .then((signedIn) =>
                updateProfile({ name: name.trim(), phone: phone.trim(), area: area.trim() }).then(() => signedIn),
              )
              .then((signedIn) => {
                clearJoinProfile()
                navigate(destinationFor(signedIn), { replace: true })
              })
              .catch((err: Error) => setError(err.message))
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
