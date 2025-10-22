# Implementation Tasks - API Endpoint Consolidation

## MONTH 1: Backend v2 Implementation (120 hours)

### 1.1 API Versioning Infrastructure (16 hours)
- [ ] 1.1.1 Create `/api/v2/` router structure in `api/base.go`
- [ ] 1.1.2 Keep `/api/v1/` router pointing to current handlers
- [ ] 1.1.3 Add version detection middleware
- [ ] 1.1.4 Add deprecation headers to v1 responses (`Deprecation: true`, `Sunset: date`)
- [ ] 1.1.5 Create adapter layer package `api/adapters/v1tov2/`
- [ ] 1.1.6 Test both v1 and v2 routes accessible

### 1.2 Query Parameter Standards (8 hours)
- [ ] 1.2.1 Document query param naming conventions (snake_case, standard names)
- [ ] 1.2.2 Enhance `base.QueryOptions` to support new filter patterns
- [ ] 1.2.3 Add query param validation helpers (validateStatus, validateBoolean, validateArray)
- [ ] 1.2.4 Create tests for query param parsing

### 1.3 IoT API Consolidation (24 hours) - 36 → 16 endpoints
- [ ] 1.3.1 Replace `/api/iot/active`, `/offline`, `/maintenance` with `?status=X`
- [ ] 1.3.2 Replace `/api/iot/type/{type}` with `?type=X`
- [ ] 1.3.3 Replace `/api/iot/status/{status}` with query param (redundant with above)
- [ ] 1.3.4 Replace `/api/iot/registered-by/{personId}` with `?registered_by=X`
- [ ] 1.3.5 Consolidate 5 session timeout endpoints into session resource with query params
- [ ] 1.3.6 Create v1 adapters for removed IoT endpoints
- [ ] 1.3.7 Update IoT API tests
- [ ] 1.3.8 Run Bruno IoT tests against v2

### 1.4 Active API Consolidation (20 hours) - 41 → 28 endpoints
- [ ] 1.4.1 Replace `/active/groups/{id}/end` with PATCH `{status: "ended"}`
- [ ] 1.4.2 Replace `/active/groups/{id}/claim` with PATCH `{claimed_by: X}`
- [ ] 1.4.3 Replace `/active/groups/unclaimed` with `?claimed=false`
- [ ] 1.4.4 Replace `/active/combined/active` with `?status=active`
- [ ] 1.4.5 Replace `/active/visits/{id}/end` with PATCH `{ended_at: X}`
- [ ] 1.4.6 Remove `/active/visits/student/{studentId}` (use `/students/{id}/visits`)
- [ ] 1.4.7 Remove `/active/visits/student/{studentId}/current` (use `/students/{id}/current-visit`)
- [ ] 1.4.8 Flatten `/active/scheduled-checkouts/student/{studentId}/pending` to query params
- [ ] 1.4.9 Create v1 adapters for removed active endpoints
- [ ] 1.4.10 Update active API tests

### 1.5 Auth API Simplification (16 hours) - 38 → 23 endpoints
- [ ] 1.5.1 Replace `/accounts/{id}/activate` with PATCH `{active: true}`
- [ ] 1.5.2 Replace `/accounts/{id}/deactivate` with PATCH `{active: false}`
- [ ] 1.5.3 Replace `/parent-accounts/{id}/activate` with PATCH
- [ ] 1.5.4 Replace `/parent-accounts/{id}/deactivate` with PATCH
- [ ] 1.5.5 Replace `/accounts/by-role/{roleName}` with `?role=X`
- [ ] 1.5.6 Consolidate permission grant/deny into PATCH operations
- [ ] 1.5.7 Create v1 adapters for removed auth endpoints
- [ ] 1.5.8 Update auth tests

### 1.6 Remaining APIs (24 hours)
- [ ] 1.6.1 Rooms: Replace `/by-category` and `/available` with query params (7 → 5 endpoints)
- [ ] 1.6.2 Staff: Replace `/available`, `/available-for-substitution` with `?available=true` (7 → 5)
- [ ] 1.6.3 Students: Remove `/in-group-room`, consolidate visits (8 → 6)
- [ ] 1.6.4 Config: Replace `/key/{key}`, `/category/{cat}` with query params (11 → 6)
- [ ] 1.6.5 Schedules: Replace all `/by-` patterns with query params (16 → 8)
- [ ] 1.6.6 Activities: Consolidate availability filters (15 → 10)
- [ ] 1.6.7 Substitutions: Replace `/active` with query param (3 → 2)
- [ ] 1.6.8 Users: Replace `/by-account`, `/by-tag` with query params (9 → 7)
- [ ] 1.6.9 Feedback: Remove `/mensa` hardcoded endpoint (7 → 6)
- [ ] 1.6.10 Me/UserContext: Consolidate filters (12 → 10)

