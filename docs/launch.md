# Launch prep

Phase 7. Endpoints are built and tested. Screens come next.

## Secrets

Secrets live in `apps/.env`. That file is gitignored. Copy `.env.example` and fill it in. Never paste `DATABASE_URL`, `SESSION_SECRET`, or `PAYMENT_WEBHOOK_SECRET` into source, screens, or chat.

`GOOGLE_CLIENT_ID` and `VITE_GOOGLE_CLIENT_ID` are the public OAuth client id, not a secret. The client secret stays in Google Cloud and is not used by this app.

`GET /api/v1/admin/launch/security` reports flags only. It never returns the database URL, session secret, or webhook secret.

## RBAC

The server checks the session and the role on every route. A screen hide is not the gate.

| Role | How it is granted | What it can do here |
| --- | --- | --- |
| customer | Public Google sign-in | Marketplace |
| vendor | Invite accept only | Shop, kitchen, own pool |
| support | Super admin grant | Vendors, disputes. Not money. Not grants. |
| finance | Super admin grant | Payments, pool adjustments. Not grants. |
| super_admin | `BOOTSTRAP_ADMIN_EMAILS` only | Launch security, audit, backup check, staff grant |

`POST /api/v1/admin/launch/staff` accepts `support` or `finance` only. It refuses `vendor` and `super_admin`.

## Backups

Neon holds the data and point-in-time recovery. Beta-Go does not dump the database into the repo.

Before launch, in the Neon console, confirm that recovery is on and that you can restore a recent point. Then record that check with `POST /api/v1/admin/launch/backups`. The note is an audit, not a backup file.

## Production boot

`ENV=production` refuses to start when any of these are true:

- `COOKIE_SECURE` is false
- `SESSION_SECRET` is still the example / `dev-only` value
- `PAYMENT_PROVIDER=sandbox`
- `WEB_ORIGIN` still points at localhost

Development may keep those values.
