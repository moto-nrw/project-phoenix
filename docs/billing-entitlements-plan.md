# Billing Entitlements and Subscription Tiers Plan

## Purpose

Phoenix needs contract-aware feature access. A school or organization should have a subscription contract, that contract should resolve to an effective subscription tier for each school, and the backend should use that effective tier to decide whether a tenant may use paid features.

This document is a discussion draft. It intentionally focuses on requirements, architecture, and an implementation plan before committing to concrete migrations or API shapes.

## Requirements

### Functional Requirements

1. Store contract information for organizations and schools.
2. Allow schools to override the organization contract.
3. Store a subscription tier as part of the effective contract.
4. Store a child bundle limit as part of the effective contract.
5. Keep subscription tier definitions and tier feature contents configurable in the backend.
6. Restrict access to paid features based on the effective school entitlement.
7. Return a clear paywall response when a tenant tries to access a feature outside its tier.
8. Return a clear "Contact Sales" response when a tenant tries to exceed its child bundle.
9. Expose effective entitlements to the frontend so navigation and pages can show upgrade prompts instead of broken screens.
10. Enforce restrictions in the backend even when the frontend hides or disables UI.

### Contract Resolution Requirements

For a school request, contract resolution should follow this order:

1. Active school-level contract for the school.
2. Active organization-level contract for the school's organization.
3. No contract fallback.

The "no contract" behavior needs a product decision. Recommended production behavior is fail closed for paid features and block quota-sensitive writes unless an explicit contract exists. For local development and tests, fixtures can create a default internal contract.

### Feature Gating Requirements

The feature catalog and tier-to-feature matrix should be editable by platform operators. However, product code still needs stable feature keys where behavior branches.

Example:

```text
Feature key in code: time_tracking
Tier-feature mapping in DB:
  basic    -> time_tracking disabled
  standard -> time_tracking enabled
  premium  -> time_tracking enabled
```

The important split is:

- Stable feature keys are referenced by code and migrations.
- Tier names, tier labels, sort order, and which features a tier includes are data.

### Child Bundle Requirements

The child bundle limit should be contract metadata, not a normal feature flag.

Example:

```text
Organization contract: standard, 150 children
School A override: basic, 50 children
School B: inherits standard, 150 children
```

The limit should be enforced in every path that creates billable children:

- Manual student creation.
- Enrollment approval that creates a student.
- Import flows that create students.
- Future bulk creation flows.

Open product decision: which student statuses count toward the bundle. Recommended initial rule:

```text
Count users.students where status IN ('pending', 'active')
Do not count inactive or alumnus students.
```

This matches the business statement that child count changes should be flexible for arrivals and departures.

## Non-Goals for the First Version

1. Payment provider integration.
2. In-app self-service checkout.
3. Proration, invoices, or billing history.
4. Usage metering beyond the child bundle limit.
5. Per-user entitlements. The first version is tenant/org contract based.

## Basic Approach

Add a dedicated billing/entitlements domain instead of forcing this entirely into the current settings system.

The existing settings system is tenant-scoped and useful for operational configuration, but this problem has two extra dimensions:

- Contracts can live at organization or school scope.
- Subscription tiers and their feature contents are product catalog data, not normal tenant settings.

The recommended shape is:

```text
Billing catalog:
  features
  subscription tiers
  tier feature matrix

Contracts:
  organization contract
  optional school override contract

Runtime:
  entitlement service resolves school entitlements
  middleware and service checks gate feature access
  quota checks guard child creation
```

## Architecture

### Data Model

Create a new `billing` schema with these core tables.

#### `billing.features`

Stores configurable feature metadata.

Suggested fields:

- `id`
- `key`, unique, stable machine key, e.g. `time_tracking`
- `label`
- `description`
- `category`
- `active`
- `sort_order`
- `created_at`
- `updated_at`

Feature keys should not be casually renamed because code references them. If rename is required, it should be a migration plus code change.

#### `billing.subscription_tiers`

Stores configurable subscription tiers.

Suggested fields:

- `id`
- `key`, unique, e.g. `basic`, `standard`, `premium`
- `label`
- `description`
- `active`
- `sort_order`
- `created_at`
- `updated_at`

The three sales tiers can be seeded as initial data, but operators can edit labels and feature contents.

#### `billing.subscription_tier_features`

Stores the tier-to-feature matrix.

Suggested fields:

- `tier_id`
- `feature_id`
- `enabled`
- `created_at`
- `updated_at`

Unique key:

```text
(tier_id, feature_id)
```

Recommended interpretation:

- Missing row means disabled.
- Existing row with `enabled = true` means enabled.
- Existing row with `enabled = false` can be used for explicit audit/history clarity, but the runtime behavior is still disabled.

