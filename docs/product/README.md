# Product Docs

These four documents are the source of truth for Meguru's product and engineering direction:

- [BRD.md](BRD.md) — business requirements, goals, milestones
- [PRD.md](PRD.md) — product requirements, user stories, feature scope
- [TECH_STACK.md](TECH_STACK.md) — language/framework choices, schema, architecture
- [CONSTITUTION.md](CONSTITUTION.md) — binding principles, security rules, threat model

Spec Kit derives its own artifacts *from* these docs — it does not duplicate them:

- `.specify/memory/constitution.md` is the spec-kit-formatted, binding derivative of `CONSTITUTION.md`.
- `specs/NNN-*/{spec,plan,tasks}.md` are per-feature specs authored from these docs, scoped to one milestone at a time.

If a spec-kit artifact and one of these docs ever disagree, treat the disagreement as a defect to resolve, not an ambiguity to route around.
