# Architecture Decision Records (ADRs)

An ADR captures **one decision**, the context that forced it, and the consequences
we accepted. It exists so the *why* survives — and so an LLM or a new engineer cites
the real reasoning instead of inventing one or silently re-litigating a settled bet.

## Rules

- One decision per file: `NNNN-short-kebab-title.md` (zero-padded, monotonic).
- Status is one of: `Proposed` · `Accepted` · `Superseded by ADR-XXXX` · `Deprecated`.
- ADRs are **append-only**. Don't rewrite an accepted decision — write a new ADR that
  supersedes it and flip the old one's status.
- Keep it short. Context → Decision → Consequences. No essays.
- The two load-bearing bets (ADR-0001, ADR-0002) are flagged "do not revisit lightly"
  in `CLAUDE.md`. These ADRs are *why*. Reopening them needs a superseding ADR with
  new evidence, not a casual code change.

## Index

| ADR | Title | Status |
|---|---|---|
| [0001](0001-firecracker-microvms-on-nomad.md) | Firecracker microVMs scheduled by Nomad | Accepted |
| [0002](0002-neon-engine-own-control-plane.md) | Neon engine + our own control plane | Accepted |
| [0003](0003-go-rust-language-split.md) | Go for control plane, Rust for the hot path | Accepted |
| [0004](0004-config-driven-jurisdiction-profiles.md) | Config-driven jurisdiction profiles, never a code fork | Accepted |

## Template

```markdown
# ADR-NNNN: <title>

- **Status:** Proposed | Accepted | Superseded by ADR-XXXX | Deprecated
- **Date:** YYYY-MM-DD
- **Deciders:** <who>

## Context
What forces a decision now? Constraints, requirements, the problem.

## Decision
What we will do. State it plainly.

## Consequences
What becomes easier, what becomes harder, what we accept as the cost.

## Alternatives considered
What else we weighed and why we rejected it.
```
