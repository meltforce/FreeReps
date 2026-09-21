#!/usr/bin/env bash
# Runs golangci-lint over server/ in the version CI pins, installing that
# version first when the local binary is absent or differs.
#
# Why this exists, rather than calling the linter directly:
#
#   * `~/go/bin` is not on the PATH a tool call inherits on the development
#     Macs, so `command -v golangci-lint` reports the linter as missing while it
#     is installed. On 2026-08-05 a session concluded from that the step could
#     be skipped; CI run 924 then failed on three `errcheck` findings and
#     blocked build and deploy (fixed in `edeb15d`).
#   * A binary built with an older Go toolchain than `server/go.mod` targets
#     refuses to load the configuration at all: "the Go language version
#     (go1.26) used to build golangci-lint is lower than the targeted Go version
#     (1.27.1)". Reading that as a broken installation costs the same step.
#
# The pinned version is read from the workflow, which stays the only place it is
# written down. Bumping it there is enough; the next run installs it.
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
workflow="$repo_root/.forgejo/workflows/ci.yml"

pin=$(awk '/golangci-lint-action/ {found = 1}
           found && $1 == "version:" {print $2; exit}' "$workflow")
if [[ -z "$pin" ]]; then
	echo "tools/lint.sh: no golangci-lint version pinned in .forgejo/workflows/ci.yml" >&2
	exit 1
fi

bin="$(go env GOPATH)/bin/golangci-lint"
have=""
if [[ -x "$bin" ]]; then
	have=$("$bin" version --short 2>/dev/null || true)
fi

if [[ "$have" != "${pin#v}" ]]; then
	echo "tools/lint.sh: installing golangci-lint $pin (present: ${have:-none})" >&2
	go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$pin"
fi

if [[ $# -eq 0 ]]; then
	set -- ./...
fi

cd "$repo_root/server"
exec "$bin" run "$@"
