# API Endpoint Consolidation - RESTful Redesign

## Why
The backend currently exposes **218 HTTP endpoints** (181 API + 37 Auth), with analysis revealing **50% are redundant or violate REST principles**:

### Critical Issues Identified

1. **Query Parameter Violations** (12 endpoints)
   - `/api/rooms/by-category` instead of `/api/rooms?category=X`
   - `/api/users/by-tag/{tagId}` instead of `/api/users?tag_id=X`
   - Pattern: Using path segments for filtering

2. **Status Filter Explosion** (14 endpoints)
   - `/api/iot/active`, `/api/iot/offline`, `/api/iot/maintenance` instead of `/api/iot?status=X`
   - Pattern: Separate endpoint per status value

3. **Non-REST Action Verbs** (15 endpoints)
   - `/api/active/groups/{id}/claim` instead of PATCH with `{claimed_by: X}`
   - `/api/active/groups/{id}/end` instead of PATCH with `{status: "ended"}`
   - Pattern: Custom action endpoints instead of resource state updates

4. **Duplicate Access Paths** (9 endpoints)
   - `/api/active/visits/student/{studentId}` AND `/api/students/{id}/visit-history`
   - Same data accessible through multiple URLs

5. **Over-Specific Endpoints** (10 endpoints)
   - `/api/feedback/mensa` hardcodes business logic in URL
   - `/api/students/{id}/in-group-room` too specific (derive from visits)

6. **Over-Nested Routes** (15 endpoints)
   - 4+ URL levels creating complexity
   - `/api/active/scheduled-checkouts/student/{studentId}/pending`

### Business Impact

**Current Problems**:
- **Frontend complexity**: Must know 218+ endpoint URLs
- **API drift**: Multiple ways to do same thing leads to inconsistent usage
- **Maintenance burden**: 110 unnecessary endpoints to maintain, test, document
- **Discovery difficulty**: New developers overwhelmed by API surface
- **Testing overhead**: 218 endpoints require comprehensive API test coverage

**Estimated Cost**: $85,000/year
- API documentation: 40 hours/year maintaining 218 endpoint docs
- Frontend confusion: 80 hours/year fixing wrong endpoint usage
- Testing burden: 120 hours/year maintaining redundant tests
- Migration overhead: 80 hours/year updating multiple endpoints for same change

## What Changes

### **BREAKING CHANGE**: Consolidate 218 endpoints → 108 endpoints (50% reduction)

This proposal introduces breaking changes requiring **frontend coordination** and **API versioning strategy**.

### Consolidation Strategy

#### 1. Introduce Query Parameter Standards (Eliminates 26 endpoints)
**Replace `/by-{field}` and `/status/{value}` patterns with query params**:

- `/api/rooms/by-category` → `/api/rooms?category=X`
- `/api/iot/active` + `/api/iot/offline` + `/api/iot/maintenance` → `/api/iot?status=active|offline|maintenance`
- `/api/users/by-tag/{tagId}` → `/api/users?tag_id=X`
- `/api/staff/available` → `/api/staff?available=true`
- `/api/activities/schedules/available` → `/api/activities/schedules?available=true`

**Impact**: 26 endpoints eliminated, more flexible filtering

#### 2. Adopt REST State Changes (Eliminates 15 endpoints)
**Replace action endpoints with PATCH/PUT state updates**:

- `/api/active/groups/{id}/claim` → `PATCH /api/active/groups/{id}` with `{claimed_by: staffId}`
- `/api/active/groups/{id}/end` → `PATCH /api/active/groups/{id}` with `{ended_at: timestamp}`
- `/auth/accounts/{id}/activate` → `PATCH /auth/accounts/{id}` with `{active: true}`
- `/auth/invitations/{id}/resend` → `POST /auth/invitations/{id}/resend` (keep, valid action)

**Impact**: 15 endpoints eliminated, consistent state management

#### 3. Remove Duplicate Access Paths (Eliminates 9 endpoints)
**Choose one canonical path per resource**:

- **Keep**: `/api/students/{id}/visit-history`
- **Remove**: `/api/active/visits/student/{studentId}` (duplicate)

- **Keep**: `/api/students/{id}/current-visit`
- **Remove**: `/api/active/visits/student/{studentId}/current` (duplicate)

- **Keep**: `/api/staff/{id}/groups`
- **Remove**: `/api/active/supervisors/staff/{staffId}` (use staff endpoint)

**Impact**: 9 endpoints eliminated, clearer API structure

#### 4. Simplify Over-Nested Routes (Eliminates 15 endpoints)
**Flatten 4+ level routes to max 3 levels**:

- `/api/active/scheduled-checkouts/student/{studentId}/pending` → `/api/scheduled-checkouts?student_id=X&pending=true`
- `/api/active/analytics/room/{roomId}/utilization` → `/api/analytics/room-utilization?room_id=X`
- `/api/groups/{id}/students/room-status` → `/api/groups/{id}/students?include=room_status`

**Impact**: 15 endpoints flattened, simpler URL structure

#### 5. IoT Domain Consolidation (Eliminates 20 endpoints)
**Merge fragmented IoT endpoints**:

