# Sprint 015 Merge Notes

## Codex Draft Strengths

- Correctly scoped Sprint 015 as an offline, Cruise-Director-operated
  checkout flow rather than online payment processing.
- Reused Sprint 013 foundations: USD catalog pricing, FX integer
  conversion, exchange-rate snapshots, and `folio_charge` movement
  semantics.
- Reused Sprint 014 foundations: `trip_guests`, manifest access, and
  assigned Cruise Director scoping.
- Chose basis points for card transaction fee storage.
- Kept closed folios immutable and deferred void/refund complexity.
- Added email template work for sending the final folio.

## Claude Code Critique Strengths

- Identified that sequential `AdjustStock` calls would not be atomic
  with folio close.
- Correctly rejected a client-controlled `apply_card_fee` toggle.
- Correctly rejected "GET creates folio" API semantics.
- Flagged the ambiguity between Sprint 013 `checkout_quotes` and new
  settlement folios.
- Found schema issues around inline partial unique constraints, tip
  quantity, denormalized line fields, and undefined currency sync.
- Added concrete testing requirements for stock rollback, duplicate
  close, email resend, and tenant/guest scoping.

## Valid Critiques Accepted

- Folio close must be a single store transaction that locks the folio,
  recomputes totals, decrements all counted inventory, marks closed, and
  commits or rolls back as one unit.
- `apply_card_fee` is removed. Card fee is automatic when payment method
  is card and org fee basis points are non-zero.
- `GET /folio` is read-only. `POST /folio` explicitly opens the folio.
- `checkout_quotes` remains a general quote snapshot/dev helper; guest
  folios are the authoritative closed settlement record and do not
  depend on `checkout_quotes`.
- `guest_folio_lines` does not denormalize `trip_id` or
  `trip_guest_id`; scope is enforced by joining through `guest_folios`.
- Tip line quantity is always 1 and enforced.
- `folio_email_from_name` is removed from Sprint 015.
- `custom` line type is removed from Sprint 015.
- The close path must load/lock the trip and boat context for inventory
  movements.
- Email failure is non-blocking after DB close; status is recorded and
  resend is available.

## Critiques Rejected or Modified

- Resend email was kept in Sprint 015 despite the simplification
  suggestion because a closed folio email is a core user-visible
  artifact and SMTP failure needs an operator recovery path.
- Org Admin operation of checkout was kept for support/admin override
  on any org trip. Cruise Director remains assigned-trip-only.
- Rate readiness hints stay in Payments but are satisfied by extending
  the payment settings GET response with current rate status; no
  separate endpoint is required.

## Interview Refinements Applied

- Checkout is exactly one end-of-trip event per guest/trip. The schema
  enforces one folio per `trip_guest`, not just one open folio.
- Tips are optional and entered by the Cruise Director only if the guest
  asks to add one. There is no guest-facing tip-entry screen in Sprint
  015.
- Initial payment methods are `card`, `cash`, and `other`.
- Liveaboard records that the guest paid by closing the folio. It does
  not track POS confirmation, transaction IDs, or payment references.
- If org-level card transaction fees are configured and the payment
  method is card, the fee is applied automatically. Site Directors
  cannot waive it.
- Directors can modify item quantities before close to correct errors,
  such as charging five beers instead of six.

## Final Decisions

- Create `organization_payment_settings` with supported currencies,
  default settlement currency, enabled payment methods, card fee basis
  points, and folio email footer.
- Create `guest_folios` and `guest_folio_lines` with one folio per
  `trip_guest`.
- Close folio with server-side totals only; no client totals and no
  client fee toggle.
- Keep payment processing offline and store no card/POS-sensitive data.
- Snapshot card fee, FX rate, settlement currency, totals, lines, actor,
  and timestamps on close.
- Send folio email after close; email failure does not reopen or roll
  back a paid folio.