#### `billing.contracts`

Stores organization and school contracts.

Suggested fields:

- `id`
- `scope_type`, enum-like text: `organization` or `school`
- `organization_id`, nullable
- `school_id`, nullable
- `tier_id`
- `child_bundle_limit`
- `status`, e.g. `active`, `scheduled`, `expired`, `cancelled`
- `effective_from`
- `effective_until`, nullable
- `notes`, nullable
- `created_by`, nullable
- `updated_by`, nullable
- `created_at`
- `updated_at`

Constraints:

- Exactly one of `organization_id` or `school_id` is set.
- `scope_type` matches the populated ID.
- `child_bundle_limit` is positive.
- `child_bundle_limit % 50 = 0` if sales wants strict 50er bundles.
- Only one active contract for the same scope at a given time.

The "only one active contract" rule can be enforced either by exclusion constraints over effective date ranges or by service-level validation first. For the first version, service-level validation plus a partial unique index for currently open-ended active contracts may be pragmatic.

### Backend Domain Layout

Follow the existing handler -> service -> repository pattern:

```text
backend/models/billing/
backend/database/repositories/billing/
backend/services/billing/
backend/api/operator/billing/
```

Runtime feature checks should not be implemented directly in API handlers. Add a service with a narrow API and reuse it everywhere.

### Entitlement Service

Suggested service methods:

```go
type FeatureKey string

const (
    FeatureTimeTracking FeatureKey = "time_tracking"
)

type EffectiveEntitlements struct {
    SchoolID          int64
    OrganizationID    int64
    ContractScope     string
    TierKey           string
    TierLabel         string
    ChildBundleLimit  int
    EnabledFeatures   map[FeatureKey]bool
}

type EntitlementService interface {
    ResolveForSchool(ctx context.Context, schoolID int64) (*EffectiveEntitlements, error)
    ResolveForCurrentTenant(ctx context.Context) (*EffectiveEntitlements, error)
    CanUseFeature(ctx context.Context, feature FeatureKey) (bool, error)
    RequireFeature(ctx context.Context, feature FeatureKey) error
    RequireStudentCapacity(ctx context.Context, additionalCount int) error
}
```

Recommended errors:

```go
var ErrNoContract = errors.New("no active contract")
var ErrFeaturePaywalled = errors.New("feature paywalled")
var ErrChildBundleLimitReached = errors.New("child bundle limit reached")
```

The service should return structured details for API mapping:

- feature key
- current tier
- required action: `contact_sales`
- current child count
- child bundle limit

### Feature Restriction Model

There are three layers, and all three matter:

1. Backend route middleware for broad HTTP access.
2. Service-level checks for business operations that may be reachable from multiple routes.
3. Frontend entitlement state for UX and navigation.

#### Backend Middleware

Add middleware similar to permission middleware:

```go
r.With(
    authorize.RequiresPermission(permissions.TimeTrackingOwn),
    entitlement.RequiresFeature(billing.FeatureTimeTracking),
    withTx,
).Post("/check-in", rs.checkIn)
```

Ordering should be decided carefully. Since entitlement lookup needs tenant context and probably DB access, it should run after JWT tenant context is available and inside a tenant transaction where RLS applies. A helper can combine the tenant transaction and entitlement check if ordering becomes awkward.

#### Service-Level Checks

Some operations are not cleanly represented by one route group. For those, the service that creates or mutates paid-domain data should call `RequireFeature`.

For time tracking, route-level gating will cover most paths, but admin staff routes also need coverage:

- `/api/time-tracking/*`
- `/api/staff/{id}/time-tracking/*`
- schedule/model routes if product decides those are part of the paid time-tracking feature
- export routes
- vacation/absence routes if product decides they belong to time tracking

#### Frontend UX

Expose an endpoint such as:

```text
GET /api/entitlements/effective
```

Response includes:

- effective tier
- contract scope
- child bundle limit
- enabled feature keys
- optional labels/descriptions for upgrade prompts

Frontend uses this to:

- hide or disable nav entries
- show paywall screens
- show contact-sales prompt when student creation exceeds quota

The frontend should not be the authority. Backend 403 responses remain required.

### API Error Contract

Use stable error codes so the frontend does not parse English/German text.

Feature paywall:

```json
{
  "status": "error",
  "error": "feature paywalled",
  "code": "feature_paywalled",
  "details": {
    "feature": "time_tracking",
    "tier": "basic",
    "action": "contact_sales"
  }
}
```

Child bundle limit:

