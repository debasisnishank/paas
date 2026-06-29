# ADR-0004: Config-driven jurisdiction profiles, never a code fork

- **Status:** Accepted
- **Date:** 2026-06-29
- **Deciders:** Founding team

## Context

The platform is India-first but explicitly designed to expand to MEA and beyond
(UAE → KSA → Nigeria → South Africa, then cloud regions). Every jurisdiction differs
on the axes that touch our code:

- **Data residency** — none / soft (SCC-style transfers) / hard (must stay
  in-country, e.g. KSA, payment data).
- **Breach authority + window** — DPB (72h) / CERT-In (6h) / SDAIA / DIFC …
- **Payment gateway** — Razorpay (IN) / PayTabs/Telr (GCC) / Paystack/Flutterwave
  (Africa).
- **Tax model + e-invoicing** — GST/IRP (IN) / ZATCA Fatoora (KSA 15% VAT) / …
- **Currency.**

The naive approach — a code branch or `if country == "KSA"` scattered across
services — produces an unmaintainable thicket and makes every new country a
release-blocking engineering project. It also makes residency violations (a fineable
event under SAMA/SDAIA/DPB) easy to introduce by omission.

## Decision

Encode all jurisdiction variation as **data, not control flow**. Each region carries
a `JurisdictionProfile` (defined in `services/api/internal/domain/types.go`):
`country_code`, `residency_rule`, `transfer_mechanism`, `breach_authority`,
`breach_window_hrs`, `payment_gateway`, `currency`, `tax_model`, and
`data_class_pinning`.

**Adding a country = adding a profile row + (at most) two adapters** — a payment
gateway adapter and a tax/invoice adapter behind stable interfaces. **Never** a new
service, a new Nomad job, or a code branch.

Two enforcement rules make this real:

1. **Data-class placement is decided at write time**, by intersecting
   (`DataClass` × `JurisdictionProfile`) — not audited after the fact. Every record
   is tagged `personal | sensitive | payment | telemetry`; the placement engine,
   not a human, decides where it may live.
2. **Caches are residency-aware**: public/static assets cache globally; anything
   tagged `personal | sensitive | payment` is `no-store` at foreign PoPs.

## Consequences

- **Easier:** new-country onboarding is mostly configuration + two small adapters;
  compliance logic lives in one place and is auditable.
- **Harder:** the abstractions (placement engine, gateway/tax adapter interfaces)
  must be right early, before we have many countries to generalize from. Getting the
  profile schema wrong is expensive to migrate.
- **Accepted:** some regulatory edge cases may not fit cleanly into the profile
  schema; when that happens we extend the *schema*, not fork the *code*. If a case
  truly cannot be expressed as data, that warrants a new ADR — not an inline branch.
- **Risk if violated:** a placement bug is a residency breach (regulatory fines +
  trust loss). This is why placement is write-time and non-optional (`CLAUDE.md`
  warning #2).

## Alternatives considered

- **Per-country code branches / forks** — diverges immediately, unmaintainable,
  unauditable. The thing this ADR exists to forbid.
- **Per-country microservices** — isolates logic but multiplies operational surface
  and still duplicates the shared 90%. Rejected.
- **Audit-after placement** (write anywhere, reconcile later) — guarantees periodic
  violations of hard-residency rules. Unacceptable for `payment`/`sensitive` data.

## Related

- Domain model: `JurisdictionProfile`, `DataClass`, `ResidencyRule` in
  `services/api/internal/domain/types.go` (these types **do** exist today).
- Billing adapter interface + tax profiles: `services/billing` (planned).
- MEA rollout sequence and the placement-engine warning: `CLAUDE.md`.
- Reality: the placement engine is Phase-4; the profile *types* exist now — see
  `docs/STATUS.md`.
