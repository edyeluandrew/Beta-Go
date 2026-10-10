# Beta-Go

Beta-Go is its own platform. The repository is [edyeluandrew/Beta-Go](https://github.com/edyeluandrew/Beta-Go).

Food marketplace for Kabale. Customers order from existing kitchens and pay with MTN MoMo or Airtel Money. Vendors cook and arrange pickup or their own delivery. Beta-Go takes a 10% commission from a prepaid vendor pool.

This repo is the build. The working specification is in `docs/`:

- [Architecture](docs/architecture.md)
- [Workflows](docs/workflows.md)
- [API](docs/api.md)
- [Rules](docs/rules.md)
- [Launch prep](docs/launch.md)

The original Word file stays in the repo root. Screens for a phase are added only after that phase's endpoints pass their tests. Customer, vendor, and admin screens are in `apps/web/src/features`. The server still enforces every role.

## Stack

- React (Vite) in `apps/web`, Tailwind CSS, modern ES modules
- Go and Gin in `apps/api`
- Neon Postgres, connected with `DATABASE_URL`
- Google sign-in. Phone OTP stays out until you choose a paid SMS service.

## How the build is organised

Work moves in phases. Each phase gets its own migration, its own API package, and its own React feature folder. Do not fold the next phase into an old file.

| Phase | What it delivers | Where it goes | State |
| --- | --- | --- | --- |
| 1. Identity | Google sign-in, roles, sessions | `internal/auth`, `internal/identity`, `features/auth` | In place |
| 2. Vendors | Admin creates the account, vendor builds the menu | `internal/vendor` | Endpoints built and tested |
| 3. Marketplace | Browse active kitchens, one vendor per order | `internal/order` | Endpoints built and tested |
| 4. Payments | Server-verified payment result. Model A: customer pays the kitchen | `internal/payment` | Endpoints built |
| 5. Commission | Prepaid pool, reserve, capture, release, UGX 10,000 floor | `internal/commission` | Endpoints built |
| 6. Operations | Accept, reject, fulfilment, refunds, disputes | `internal/ops` | Endpoints built and tested |
| 7. Launch prep | Audit, backups, security checks | `internal/launch` | Endpoints built and tested |

Routes are registered in `apps/api/internal/platform/httpserver/server.go`. That file is the map. New behaviour is a new route group there, not a new framework.

### Adding the next piece

1. Add `apps/api/migrations/00N_name.sql`. The API applies new files on startup and never re-runs an old one.
2. Add `apps/api/internal/<feature>` with a store, a service, and a handler.
3. Register the routes in `server.go`, behind the role check.
4. Add the screen under `apps/web/src/features/<feature>` after those endpoints pass.

Money amounts are whole shillings. The server decides prices, payment success, and commission. The screen only displays them.

## Roles

Signing in with Google opens a **customer** account. Nobody can grant themselves another role.

`BOOTSTRAP_ADMIN_EMAILS` in `.env` gives those Google emails **super_admin** as well. Vendor is granted only by invite accept. Support and finance are granted by a super_admin on `/api/v1/admin/launch/staff`. Nobody grants themselves super_admin. The server is the RBAC enforcer.

## Run it locally

1. Copy `.env.example` to `apps/.env`.
2. Paste the Neon pooled connection string into `DATABASE_URL`.
3. In Google Cloud, create an OAuth **Web** client. Put `http://localhost:5173` in Authorized JavaScript origins. Paste the client ID into both `GOOGLE_CLIENT_ID` and `VITE_GOOGLE_CLIENT_ID`.
4. Put your Gmail in `BOOTSTRAP_ADMIN_EMAILS` if you need the admin workspace.
5. From the repo root:

```bash
make run
make web
```

On this Windows machine, if `make` is not on PATH, use `.\make.cmd run` from the repo root. Same targets.

Open `http://localhost:5173`. The first sign-in creates your customer, and your bootstrap email also gets super admin.

`make test` runs the Go tests. If Windows blocks the test binaries, use `make test-docker`. `make build` typechecks and builds the React app.
