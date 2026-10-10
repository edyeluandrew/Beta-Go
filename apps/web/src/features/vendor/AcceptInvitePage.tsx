import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ApiError } from '../../shared/api/client'
import { useAuth } from '../../shared/auth/AuthProvider'
import { Shell } from '../../shared/ui/Shell'
import { ErrorText, Field, PrimaryButton, inputClass } from '../../shared/ui/widgets'
import { acceptInvite } from './api'

export function AcceptInvitePage() {
  const { refresh, user } = useAuth()
  const navigate = useNavigate()
  const [token, setToken] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  return (
    <Shell title="Accept a kitchen invite" lede={`You are signed in as ${user?.email ?? '…'}. The invite only works if that is the Google email on the kitchen.`}>
      <form
        className="mt-6 grid w-full max-w-[440px] gap-3 rounded-[20px] border border-line bg-card p-6"
        onSubmit={(event) => {
          event.preventDefault()
          setBusy(true)
          setError('')
          acceptInvite(token.trim())
            .then(() => refresh())
            .then((user) => navigate(user.roles.includes('vendor') ? '/v' : '/c', { replace: true }))
            .catch((err: unknown) => setError(err instanceof ApiError ? err.message : 'That invite could not be used.'))
            .finally(() => setBusy(false))
        }}
      >
        <Field label="Invite token">
          <input className={inputClass} value={token} onChange={(event) => setToken(event.target.value)} required />
        </Field>
        <PrimaryButton type="submit" disabled={busy}>
          Accept invite
        </PrimaryButton>
        <ErrorText>{error}</ErrorText>
      </form>
    </Shell>
  )
}