### 1.7 Adapter Layer Completion (8 hours)
- [ ] 1.7.1 Implement all v1-to-v2 adapters (110 total)
- [ ] 1.7.2 Test each adapter transforms correctly
- [ ] 1.7.3 Create adapter test suite
- [ ] 1.7.4 Document adapter patterns

### 1.8 Backend Validation (4 hours)
- [ ] 1.8.1 Run ALL Bruno tests against v1 endpoints (should still pass)
- [ ] 1.8.2 Run ALL Bruno tests against v2 endpoints (update URLs first)
- [ ] 1.8.3 Verify v2 tests complete in <300ms
- [ ] 1.8.4 Generate new routes.md showing v2 structure
- [ ] 1.8.5 Count final v2 endpoints: target 108

## MONTH 2: Frontend Migration Phase 1 (30 hours)

### 2.1 v2 API Client Creation (12 hours)
- [ ] 2.1.1 Create `lib/api-v2.ts` client wrapper
- [ ] 2.1.2 Implement query parameter builder utilities
- [ ] 2.1.3 Update API client to support PATCH for state changes
- [ ] 2.1.4 Create migration helper functions for common patterns
- [ ] 2.1.5 Document v2 client usage with examples

### 2.2 High-Traffic Endpoint Migration (18 hours)
- [ ] 2.2.1 Migrate students API calls (8 endpoints → 6)
- [ ] 2.2.2 Migrate groups API calls (6 endpoints → 6, URL changes)
- [ ] 2.2.3 Migrate rooms API calls (7 → 5)
- [ ] 2.2.4 Test migrated pages thoroughly
- [ ] 2.2.5 Deploy and monitor for errors

## MONTH 3: Frontend Migration Phase 2 (30 hours)

### 3.1 Complete API Migration (24 hours)
- [ ] 3.1.1 Migrate IoT/device pages (if any frontend UI)
- [ ] 3.1.2 Migrate active sessions pages (groups, visits)
- [ ] 3.1.3 Migrate activities pages
- [ ] 3.1.4 Migrate staff/teacher pages
- [ ] 3.1.5 Migrate config/settings pages
- [ ] 3.1.6 Update all remaining API calls to v2

### 3.2 Frontend Validation (6 hours)
- [ ] 3.2.1 Test all migrated features manually
- [ ] 3.2.2 Run frontend type checks: `npm run typecheck`
- [ ] 3.2.3 Run frontend lint: `npm run lint`
- [ ] 3.2.4 Smoke test entire application
- [ ] 3.2.5 Verify zero v1 API calls remaining

## MONTH 4: Deprecation Period (8 hours)

### 4.1 Monitor v1 Usage (4 hours)
- [ ] 4.1.1 Add logging for all v1 endpoint calls
- [ ] 4.1.2 Monitor for external clients still using v1
- [ ] 4.1.3 Contact external clients if v1 usage detected
- [ ] 4.1.4 Verify internal usage is zero

### 4.2 Prepare for v1 Removal (4 hours)
- [ ] 4.2.1 Send final deprecation notice (2 weeks before removal)
- [ ] 4.2.2 Update documentation to remove v1 references
- [ ] 4.2.3 Prepare rollback plan if needed
- [ ] 4.2.4 Schedule v1 removal deployment

## MONTH 5: Cleanup (8 hours)

### 5.1 Remove v1 Code (6 hours)
- [ ] 5.1.1 Remove `/api/v1/` router from `api/base.go`
- [ ] 5.1.2 Remove adapter layer package `api/adapters/v1tov2/`
- [ ] 5.1.3 Remove 110 deprecated endpoint handlers
- [ ] 5.1.4 Remove v1-specific tests
- [ ] 5.1.5 Update routes.md (should show 108 v2 endpoints only)

### 5.2 Final Validation (2 hours)
- [ ] 5.2.1 Run full backend test suite
- [ ] 5.2.2 Run Bruno tests (v2 only)
- [ ] 5.2.3 Verify frontend works correctly
- [ ] 5.2.4 Measure final endpoint count: confirm 108
- [ ] 5.2.5 Generate final API documentation

## Success Criteria

- ✅ API endpoints reduced from 218 to 108 (50% reduction)
- ✅ All filtering uses query parameters (no /by-{field} endpoints)
- ✅ All state changes use PATCH (no action verb endpoints)
- ✅ No duplicate access paths (one canonical route per resource)
- ✅ Maximum 3 levels of URL nesting
- ✅ v1 API deprecated and removed after transition
- ✅ Frontend 100% migrated to v2
- ✅ Zero breaking changes for external clients (3-month notice given)
- ✅ All Bruno tests pass against v2 in <300ms
- ✅ API documentation updated and accurate
- ✅ Query parameter validation prevents injection
