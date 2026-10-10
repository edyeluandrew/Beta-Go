# API

Base path `/api/v1`. Every route checks role and ownership on the server. Amounts are integers in UGX. Errors look like `{ "error": { "code", "message" } }`.

The foundation routes below are implemented and covered by `apps/api/internal/platform/httpserver/endpoints_test.go`. Later routes are the contract from the specification. Each one is built and tested in its phase before a screen is added for it.

## Foundation, phase 1

| Method | Path | Who | State |
| --- | --- | --- | --- |
| GET | `/healthz` | Public | Built |
| GET | `/readyz` | Public, checks Neon | Built |
| POST | `/api/v1/auth/google` | Public | Built. Body `{ "idToken" }`. Creates a customer. Bootstrap emails also receive super_admin. |
| POST | `/api/v1/auth/refresh` | Refresh cookie | Built. Rotates the refresh token. |
| POST | `/api/v1/auth/logout` | Refresh cookie | Built. Revokes the session. |
| GET | `/api/v1/me` | Signed in | Built |
| GET | `/api/v1/customer/home` | Customer | Built as the role gate. Marketplace data arrives in phase 3. |
| GET | `/api/v1/vendor/home` | Vendor | Built as the role gate. Shop data arrives in phase 2. |
| GET | `/api/v1/admin/home` | support, finance, or super_admin | Built as the role gate. |

Cookies: `bg_access` for API calls, `bg_refresh` only on `/api/v1/auth`. Both are httpOnly.

## Vendors, phase 2

Built and covered by `vendor_endpoints_test.go`. The invitation is bound to the vendor's Google email, because sign-in is Google. The admin receives the token once and gives it to the vendor. A shop photo is a URL for now. File upload is still to come. Hiding a menu item is the vendor's availability flag. Admin unpublish of a single item is still to come.

| Method | Path | Who |
| --- | --- | --- |
| POST | `/api/v1/admin/vendors` | support, super_admin |
| GET | `/api/v1/admin/vendors` and `/:id` | support, super_admin |
| PATCH | `/api/v1/admin/vendors/:id` | support, super_admin |
| POST | `/api/v1/admin/vendors/:id/invitations` | support, super_admin |
| POST | `/api/v1/admin/vendors/:id/destination` and `/destination/verify` | support, super_admin |
| POST | `/api/v1/admin/vendors/:id/approve` | support, super_admin. Refuses until 3 priced items, a shop photo, hours, and a verified destination exist. Sets INACTIVE while the pool is under UGX 10,000. |
| POST | `/api/v1/admin/vendors/:id/reject`, `/pause`, `/suspend`, `/reactivate` | support, super_admin |
| POST | `/api/v1/vendor/invitations/accept` | Signed-in Google user matching the invite |
| GET, PATCH | `/api/v1/vendor/shop` | vendor |
| PUT | `/api/v1/vendor/hours` | vendor |
| POST | `/api/v1/vendor/categories` and `/items` | vendor |
| PATCH | `/api/v1/vendor/items/:id` | vendor |

A vendor cannot change the payment destination or the commission rate.

Public read is built. Inactive, paused, and suspended shops are excluded. Payment destinations and contact emails are never public.

| Method | Path | Who |
| --- | --- | --- |
| GET | `/api/v1/vendors` | Public |
| GET | `/api/v1/vendors/:id` | Public. Menu, hours, and pickup address. |

## Orders, phase 3 and 6

Phase 3 places the order. Phase 6 runs it after payment. Phase 6 endpoints are locked: accept, reject, fulfilment, confirm, cancel rules, disputes, and reconcile passed their tests. A refund being due is not the same as the provider sending the money back.

| Method | Path | Who |
| --- | --- | --- |
| POST | `/api/v1/orders` | customer |
| GET | `/api/v1/orders` and `/:id` | the customer who placed it |
| POST | `/api/v1/orders/:id/cancel` | customer. Unpaid: close. Paid, not accepted: close and mark refund. After accept, before preparing: request. After preparing: refused. |
| POST | `/api/v1/orders/:id/confirm` | customer, from FULFILLED. Captures commission. |
| POST | `/api/v1/orders/:id/disputes` | customer, from FULFILLED within 24 hours |
| GET | `/api/v1/vendor/orders` and `/:id` | vendor |
| POST | `/api/v1/vendor/orders/:id/accept` | vendor. Reserves commission. |
| POST | `/api/v1/vendor/orders/:id/reject` | vendor, while waiting. Reason `VENDOR_UNABLE`. Marks refund. |
| POST | `/api/v1/vendor/orders/:id/preparing`, `/ready`, `/fulfilled` | vendor |
| POST | `/api/v1/vendor/orders/:id/cancel` | vendor after accept, if they cannot fulfil. Releases commission and marks refund. |
| POST | `/api/v1/vendor/orders/:id/cancel-approve` | vendor, if the customer asked before preparing |
| POST | `/api/v1/admin/orders/:id/disputes/resolve` | support, finance, super_admin. Outcome `COMPLETED` or `CANCELLED`. |
| POST | `/api/v1/admin/orders/reconcile` | support, finance, super_admin. Expires a 10-minute silence, auto-completes 2 hours after fulfilled, escalates a 48-hour dispute. |

