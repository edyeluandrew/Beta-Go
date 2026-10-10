import { useEffect, useRef } from 'react'

export function GoogleButton({
  onToken,
  onError,
}: {
  onToken: (idToken: string) => void
  onError: (message: string) => void
}) {
  const buttonRef = useRef<HTMLDivElement>(null)
  const onTokenRef = useRef(onToken)
  const onErrorRef = useRef(onError)
  const clientId = import.meta.env.VITE_GOOGLE_CLIENT_ID
  onTokenRef.current = onToken
  onErrorRef.current = onError

  useEffect(() => {
    if (!clientId || !buttonRef.current) return

    let cancelled = false
    let started = false
    const mount = () => {
      if (cancelled || started || !buttonRef.current || !window.google?.accounts?.id) return false
      started = true
      buttonRef.current.innerHTML = ''
      window.google.accounts.id.initialize({
        client_id: clientId,
        callback: (response) => {
          const idToken = response.credential
          if (!idToken) {
            onErrorRef.current('Google did not return a sign-in token.')
            return
          }
          onTokenRef.current(idToken)
        },
      })
      window.google.accounts.id.renderButton(buttonRef.current, {
        theme: 'outline',
        size: 'large',
        width: buttonRef.current.clientWidth || 280,
        text: 'continue_with',
        shape: 'pill',
      })
      return true
    }

    if (!mount()) {
      const timer = window.setInterval(() => {
        if (mount()) window.clearInterval(timer)
      }, 200)
      return () => {
        cancelled = true
        window.clearInterval(timer)
      }
    }
    return () => {
      cancelled = true
    }
  }, [clientId])

  if (!clientId) {
    return <p className="mt-4 text-muted">Add VITE_GOOGLE_CLIENT_ID to apps/.env, then restart the web app.</p>
  }
  return <div className="mt-5 min-h-11 w-full" ref={buttonRef} />
}
