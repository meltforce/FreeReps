# FreeReps

Self-hosted server for health data: ingest from Apple Health, Oura and Alpha
Progression, storage in PostgreSQL/TimescaleDB, a web dashboard with freely
configurable correlations, and an MCP server. It computes no scores and does no
coaching — that is a decision, not a gap ([`DECISIONS.md`](DECISIONS.md)).

Monorepo. `server/` holds the Go binary `freereps` with the web UI embedded;
`app/` holds the iOS companion (`FreeReps.xcodeproj`). Build and run
instructions, and the wire-format specs every ingest change starts from, live in
`server/CLAUDE.md` and `app/CLAUDE.md`, which load only when working under those
paths.

## Gotchas

- **The backend does not compile without `server/web/dist`.** `server/web.go`
  embeds it via `go:embed`. Build the frontend first, or create the stub:
  `mkdir -p server/web/dist && touch server/web/dist/.gitkeep`. *Why:* a
  missing directory fails the Go build with an embed error that names the
  directive, not the cause.

- **A metric arriving from two sources needs a unit decision and a source
  filter.** Apple Health and Oura disagree on both units and granularity.
  *Why:* both classes of failure have already occurred and neither raised an
  error — see the 2026-03-25 and 2026-03-26 entries in
  [`INCIDENTS.md`](INCIDENTS.md).

- **What you learn about building or running goes into the `CLAUDE.md` next to
  the code**, not into this file. *Why:* `server/` and `app/` instructions load
  only for sessions working there; in this file they would load in every
  session.

## Repository & CI/CD

**Forgejo is the source of truth**: `git.coydog-fence.ts.net/meltforce.net/freereps`
(`origin`). `github.com/meltforce/FreeReps` is a push mirror — never push there
directly. The mirror is `git push --mirror`: it force-pushes *and* prunes refs
that don't exist on Forgejo, so anything that must survive on GitHub has to
exist on Forgejo first (a GitHub-only branch is deleted at the next sync).

| Where | What runs | Triggered by |
|---|---|---|
| `.forgejo/workflows/ci.yml` | Go build/vet/test/lint, frontend tsc + build, document contract check, then build → `git.coydog-fence.ts.net/meltforce.net/freereps:edge` → redeploy on `freereps-lxc` | push/PR on `main` |
| `.github/workflows/ios.yml` | Xcode build (no macOS runner exists on Forgejo) | mirror push |
| `.github/workflows/release.yml` | Docker Hub image + GitHub Release with `freereps-upload` binaries | tag push, carried over by the mirror |

Deploy runs through the shared reusable workflow
`meltforce.net/ci-workflows/.forgejo/workflows/build-push-deploy.yml@v4` with
`sync_compose: false` — **the deployed compose belongs to the homelab repo**
(`docker/stacks/freereps/compose.yaml`, plus the catalog entry in
`configuration/docker-stacks/stacks/freereps.yml`, which renders `.env` and
`config.yaml`). Change the image ref, ports or volumes there, not here.
`server/docker-compose.yml` is for local development only.

Runner labels: `docker` (normal jobs), `docker-buildx` (image builds), `host`
(runs on the runner LXC itself — needed for Tailscale SSH into deploy targets).
The Forgejo org `meltforce.net` already provides `REGISTRY_USER`,
`REGISTRY_TOKEN`, `DOCKERHUB_USERNAME`, `DOCKERHUB_TOKEN`; no repo secrets.

## Repo documents

These documents carry state over time. The axis is where a thing *is*, not what
it is about.

| File | Holds |
|---|---|
| `ROADMAP.md` | Open work only. Status token `[open]`. |
| `DECISIONS.md` | Decisions taken, including decisions not to do something — those are the ones most likely to be re-derived from scratch otherwise. |
| `INCIDENTS.md` | Postmortems for things that broke. Newest first. |