## Payments, phase 4

The customer can start MTN or Airtel collection. The payment stays PENDING until a signed provider notice says SUCCESS. A status sent by the customer is ignored. Repeating the same notice does nothing. After 15 minutes an unpaid order expires. A success that arrives after cancel or expiry is marked for refund and does not move the order forward. Live MTN and Airtel are not connected. The routes stay closed until `PAYMENT_PROVIDER=sandbox` and `PAYMENT_WEBHOOK_SECRET` are set, and the sandbox is never used in production.

| Method | Path | Who |
| --- | --- | --- |
| POST | `/api/v1/orders/:id/payment` | customer. Body is `provider` (`mtn` or `airtel`) and `phone`. |
| GET | `/api/v1/orders/:id/payment` | the customer who placed the order |
| POST | `/api/v1/payments/webhook` | the provider. Header `X-Payment-Signature` is required. |
| POST | `/api/v1/admin/payments/reconcile` | finance, super_admin. Asks the provider. Silence leaves the payment pending. |

## Commission, phase 5

Model A. The food payment never credits this pool. A screenshot never credits this pool. Posted ledger rows are never edited. Available never goes below zero. Below UGX 10,000 available the shop becomes INACTIVE. A verified top-up back to UGX 10,000 or more returns an INACTIVE shop to ACTIVE. PAUSED and SUSPENDED stay as they are.

Reserve, capture, and release each succeed only once for an order. Accept will call reserve in phase 6. Completion will call capture. A valid reject or cancel after accept will call release.

| Method | Path | Who |
| --- | --- | --- |
| GET | `/api/v1/vendor/commission` | vendor. Available, reserved, and a low-balance flag at UGX 20,000. |
| GET | `/api/v1/vendor/commission/ledger` | vendor |
| POST | `/api/v1/vendor/commission/top-ups` | vendor. Amount UGX 10,000 to 1,000,000. Body is `amountUgx`, `provider`, `phone`, `idempotencyKey`. |
| GET | `/api/v1/vendor/commission/top-ups/:id` | vendor |
| POST | `/api/v1/commission/top-ups/webhook` | the provider. Header `X-Payment-Signature` is required. |
| GET | `/api/v1/admin/vendors/:id/commission` | finance, super_admin |
| POST | `/api/v1/admin/vendors/:id/commission/adjustments` | finance, super_admin. Reason required. Above UGX 50,000 a second person must approve. |
| POST | `/api/v1/admin/commission/adjustments/:id/approve` | finance, super_admin. Not the person who requested it. |

A paid reject or valid cancel marks the payment `REFUND_PENDING`. Provider refund confirmation is still a separate step.

## Launch prep, phase 7

Built and covered by `launch_endpoints_test.go`. The server is the RBAC enforcer. These routes are `super_admin` only. Support cannot grant roles. Finance cannot grant roles. The security report never returns `DATABASE_URL`, `SESSION_SECRET`, or the webhook secret. Vendor is still granted only by accepting an invite. `super_admin` is not granted here.

The person who received a staff role must sign in again, or refresh, before the new role is in the access cookie.

| Method | Path | Who |
| --- | --- | --- |
| GET | `/api/v1/admin/launch/security` | super_admin. Flags only: env, cookie secure, Google configured, payment provider name, whether secrets are set, whether the session secret is still the example. |
| GET | `/api/v1/admin/launch/audit` | super_admin. Last 100 append-only rows. |
| GET | `/api/v1/admin/launch/backups` | super_admin. Recorded Neon recovery checks. |
| POST | `/api/v1/admin/launch/backups` | super_admin. Body `{ "note" }`. Confirms a recovery check. Does not dump the database. |
| POST | `/api/v1/admin/launch/staff` | super_admin. Body `{ "email", "role" }`. Role is `support` or `finance` only. The person must already have signed in with Google. |
