import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { homeFor, useAuth } from '../../shared/auth/AuthProvider'
import { ErrorText } from '../../shared/ui/widgets'
import { destinationFor, saveNextPath } from './afterSignIn'
import { GoogleButton } from './GoogleButton'

export function SignInPage() {
  const { user, ready, signInWithGoogle } = useAuth()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const [error, setError] = useState('')
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
        <p className="mt-3 text-muted">Use the same Google account you created with. That opens your orders and dashboard.</p>
        <GoogleButton
          onToken={(idToken) => {
            pending.current = true
            void signInWithGoogle(idToken)
              .then((signedIn) => navigate(destinationFor(signedIn), { replace: true }))
              .catch((err: Error) => setError(err.message))
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