**The movement rule.** When an item closes it is *removed* from `ROADMAP.md`,
and its reasoning moves to whichever document above holds that kind of thing.
Nothing is struck through — a struck-through row is a row that should have been
moved. Status tokens are exactly `[open]`, `[done YYYY-MM-DD]`,
`[dropped YYYY-MM-DD]`; emoji never carry status.

**Before closing an item, read its entry for residual work, dates, or
triggers.** Each of those becomes its own `[open]` row before the entry leaves
the roadmap. This is the step that gets skipped, and skipping it is how
finished-looking work quietly loses its tail.

**No new top-level documents** unless the concern is genuinely orthogonal to the
ones above. Everything else lives inside a project directory.

**Operational exceptions never live in documentation.** Excluding a host from a
run, skipping a check for one case — those belong in configuration, in a
condition, in the inventory. Documentation may reference them; it may not
replace them.

Every rule written here carries a one-line **why**, so it can be revisited when
the context that produced it changes.

## Language

English is the only language used inside this repo. This is not a style
preference — German prose describing English identifiers forces a translation
layer ("Rolle" in the text, `role` in the YAML) and breaks keyword search
between an explanation and the code it explains.

Applies to every document, every code comment and docstring in every language
present, log messages, error strings, user-facing CLI output, identifiers
(variables, functions, roles, tags, unit names, secret paths), and commit
messages.

Number and date formats follow the English convention: `1.82 GB` (decimal
point), `217,226` (comma as thousands separator), `2026-08-04` (ISO 8601, never
`04.08.2026`).

The only exception is a verbatim quote of external output — an upstream error
message, vendor documentation — which keeps its original wording.

Conversation language is independent of this and follows the operator.

## Git workflow

Single developer. **`main` is the only long-lived branch** — commit straight to
`main`, never open a PR for this repo. This overrides the harness default of
branching before committing.

**Commit and push autonomously** once a coherent change is complete and
verified. No approval needed per commit.

**Stage explicitly, never `git add -A`.** Parallel sessions run in this same
checkout; a blanket add sweeps up their work in progress and commits it under
your message. Name the paths you touched.

### Parallel sessions

Isolate concurrent sessions with worktrees:

```bash
claude --worktree <name>          # .claude/worktrees/<name>, branch worktree-<name>
```

A worktree branch is **ephemeral plumbing, not a feature branch**. Never push it
as a branch, never open a PR from it. Land the work on `main`:

```bash
git fetch origin
git rebase origin/main
git push origin HEAD:main          # a rejection means another session landed first
```

Rebase and push again rather than forcing.

Do **not** start background sessions for work that edits this repo — a
background session commits, pushes its own branch and opens a draft PR without
asking, and is hard-wired never to push to `main`. Background sessions are fine
for read-only investigation.

**If a harness rule conflicts with this, this file wins.** `main` *is* the review
surface here and `git revert` is the undo. Say plainly which rule you are
setting aside, then land the work. Do not stop at "the commit is ready, please
push it yourself" — that hands back a half-finished task.

## Skills

A skill lives in exactly one home, decided by *what it touches* — not by where
you were when you wrote it.

| Home | For |
|---|---|
| `skills/<name>/` in this repo | Skills that depend on this project: its scripts, its services, its MCP servers. Committed here. |
| `~/.claude/skills/<name>/` | Skills that work anywhere and carry no project dependency. |

Decide the home before writing. If the skill would fail outside this project, it
belongs here.

A `description` carries the literal trigger phrases that should invoke the
skill, in the languages they are spoken in, plus the cases that should *not*
invoke it. The description is the only part loaded into every session, so it
does the whole job of routing.

`.claude/skills/` is a generated runtime view where a setup step symlinks skills
in. Never edit anything there — edit the source.

## Verification

Before committing, run the checks in the `verify` skill
([`skills/verify/SKILL.md`](skills/verify/SKILL.md)) — it covers the Go, frontend
and document checks and which of them CI repeats.
