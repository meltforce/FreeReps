---
name: verify
description: Run the checks that have to pass before a FreeReps commit lands — Go build, vet, tests and golangci-lint, the frontend type check and build, and the document contract check. Triggers — "verify", "prüf das durch", "vor dem commit", "läuft das durch", "check before committing", "run the checks", "does CI pass". Not for running the server itself — that is `server/CLAUDE.md`.
---

# Verify

Run from the repo root. The order below is cheapest-first: a failure in step 1
makes the rest irrelevant.

## 1. Go

```bash
mkdir -p server/web/dist && touch server/web/dist/.gitkeep   # only if dist is absent
cd server && go build ./... && go vet ./... && go test ./...
```

The stub matters: `server/web.go` embeds `web/dist` via `go:embed`, and without
the directory the build fails with an embed error that names the directive
rather than the missing directory.

`go test ./...` skips the integration tests. Those need a running TimescaleDB and
run with `go test -tags integration ./...` — run them when the change touches
`server/internal/storage/` or a migration, because the unit tests do not execute
a single SQL statement against a real server. The aggregation tests in
`server/internal/storage/aggregation_integration_test.go` are the reason that
matters: they check figures, and a unit test that inspects the generated SQL
string proves the shape and nothing about the numbers.

A scratch database for them, in the version the deployment runs:

```bash
docker run -d --name freereps-scratch \
  -e POSTGRES_DB=freereps_scratch -e POSTGRES_USER=freereps \
  -e POSTGRES_PASSWORD=scratch -p 55432:5432 \
  timescale/timescaledb:latest-pg16

FREEREPS_TEST_DSN="postgres://freereps:scratch@localhost:55432/freereps_scratch?sslmode=disable" \
  go test -tags integration ./internal/storage/
```

The harness refuses to run against a database named `freereps` and removes only
its own user's rows, so a scratch database shared with other tests keeps its
contents.

## 2. golangci-lint

```bash
tools/lint.sh
```

Run it from the repo root; `make lint` calls the same script. It reads the
pinned version from `.forgejo/workflows/ci.yml`, installs that version when the
binary at `$(go env GOPATH)/bin/golangci-lint` is absent or older, and runs it
over `server/`.

**Do not reach for the bare `golangci-lint` instead.** `~/go/bin` is not on the
PATH a tool call inherits on the development Macs, so `command -v` reports the
linter as missing; and a binary built with an older Go toolchain than
`server/go.mod` targets refuses to load the configuration. Both read like an
absent installation, and skipping the step over it is what broke CI run 924 on
2026-08-05: build, vet, tests and the frontend were green, `errcheck` rejected
three discarded `fmt.Fprint` results, and the fix landed as `edeb15d`.

## 3. Frontend

```bash
cd server/web && npm ci && npx tsc --noEmit && npm run build
```

`npm run build` runs `tsc -b` itself, so the separate `tsc --noEmit` is only
worth running on its own when you want the type errors without waiting for Vite.

## 4. Documents

```bash
tools/check-docs.sh --all
```

Checks the movement rule and status tokens in `ROADMAP.md`, ISO dates in the
tracked documents, and sweeps the repo for German text. It detects; it does not
prevent. On a German hit that is a verbatim quote of upstream output, add that
specific line to `tools/check-docs.allow` with a reason — do not weaken the
pattern in the script.

## What CI repeats, and what it does not

`.forgejo/workflows/ci.yml` runs steps 1 through 4 on every push and PR against
`main`. It does **not** run the integration tests, and it does not build the iOS
app — that happens on GitHub via the mirror, in `.github/workflows/ios.yml`.

A green CI run on `main` continues into build and deploy to `freereps-lxc`. A
failure after the build stage means `:edge` may be in a broken state; the ntfy
alert in `notify-deploy-failure` carries the run URL.

## Reading a CI log from the shell

Three calls, and the IDs are not interchangeable. `/actions/tasks` returns a
*task* id; the logs endpoint wants a *job* id, and passing the task id there
returns 404 — which reads like "this instance serves no logs" and is not that.

```bash
F=https://git.coydog-fence.ts.net/api/v1/repos/meltforce.net/freereps
curl -sS "$F/actions/runs?limit=5"        # -> run id, newest first
curl -sS "$F/actions/runs/<run_id>/jobs"  # -> job id per job name
curl -sSL "$F/actions/jobs/<job_id>/logs" # -> plain text
```

The repo is public on Forgejo, so all three work without a token. `swagger.v1.json`
at the instance root lists the routes the running version actually has — check
there before concluding an endpoint is missing.

**A silent step proves nothing.** `check-docs.sh` prints nothing when everything
passes, so a green `documents` job produces no log line saying it ran. What the
log does show is `/usr/bin/python3` from the `Ensure python3` step; without that
line, `has_module` would have returned false and the language sweep would have
printed `language sweep skipped` while the job still went green.

## Tests

A new test carries a doc comment saying **why the test exists** — which failure
it would catch. *Why:* a test named `TestParseSet` documents nothing; the
regression tests in `server/internal/ingest/alpha/parser_test.go` exist because
of a specific silent data loss, and that is the part worth keeping.
