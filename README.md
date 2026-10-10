# Beta-Go

Kabale food marketplace. Customers order from kitchens that are already cooking — chips, chicken, pork, burgers, pizza — and pay that shop on MTN or Airtel. Beta-Go takes 10% from a prepaid vendor pool. We do not cook, and we do not run riders.

Repo: [edyeluandrew/Beta-Go](https://github.com/edyeluandrew/Beta-Go)

## What’s in here

- `apps/web` — React (Vite, Tailwind)
- `apps/api` — Go / Gin
- `docs/` — architecture, API, money rules, launch notes
- Neon for Postgres
- Google for sign-in. No phone OTP in this build.

The Word spec stays in the repo root. Amounts are whole Uganda shillings. The API sets prices, payment results, roles, and commission. The browser only shows what the server already decided.

## Run it

Copy `.env.example` to `apps/.env` and fill it in.

1. Neon pooled `DATABASE_URL`
2. A Google OAuth **Web** client. Authorized JavaScript origin: `http://localhost:5173`. Same id in `GOOGLE_CLIENT_ID` and `VITE_GOOGLE_CLIENT_ID`
3. `SESSION_SECRET` — at least 32 characters
4. `BOOTSTRAP_ADMIN_EMAILS` — the Gmail that should also be super_admin

Then, from the repo root:

```bash
make run
make web
```

or:

```bash
npm run dev:api
npm run dev:web
```

- App: http://localhost:5173
- API: http://localhost:8080

Guests can browse kitchens. An account is only required to place an order. Google sign-in creates a customer. A vendor gets in through an invite, not by picking a role.

```bash
make test
make test-docker
make build
```

## Docs

- [Architecture](docs/architecture.md)
- [Workflows](docs/workflows.md)
- [API](docs/api.md)
- [Rules](docs/rules.md)
- [Launch](docs/launch.md)

## Hosting

| Piece | Where | Root |
| --- | --- | --- |
| Frontend | Vercel | `apps/web` |
| API | Render | `apps/api` |
| Database | Neon | already in `DATABASE_URL` |

Do not put `apps/.env` on GitHub. Set the same keys in Vercel and Render.

On Vercel: `VITE_GOOGLE_CLIENT_ID`. Add the Vercel URL as a Google OAuth JavaScript origin.

On Render: `DATABASE_URL`, `GOOGLE_CLIENT_ID`, `SESSION_SECRET`, `BOOTSTRAP_ADMIN_EMAILS`, `ENV=production`, `COOKIE_SECURE=true`, `WEB_ORIGIN` = the Vercel URL.

After the API has a Render URL, add a Vercel rewrite so `/api/*` goes there. The browser then stays on one host and the session cookies work. Until that rewrite exists, the hosted site can render, but it cannot talk to the API.
