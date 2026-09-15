#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
source "$root/scripts/quality-timing.sh"
cd "$root/frontend"
if [[ "${1:-quality}" == react-doctor ]]; then
  npx --ignore-scripts -y react-doctor@0.8.1 . --yes --scope changed \
    --base "${2:?base revision is required}" --no-telemetry --no-supply-chain
  exit 0
fi
# Static-analysis placeholders, shared by local checks and CI. No servers run.
export NEXT_PUBLIC_API_URL=http://localhost:8080 SKIP_ENV_VALIDATION=true
export API_URL=http://localhost:8080 TENANT_DOMAIN=localhost
export NEXT_PUBLIC_OPERATOR_HOSTNAME=operator.localhost:3000
export NEXT_PUBLIC_PARENTS_HOSTNAME=parents.localhost:3000
export NEXT_PUBLIC_SCHOOL_HOSTNAME=schule.localhost:3000
quality_step locales pnpm exec node scripts/verify-locales.mjs
quality_step frontend-lint pnpm run lint --max-warnings 0
quality_step frontend-types pnpm run typecheck
quality_step dependency-graph pnpm depcruise
quality_step unused-code pnpm knip
quality_step production-unused-code pnpm knip:production
quality_step npm-licenses npx --yes license-checker@25.0.1 --production --failOn \
  'AGPL-1.0-only;AGPL-1.0-or-later;AGPL-3.0-only;AGPL-3.0-or-later;GPL-2.0-only;GPL-2.0-or-later;GPL-3.0-only;GPL-3.0-or-later;LGPL-2.0-only;LGPL-2.0-or-later;LGPL-2.1-only;LGPL-2.1-or-later;LGPL-3.0-only;LGPL-3.0-or-later;SSPL-1.0;EUPL-1.2' --summary
