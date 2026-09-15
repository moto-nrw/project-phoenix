# Pre-push quality timing

Measured locally on 2026-09-13, macOS arm64, with Devbox-pinned tools and
`CGO_ENABLED=0`. Implementation: `70688882bc`. Existing compiler and package
manager caches were warm; “cold” below means no custom static-success records.
This branch selected both backend and frontend checks.

| Run | Wall time |
| --- | ---: |
| Previous automatic hook, including affected tests | 290.66s |
| New hook, cold success cache | 111s |
| Same commit, warm success cache | 9s |

Cold-run stages: hook regressions 24.91s, backend quality 40.80s (including
dead-code detection 29s), lint 3.59s, architecture 13.91s, vulnerability scan 4s,
frontend installation 1s, frontend quality 17.50s and React Doctor 2.68s.
Fetch, secret scanning, context validation, cache bookkeeping and rounding
account for the remainder. Nested stage times are not additive.

The warm run reused all six static-success records. Vulnerability scanning,
secret scanning, dependency installation and context checks still ran.
These are local observations, not a guarantee for cold toolchains or CI.

## Correctness evidence

- All 27 hook regression tests passed. Cases include docs-only follow-up
  pushes, source/checker/base/tool invalidation, missing tools with a warm
  cache, failures never cached, generated frontend types, and dirty source.
- The affected backend suite passed across 212 package targets separately
  before pushing. Removing it from the automatic hook does not remove the
  contributor verification requirement or CI tests.
- Both static-quality suites, backend lint, architecture and vulnerability
  scanning passed on the real branch. Context validation reported zero errors
  and all 12 context tests passed.

Reproduce with `bash scripts/pre-push.sh`, then repeat without changing inputs.
For a docs-only follow-up, commit documentation outside the relevant checker
inputs and rerun. Source or base changes should produce misses for affected
stages. Cache records live under the worktree's Git directory in
`pre-push-cache`; removing that directory forces cold static validation.