```json
{
  "status": "error",
  "error": "child bundle limit reached",
  "code": "child_bundle_limit_reached",
  "details": {
    "limit": 50,
    "current_count": 50,
    "attempted_additional": 1,
    "action": "contact_sales"
  }
}
```

If the current common error response cannot carry `details`, add a compatible optional field rather than creating one-off response formats.

### Operator UI

Add operator-only screens for:

1. Feature catalog.
2. Subscription tiers.
3. Tier feature matrix.
4. Organization contracts.
5. School contract overrides.

The first implementation can start smaller:

- Seed features and tier rows through migrations.
- Build UI only for assigning org/school contracts and editing tier feature matrix.
- Defer feature creation UI until needed.

Because feature keys are code-facing, free-form creation should probably be operator-only and guarded with warnings. It is valid to allow editable labels/descriptions while keeping feature keys seeded by migrations.

## Broad Implementation Plan

### Phase 1: Data Model and Seed Data

1. Add `billing` schema migrations.
2. Add `features`, `subscription_tiers`, `subscription_tier_features`, and `contracts`.
3. Seed initial features, starting with:
   - `time_tracking`
4. Seed initial tiers:
   - `basic`
   - `standard`
   - `premium`
5. Seed default tier-feature mappings based on sales decision.
6. Add Go models and repositories.
7. Add repository tests for contract resolution edge cases.

### Phase 2: Entitlement Service

1. Implement `EntitlementService`.
2. Resolve effective contract by school override first, then organization.
3. Load tier feature matrix for the resolved tier.
4. Implement `RequireFeature`.
5. Implement `RequireStudentCapacity`.
6. Add tests for:
   - school override wins
   - org fallback works
   - no contract behavior
   - disabled feature returns paywall error
   - child limit blocks at boundary
   - child limit allows below boundary

### Phase 3: API Error Mapping and Middleware

1. Add common error mapping for entitlement errors.
2. Add optional error details support if needed.
3. Add `RequiresFeature` middleware.
4. Wire the entitlement service into the central service factory and API resources.
5. Add API tests that verify stable 403 codes.

### Phase 4: Gate Time Tracking

1. Gate `/api/time-tracking/*`.
2. Gate `/api/staff/{id}/time-tracking/*`.
3. Decide whether staff schedules, work time models, vacation requests, and absences belong to `time_tracking` or separate features.
4. Add backend tests for a BASIC tenant hitting time tracking endpoints.
5. Add frontend handling for `feature_paywalled`.
6. Add a reusable paywall component.
7. Update sidebar and mobile navigation behavior.

### Phase 5: Enforce Child Bundle Limit

1. Add a central child capacity check to student creation code.
2. Use tenant-scoped advisory locking around count plus insert to avoid races.
3. Apply the check before person/student writes in `POST /api/students`.
4. Apply the check before enrollment approval creates students.
5. Apply the check before import creates students.
6. Add tests for manual creation, enrollment approval, and import path if applicable.
7. Surface `child_bundle_limit_reached` in the frontend as "Contact Sales".

### Phase 6: Operator Contract Management

1. Add operator endpoints for reading and writing contracts.
2. Add endpoints for tier matrix management.
3. Add organization contract UI.
4. Add school override UI.
5. Show effective contract resolution on school detail pages:
   - inherited from organization
   - overridden at school
   - no active contract

### Phase 7: Rollout and Migration Strategy

1. Create contracts for existing schools/orgs before enabling fail-closed gates.
2. Initially log missing-contract cases in staging.
3. Add a temporary compatibility mode if needed:
   - no contract means all current features enabled
   - switch to fail-closed after data is complete
4. Run backend test suite and focused frontend checks.
5. Coordinate copy with sales for paywall and Contact Sales messages.

## Open Questions

1. What are the final tier keys and display names?
2. Which features are in scope for v1 besides time tracking?
3. Is time tracking one feature, or should absences/vacation/work-time-models be separate features?
4. Should missing contracts fail closed immediately, or should there be a temporary compatibility mode?
5. Which student statuses count toward the child bundle?
6. Should contract changes take effect immediately or support future effective dates in v1?
7. Should operators be able to create new feature keys in the UI, or only edit seeded features and tier mappings?
8. Should a school override be a full contract replacement, or should it override only selected fields like tier or child limit?

## Recommended Decisions for V1

1. Use stable seeded feature keys, editable labels, and editable tier-feature mappings.
2. Support org contracts plus school override contracts.
3. Treat school override as a full contract override.
4. Count `pending` and `active` students toward child limits.
5. Add only one feature gate initially: `time_tracking`.
6. Return stable backend codes: `feature_paywalled` and `child_bundle_limit_reached`.
7. Build frontend paywall UX from effective entitlements, but keep backend as authority.
