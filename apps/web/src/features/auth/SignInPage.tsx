import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { homeFor, useAuth } from '../../shared/auth/AuthProvider'
import { ErrorText, Field, inputClass, PrimaryButton } from '../../shared/ui/widgets'
import { destinationFor, saveNextPath } from './afterSignIn'
import { GoogleButton } from './GoogleButton'

export function SignInPage() {
  const { user, ready, signInWithGoogle, signInWithPassword } = useAuth()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const pending = useRef(false)

  useEffect(() => {
    const next = params.get('next')
    if (next) saveNextPath(next)
  }, [params])

  useEffect(() => {
    if (ready && user && !pending.current) {
      const next = params.get('next')
      if (next) saveNextPath(next)
      navigate(destinationFor(user) || homeFor(user), { replace: true })
    }
  }, [navigate, params, ready, user])

  return (
    <main className="grid min-h-screen place-items-center bg-[radial-gradient(circle_at_top_left,#f4c48a_0%,transparent_28%),radial-gradient(circle_at_bottom_right,#9ad4b8_0%,transparent_32%),#f6efe4] px-4 py-6">
      <section className="w-full max-w-[440px] rounded-[20px] border border-line bg-card px-6 pb-6 pt-7 shadow-[0_16px_40px_rgb(26_20_16/8%)]">
        <p className="mb-2 text-xs font-semibold uppercase tracking-[0.14em] text-forest">Welcome back</p>
        <h1 className="text-[2.6rem] leading-none">Sign in</h1>
        <p className="mt-3 text-muted">Use the email and password you created with, or the same Google account.</p>
        <form
          className="mt-5 grid gap-3"
          onSubmit={(event) => {
            event.preventDefault()
            setError('')
            if (!email.trim() || !password) {
              setError('Enter your email and password.')
              return
            }
            pending.current = true
            setBusy(true)
            void signInWithPassword(email.trim(), password)
              .then((signedIn) => navigate(destinationFor(signedIn), { replace: true }))
              .catch((err: Error) => {
                pending.current = false
                setError(err.message)
              })
              .finally(() => setBusy(false))
          }}
        >
          <Field label="Email">
            <input className={inputClass} value={email} onChange={(event) => setEmail(event.target.value)} autoComplete="email" inputMode="email" />
          </Field>
          <Field label="Password">
            <input className={inputClass} value={password} onChange={(event) => setPassword(event.target.value)} type="password" autoComplete="current-password" />
          </Field>
          <PrimaryButton type="submit" className="w-full" disabled={busy}>
            {busy ? 'Signing in…' : 'Sign in'}
          </PrimaryButton>
        </form>
        <p className="mt-5 text-center text-sm text-muted">or continue with Google</p>
        <GoogleButton
          onToken={(idToken) => {
            pending.current = true
            void signInWithGoogle(idToken)
              .then((signedIn) => navigate(destinationFor(signedIn), { replace: true }))
              .catch((err: Error) => {
                pending.current = false
                setError(err.message)
              })
          }}
          onError={setError}
        />
        <ErrorText>{error}</ErrorText>
        <p className="mt-5 text-sm text-muted">
          First time here?{' '}
          <Link className="text-forest" to="/join">
            Create an account
          </Link>
          {' · '}
          <Link className="text-forest" to="/c">
            Browse kitchens first
          </Link>
        </p>
      </section>
    </main>
  )
}