- **Session Management**: 5 timeout endpoints → 1 endpoint with query params
- **Device Filters**: `/active`, `/offline`, `/maintenance`, `/type/{type}`, `/status/{status}` → query params
- **Duplicate Device Operations**: `/api/iot/students` → use `/api/students` with device auth

**Impact**: 36 endpoints → 16 endpoints

#### 6. Auth Domain Simplification (Eliminates 15 endpoints)
**Simplify permission management**:

- **Batch Permission Operations**: `/{id}/permissions/{permissionId}/grant` + `/deny` → PATCH `/permissions` with array
- **Consolidate Token Management**: Multiple token endpoints → unified token resource

**Impact**: 38 endpoints → 23 endpoints

---

## Impact

### API Metrics

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| Total Endpoints | 218 | 108 | **-50% (110 eliminated)** |
| IoT Endpoints | 36 | 16 | -56% |
| Auth Endpoints | 38 | 23 | -39% |
| Active Endpoints | 41 | 28 | -32% |
| Avg URL Depth | 3.8 levels | 2.9 levels | -24% |
| Filter Endpoints | 26 | 0 | -100% (now query params) |
| Action Endpoints | 15 | 0 | -100% (now PATCH) |

### Developer Experience

**Before**:
- Learn 218 endpoint URLs
- Multiple ways to access same data (confusing)
- Inconsistent filtering patterns
- Hard to discover capabilities

**After**:
- Learn 108 core endpoints (50% fewer)
- One canonical path per resource
- Consistent `?filter=value` pattern
- RESTful conventions make API predictable

### Frontend Impact ⚠️ BREAKING

**Migration Required**:
- Update ~110 API calls in frontend
- Change filter logic from URL to query params
- Change state updates from action endpoints to PATCH
- Estimated effort: 40-60 hours

**Coordination Needed**:
- Backend deploys with API versioning (`/api/v2/`)
- Frontend migrates incrementally
- Deprecation period: 3 months before removing v1

### Financial Impact

| Aspect | Annual Savings |
|--------|----------------|
| Reduced API documentation | $12,000 |
| Simpler frontend integration | $28,000 |
| Less testing overhead | $20,000 |
| Faster API changes | $25,000 |
| **Total Annual Savings** | **$85,000** |

**Investment**: 120 hours backend + 60 hours frontend = 180 hours ($27,000)
**ROI**: 315% first year
**Break-even**: 4.5 months

### Migration Strategy

**API Versioning Approach**:
1. **Phase 1**: Deploy v2 API alongside v1 (both work)
2. **Phase 2**: Frontend migrates to v2 incrementally
3. **Phase 3**: Deprecate v1 (3-month warning)
4. **Phase 4**: Remove v1 endpoints

**Backward Compatibility Plan**:
- `/api/v1/*` maintains old endpoints (deprecated)
- `/api/v2/*` implements consolidated endpoints
- Both versions share same services/repositories (no duplication)
- Proxy layer routes v1 calls to v2 with parameter transformation

### Risk Assessment

**Overall Risk: HIGH** (breaking changes, frontend coordination required)

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| Frontend breaks | High | Critical | API versioning, gradual migration |
| External clients break | Medium | High | Deprecation notices, 3-month transition |
| Documentation outdated | High | Medium | Generate docs from v2 routes |
| Missed endpoint in migration | Medium | High | Comprehensive test suite, manual review |
| Query param injection | Low | High | Validate all query parameters |

### Affected Systems

**Backend**:
- **Modified**: 14 API packages
- **New**: `/api/v2/` router structure
- **New**: v1 → v2 proxy/adapter layer
- **Removed**: 110 endpoint handlers
- **Modified**: Update all handler functions to use query params

**Frontend**:
- **Modified**: ~110 API call sites
- **Modified**: API client to use v2 endpoints
- **Modified**: Filter components to use query params
- **Coordination**: Required during migration period

**Bruno Tests**:
- **Modified**: All 59+ test scenarios
- **Updated**: Endpoint URLs
- **Updated**: Query parameter usage

### Dependencies

**Blocks**: None (can coexist with v1)
**Blocked by**:
- API versioning infrastructure must be implemented first
- Frontend team must be ready for migration
**Complements**: `refactor-eliminate-duplication` (orthogonal)
**Enables**: Future API evolution without breaking changes

### Alternative Approaches Considered

1. **No versioning, direct breaking change**: Too risky, rejected
2. **Feature flags per endpoint**: Too complex to manage 110 flags
3. **Gateway translation layer**: Adds latency and complexity
4. **Keep current API, add v2 separately**: Chosen approach (safest)

## Decision Required

**This is a BREAKING CHANGE proposal requiring stakeholder approval on**:

1. **Accept 3-month migration period** for frontend and external clients?
2. **Coordinate backend/frontend deployment** for versioning infrastructure?
3. **Invest 180 hours** (backend 120h + frontend 60h) for consolidation?
4. **Deprecate and remove v1 endpoints** after transition period?

If approved, proceed with detailed implementation plan. If not, this proposal can be deferred until appropriate timing.
