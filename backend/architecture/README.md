# Backend architecture policy

`policy.json` is the source for package ownership, package roles, dependency
classes, import scopes, and target import rules. Generated lint configs and
diagrams must derive from this file and must not be committed.

## Commands

Run commands from the repository root:

```bash
scripts/backend-architecture.sh check
scripts/backend-architecture.sh explain \
  --scope production \
  --source github.com/moto-nrw/project-phoenix/services/mealplan \
  --target github.com/moto-nrw/project-phoenix/models/mealplan
```

`check` loads packages with `GOOS=linux`, `GOARCH=amd64`, and `CGO_ENABLED=0`.
It checks production imports, same-package test imports, and external-package
test imports as separate scopes. Every package explicitly declares all three
roles. Test roles describe the seam (`module-internal-test`,
`module-behavior-test`, `workflow-decision-test`,
`workflow-integration-test`, `adapter-test`, or `e2e-test`), so production
permissions cannot leak into test scopes. A finding uses this stable key:

```text
scope|rule|source|target
```

The current backend still violates the target policy. Issue #2583 will add the
exact shrinking legacy manifest. Until the CI cutover, CI runs `legacy-check`
to keep the existing go-arch-lint gate active.

## Changing the policy

1. Add or move the exact package classification.
2. Classify each new direct dependency. Do not use path wildcards.
3. Add the narrowest owner, owner-kind, role, scope, and target selector that
   describes the intended seam.
4. Run `scripts/backend-architecture.sh check` and inspect every changed
   finding.
5. Run `cd backend && go test ./internal/architecture`.

The shared-kernel declaration is a closed list: `Date`, `WallClock`,
`TenantID`, and `CorrelationID`; kernel-owned packages must be contracts. The
loader rejects unknown JSON fields,
unsupported schema versions, invalid enum values, duplicate classifications,
stale packages, unknown dependencies, and logically overlapping rules. Change
`schema_version` only when the JSON shape changes. Change `policy_epoch` only
for a reviewed architecture decision; it does not approve or rebuild legacy
findings.
