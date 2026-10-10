# Platform architecture

Source: BETA-GO PRODUCT AND TECHNICAL SPECIFICATION V2.docx, 10 October 2026. This file is the working copy for the build. Launch market is Kabale, Uganda. The product repository is https://github.com/edyeluandrew/Beta-Go.

Beta-Go is a food marketplace. Customers order from existing kitchens. Vendors cook and arrange collection or their own delivery. Beta-Go is not a restaurant and does not employ riders. It earns 10% of the food subtotal from a prepaid vendor commission pool.

## Parties

- **Customer** browses, orders, pays, tracks, confirms receipt, and raises disputes.
- **Vendor** keeps the menu, prices, photos, and hours; accepts or rejects paid orders; prepares the food; arranges collection or delivery; keeps a verified payment destination and a funded commission pool.
- **Beta-Go** runs the software, records orders, talks to the payment provider, calculates commission, and keeps the audit trail.
- **Administrator** is an internal role, not a marketplace party. Admins create vendor accounts. They do not build menus. Support staff cannot make financial adjustments.

## Stack

- React in `apps/web`. Screens wait until the API for that phase is tested.
- Go and Gin in `apps/api`. The server is the only enforcer of price, payment, role, and commission rules.
- Neon Postgres. Amounts are whole shillings. A balance check outside a database transaction is not trusted.
- Google sign-in for this build. Phone OTP stays out while it is a paid SMS service. A delivery order will still store a reachable phone number later.

## How a request is trusted

1. Google proves the person. The API checks that proof and opens a customer account. Vendor and staff roles are assigned by a controlled process, never by the caller.
2. The API issues its own session. Access is a short-lived cookie. Refresh rotates. Logout revokes the session immediately.
3. Every later request checks the session, the role, and ownership of the record. A valid cookie is not enough to touch another vendor's shop.

## Money stays in three separate streams

These are never added together in a report:

1. Customer food payment, from the customer to the vendor's MTN or Airtel number.
2. Commission pool movements: top-ups, reservations, captures, releases.
3. Beta-Go commission revenue, which is captures only.

A food payment is never a pool top-up. A reservation is never revenue.

## Payment model A

Model A was chosen on 10 October 2026. The customer pays the kitchen. Beta-Go's 10% comes from the kitchen's prepaid pool. Model B stays on the record as the fallback if a live MTN or Airtel test cannot route, verify, and refund a payment that already landed on the vendor's number.

| | Model A, chosen | Model B, fallback |
| --- | --- | --- |
| Customer pays | The vendor's MoMo or Airtel destination | Beta-Go's collection account |
| Commission | Prepaid pool | Deducted at settlement |
| Refunds | Depend on the provider and the vendor | Beta-Go controls them |

Before any live payment, confirm in writing and with a live test that the provider can route a payment, verify it on the server, and refund it, and who is allowed to start that refund.

Payment success is only ever confirmed on the server. Screenshots, redirects, and client-supplied statuses do not count.

## Where code lives

| Concern | API package | Database |
| --- | --- | --- |
| Sign-in, sessions, roles | `internal/auth`, `internal/identity` | `001_identity.sql` |
| Vendors, menus, invitations | `internal/vendor` | `002_vendors.sql` |
| Catalog and orders | `internal/order` | `003_orders.sql` |
| Payments and refunds | `internal/payment` | `004_payments.sql` |
| Commission pool and ledger | `internal/commission` | `005_commission.sql` |
| Fulfilment, refunds, disputes | `internal/ops` | `006_operations.sql` |
| Launch prep, audit, staff grant | `internal/launch` | `007_launch.sql` |

Routes are registered in `apps/api/internal/platform/httpserver/server.go`. A new area is a new route group there. An old migration file is never edited after it has been applied.
