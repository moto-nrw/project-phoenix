#!/usr/bin/env bash
# Common CI/local static quality commands. Installation belongs to the caller.
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
case "${1:-}" in
  backend)
    cd "$root/backend"
    unformatted=$(gofmt -l .)
    if [[ -n "$unformatted" ]]; then
      printf 'Run gofmt on:\n%s\n' "$unformatted" >&2
      exit 1
    fi
    go test ./test -run Ratchet -count=1
    go run main.go migrate validate
    go test ./database/migrations/ -run TestNoDuplicateMigrationVersions -v
    bash "$root/scripts/check-deadcode_test.sh"
    bash "$root/scripts/check-deadcode.sh"
    go tool go-licenses check ./... --ignore github.com/moto-nrw/project-phoenix --disallowed_types=restricted,reciprocal
    ;;
  frontend)
    cd "$root/frontend"
    # Knip imports Playwright config. These CI/local placeholders only allow
    # static analysis to load it; no browser or application server is started.
    export NEXT_PUBLIC_API_URL=http://localhost:8080 SKIP_ENV_VALIDATION=true
    export API_URL=http://localhost:8080 TENANT_DOMAIN=localhost
    export NEXT_PUBLIC_OPERATOR_HOSTNAME=operator.localhost:3000
    export NEXT_PUBLIC_PARENTS_HOSTNAME=parents.localhost:3000
    export NEXT_PUBLIC_SCHOOL_HOSTNAME=schule.localhost:3000
    pnpm exec node scripts/verify-locales.mjs
    pnpm run lint --max-warnings 0
    pnpm run typecheck
    pnpm depcruise
    pnpm knip
    pnpm knip:production
    npx --yes license-checker@25.0.1 --production --failOn \
      'AGPL-1.0-only;AGPL-1.0-or-later;AGPL-3.0-only;AGPL-3.0-or-later;GPL-2.0-only;GPL-2.0-or-later;GPL-3.0-only;GPL-3.0-or-later;LGPL-2.0-only;LGPL-2.0-or-later;LGPL-2.1-only;LGPL-2.1-or-later;LGPL-3.0-only;LGPL-3.0-or-later;SSPL-1.0;EUPL-1.2' --summary
    ;;
  react-doctor)
    cd "$root/frontend"
    npx --ignore-scripts -y react-doctor@0.8.1 . --yes --scope changed \
      --base "${2:?base revision is required}" --no-telemetry --no-supply-chain
    ;;
  *) echo 'Usage: check-quality.sh backend|frontend|react-doctor [base]' >&2; exit 2 ;;
esac
