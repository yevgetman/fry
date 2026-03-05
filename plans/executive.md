# Executive Context

> **This file is optional.** If present, its content is injected into the top
> of every sprint prompt to give the AI agent high-level orientation about
> your project. If you delete this file, everything still works — the agent
> just won't have executive-level framing.
>
> Replace this entire document with your project's executive context before
> running `./fry.sh`, or simply delete it if you don't need it.

## What goes here

This file answers "why does this project exist and who is it for?" at a
level a new team member or executive stakeholder would read. It is **not**
for implementation details — those belong in `plans/plan.md`.

The content here is injected verbatim into the prompt with an explicit
instruction: "This is for orientation only — do NOT derive implementation
decisions from this section." So keep it concise and strategic.

## Good executive context includes

- **Project name and one-sentence description**
- **Problem statement** — what pain point or opportunity does this address?
- **Target users** — who will use this and how?
- **Business model** — how does it make money (if applicable)?
- **Success metrics** — how do you measure whether the project is working?
- **Key constraints** — timeline, budget, compliance, team size, etc.
- **Non-goals** — what is explicitly out of scope?

## What does NOT belong here

- Database schemas, API endpoints, function signatures → `plans/plan.md`
- Coding rules, framework choices, testing patterns → `AGENTS.md`
- Sprint tasks, iteration counts, build steps → `epic.md`

## Example (abbreviated)

---

### TaskFlow — Executive Brief

**What:** A task management API for small engineering teams that integrates
with Slack and GitHub.

**Problem:** Engineers track work across 3-5 tools. Context switching between
them wastes 30-60 min/day. TaskFlow consolidates task state into a single
API that syncs bidirectionally with existing tools.

**Users:** Engineering teams of 5-20 at seed-to-Series-A startups who already
use Slack and GitHub but haven't adopted a heavyweight PM tool.

**Business model:** Free for up to 5 users, $8/user/month after that.
Self-serve signup, no sales team.

**Success metrics:**
- Sub-100ms p95 API latency
- 99.9% uptime
- 60% weekly active retention after onboarding

**Constraints:**
- Solo developer for Phase 1, hire after launch
- Must be deployable to a single $20/month VPS (no Kubernetes)
- GDPR-compliant data handling from day one

**Non-goals (Phase 1):**
- No mobile app
- No real-time collaboration (WebSocket)
- No custom workflow builders

---

## Next steps

1. Replace this file with your project's executive context, or delete it
2. Your build plan goes in `plans/plan.md`
3. Run `./fry.sh epic.md` to start
