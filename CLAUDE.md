# FreeReps

Self-hosted server for health data: ingest from Apple Health, Oura and
training loggers, storage in PostgreSQL/TimescaleDB, a web dashboard with
configurable correlations, and an MCP server. It computes no scores and does no
coaching, by decision.

Monorepo. `server/` holds the Go binary `freereps` with the web UI embedded;
`app/` holds the iOS companion. Build notes and wire-format specs are in
`server/CLAUDE.md` and `app/CLAUDE.md`, which load when working under those
paths; what you learn about building or running goes there, not here.

## Gotchas

- **The backend does not compile without `server/web/dist`** (`go:embed` in
  `server/web.go`). Build the frontend first, or
  `mkdir -p server/web/dist && touch server/web/dist/.gitkeep`.
- **A metric arriving from two sources needs a unit decision and a source
  filter.** Apple Health and Oura disagree on units and granularity, and a
  wrong mix raises no error.
- **Forgejo is canonical; `github.com/meltforce/FreeReps` is a push mirror**
  (`git push --mirror`: force and prune). Never push to GitHub; a GitHub-only
  branch is deleted at the next sync. The iOS build (`.github/workflows/ios.yml`)
  and the release (`release.yml`, on a tag) run on GitHub through the mirror.
- **A push to `main` builds `:edge` and redeploys on `smokey`.** The deployed
  compose file, `.env` and `config.yaml` belong to the homelab repo
  (`docker/stacks/freereps/`, catalog entry `stacks/freereps.yml`); change
  image ref, ports or volumes there. `server/docker-compose.yml` is for local
  development only.
- **Skills live in `skills/`**; `.claude/skills/` is a generated view of
  symlinks and is never edited.
- Run the `verify` skill (`skills/verify/SKILL.md`) before committing.

## Git

`main` is the only long-lived branch: commit and push to `main` yourself once a
change is complete and verified, no PR. This overrides the harness default of
branching first. Stage paths explicitly, never `git add -A` — parallel sessions
share this checkout.

## Documents

Do not write documentation that restates the code. Reasoning goes into the
commit message, or into a comment next to the code when a later reader would
otherwise undo the change. Everything written into the repo is English.
