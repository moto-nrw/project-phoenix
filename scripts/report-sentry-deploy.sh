#!/usr/bin/env bash
# Reports a successful deploy to Sentry: finalizes the release (the commit SHA)
# for the backend and frontend projects and records a deploy for the
# environment, so "new since deploy" and regression detection line up (#3642).
#
# Usage: report-sentry-deploy.sh <release> <environment>
# Reads SENTRY_AUTH_TOKEN and SENTRY_ORG from the environment; sentry-cli
# picks both up itself, so the token never appears on a command line.
#
# Never fails the deploy: every problem ends as a workflow warning and exit 0.
# The calling step bounds the runtime with timeout-minutes, so an unreachable
# Sentry cannot stall the deploy job either.
set -uo pipefail

SENTRY_CLI_VERSION=2.58.6
PROJECTS=(backend frontend)

release="${1:-}"
environment="${2:-}"

warn() {
  echo "::warning title=Sentry deploy not reported::$1"
  exit 0
}

[ -n "$release" ] || warn "release argument is missing"
case "$environment" in
  production | staging | demo) ;;
  *) warn "unexpected environment '$environment'" ;;
esac
[ -n "${SENTRY_AUTH_TOKEN:-}" ] || warn "SENTRY_AUTH_TOKEN is not set"
[ -n "${SENTRY_ORG:-}" ] || warn "SENTRY_ORG is not set"

if command -v sentry-cli >/dev/null 2>&1; then
  cli=(sentry-cli)
else
  cli=(npx --yes "@sentry/cli@${SENTRY_CLI_VERSION}")
fi

project_args=()
for project in "${PROJECTS[@]}"; do
  project_args+=(--project "$project")
done

run() {
  "${cli[@]}" "$@"
}

run releases new "$release" "${project_args[@]}" ||
  warn "could not create release $release for ${PROJECTS[*]}"
run releases finalize "$release" ||
  warn "could not finalize release $release"
run deploys new --release "$release" --env "$environment" "${project_args[@]}" ||
  warn "could not record deploy of $release to $environment"

echo "Reported deploy of release $release to $environment for ${PROJECTS[*]}"
