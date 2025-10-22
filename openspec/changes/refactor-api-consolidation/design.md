# API Endpoint Consolidation - Technical Design

## Context

### Current API Surface
- **218 total endpoints** (181 /api/* + 37 /auth/*)
- **~50% violate REST principles** (query params as paths, action verbs, duplicate access)
- **Multiple patterns for same operation** (inconsistent, confusing)
- **Deep nesting** (4-5 levels in URLs)

### Investigation Findings
Deep-dive analysis revealed **110 endpoints can be eliminated** through:
- Query parameter adoption (26 endpoints)
- REST state changes via PATCH (15 endpoints)
- Removing duplicate access paths (9 endpoints)
- Simplifying over-nested routes (15 endpoints)
- IoT domain consolidation (20 endpoints)
- Auth simplification (15 endpoints)
- Over-specific endpoints (10 endpoints)

### Business Drivers
- **Reduce frontend complexity**: Fewer endpoints to learn and maintain
- **Improve API discoverability**: RESTful conventions make API predictable
- **Lower testing burden**: 110 fewer endpoints to test
- **Enable future evolution**: Versioning strategy allows safe changes

### Constraints
- **Must not break existing clients** during transition
- **Frontend coordination required** for migration
- **3-month deprecation period** minimum
- **Zero data loss** during migration
- **Performance maintained** (Bruno tests <300ms)

## Goals / Non-Goals

### Goals
1. **Reduce API surface from 218 to 108 endpoints** (50% reduction)
2. **Adopt consistent REST principles** (query params for filtering, PATCH for state)
3. **Eliminate duplicate access paths** (one canonical URL per resource)
4. **Implement API versioning** (`/api/v1/` and `/api/v2/`)
5. **Provide safe migration path** (both versions work during transition)
6. **Improve API documentation** (clearer, more predictable structure)

### Non-Goals
1. **GraphQL migration**: Staying with REST
2. **Complete API redesign**: Only consolidating existing endpoints
3. **Business logic changes**: Functionality remains identical
4. **Database changes**: No schema modifications
5. **Authentication changes**: Auth mechanism unchanged

## Decisions

### Decision 1: API Versioning Strategy
**Choice**: Path-based versioning (`/api/v2/`) with v1 proxy to v2

**Rationale**:
- **Clear separation**: v1 and v2 coexist without conflicts
- **Simple routing**: Chi router easily distinguishes versions
- **Gradual migration**: Frontend can migrate endpoint-by-endpoint
- **Easy deprecation**: Remove v1 router when ready

**Implementation**:
```go
// api/base.go
r.Route("/api/v1", func(r chi.Router) {
    // Proxy to v2 with parameter transformation
    r.Mount("/users", adaptV1ToV2(v2.Users.Router()))
})

r.Route("/api/v2", func(r chi.Router) {
    // New consolidated endpoints
    r.Mount("/users", v2.Users.Router())
})
```

**Alternatives Considered**:
- **Header versioning** (`Accept: application/vnd.api.v2`): More complex for clients
- **Query param versioning** (`?api_version=2`): Easy to forget, less clear
- **Subdomain versioning** (`v2.api.example.com`): Overkill for single app

### Decision 2: v1-to-v2 Adapter Pattern
**Choice**: Thin adapter layer transforms v1 calls to v2 format

**Rationale**:
- **No code duplication**: v1 and v2 share same services/repositories
- **Maintainable**: Changes only needed in v2, v1 adapts automatically
- **Testable**: Adapter logic isolated and unit testable
- **Removable**: Clean removal when v1 deprecated

**Example**:
```go
// v1: GET /api/v1/rooms/by-category/{category}
// v2: GET /api/v2/rooms?category={category}

func AdaptRoomsByCategoryToV2(v2Handler http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        category := chi.URLParam(r, "category")
        q := r.URL.Query()
        q.Set("category", category)
        r.URL.RawQuery = q.Encode()
        r.URL.Path = strings.Replace(r.URL.Path, "/v1/rooms/by-category/"+category, "/v2/rooms", 1)
        v2Handler.ServeHTTP(w, r)
    })
}
```

**Alternatives Considered**:
- **Duplicate handler implementations**: Rejected (doubles maintenance)
- **Gateway service**: Rejected (adds latency, operational complexity)

### Decision 3: Query Parameter Standards
**Choice**: Consistent query param naming across all APIs

**Standards**:
- **Filtering**: `?field_name=value` (snake_case)
- **Status**: `?status=active|inactive|archived`
- **Boolean flags**: `?available=true|false`, `?active=true|false`
- **Relationships**: `?include=related_entity`
- **Pagination**: `?page=1&page_size=50` (already consistent)
- **Sorting**: `?sort=field&order=asc|desc`

**Example**:
```
GET /api/v2/iot?status=offline&type=rfid&include=location
GET /api/v2/rooms?available=true&category=classroom
GET /api/v2/staff?active=true&sort=last_name&order=asc
```

### Decision 4: PATCH for State Changes
**Choice**: Use PATCH with partial updates for resource state changes

**Pattern**:
```go
// Old v1: POST /api/active/groups/{id}/end
// New v2: PATCH /api/active/groups/{id}
// Body: {"status": "ended", "ended_at": "2025-01-22T10:30:00Z"}

// Old v1: POST /auth/accounts/{id}/activate
// New v2: PATCH /auth/accounts/{id}
// Body: {"active": true}
```

**Rationale**:
- **RESTful**: State changes are resource updates
- **Flexible**: Can update multiple fields in one call
- **Clear semantics**: PATCH = partial update, PUT = full replacement
- **Idempotent**: Can retry safely

### Decision 5: Canonical Resource Paths
**Choice**: One primary path per resource, remove alternate access routes

**Rules**:
1. **Resource-centric URLs**: `/api/{resource}/{id}` is primary
2. **Nested resources**: `/api/{parent}/{id}/{child}` for owned relationships
3. **No sibling nesting**: Don't access child through multiple parents
4. **Query for filtering**: Use query params, not path segments

**Examples**:
- **Keep**: `/api/students/{id}/visits` (student owns visits)
- **Remove**: `/api/visits/student/{studentId}` (duplicate access)
- **Keep**: `/api/groups/{id}/students` (group contains students)
- **Remove**: `/api/students?group_id=X` would work but less clear for this relationship

### Decision 6: Migration Timeline
**Choice**: 4-month gradual transition with 3-month v1 support

**Timeline**:
- **Month 1**: Implement v2 API, deploy alongside v1
- **Month 2**: Frontend begins migration (50% complete)
- **Month 3**: Frontend completes migration (100% on v2)
- **Month 4**: Deprecate v1 (warning headers), monitor usage
- **Month 5**: Remove v1 code

**Rationale**:
- **Safe migration**: Both versions available during transition
- **Low risk**: Can extend timeline if needed
- **Clear deadline**: 3-month deprecation is industry standard
- **Measurable**: Track v1 vs v2 usage metrics

## Risks / Trade-offs

### Risk 1: Frontend migration takes longer than planned
**Likelihood**: Medium | **Impact**: Medium

**Mitigation**:
- Start frontend migration in parallel with v2 backend development
- Provide migration script to auto-update API calls
- Create v2 API client wrapper for frontend
- Can extend v1 support if needed

**Trade-off**: Longer v1 support = more maintenance burden (acceptable if needed)

### Risk 2: Breaking changes missed in v1-to-v2 adapter
**Likelihood**: Medium | **Impact**: High

**Mitigation**:
- Comprehensive test suite for adapter layer
- Run Bruno tests against both v1 and v2
- Monitor error rates in production
- Quick rollback plan if issues found

**Trade-off**: Adapter adds slight complexity (but temporary, removed with v1)

### Risk 3: Query parameter injection vulnerabilities
**Likelihood**: Low | **Impact**: Critical

**Mitigation**:
- Validate all query parameters
- Use parameterized queries (already doing this)
- Input sanitization in QueryOptions
- Security audit of new query param handling

**Trade-off**: More validation code, but necessary for security

### Risk 4: External clients don't migrate
**Likelihood**: Low | **Impact**: Medium

**Mitigation**:
- Communicate deprecation clearly (email, docs, warning headers)
- Provide migration guide with examples
- Track v1 usage metrics
- Can maintain v1 longer if critical external clients

**Trade-off**: Extended v1 support delays cleanup (manageable)

### Risk 5: Performance regression from query param parsing
**Likelihood**: Low | **Impact**: Low

**Mitigation**:
- Benchmark query param parsing overhead
- Optimize parsePagination and filter helpers
- Cache parsed query params in request context
- Monitor API response times post-deployment

**Trade-off**: Minimal CPU overhead (<1ms) acceptable for cleaner API

## Migration Plan

### Month 1: v2 Implementation (120 hours backend)

**Week 1-2: Core v2 Endpoints** (60 hours)
1. Implement v2 router structure
2. Create query parameter standards
3. Migrate 10 high-traffic endpoints to v2
4. Create adapter layer for v1 compatibility

**Week 3-4: Complete v2 Migration** (60 hours)
5. Migrate remaining 108 endpoints
6. Implement all PATCH state changes
7. Remove duplicate access paths
8. Update Bruno tests for v2

**Deliverable**: v2 API fully functional, v1 proxies to v2

### Month 2-3: Frontend Migration (60 hours frontend)

**Month 2: API Client Update** (30 hours)
1. Create v2 API client wrapper
2. Migrate 50% of API calls to v2
3. Test migrated features

**Month 3: Complete Migration** (30 hours)
4. Migrate remaining 50% of calls
5. Remove v1 API client code
6. Update all tests

**Deliverable**: Frontend 100% on v2

### Month 4: Deprecation (8 hours)

**Week 1-2**: Add deprecation warnings to v1
**Week 3-4**: Monitor v1 usage (should be near-zero)

**Deliverable**: v1 ready for removal

### Month 5: Cleanup (4 hours)

1. Remove v1 router and adapters
2. Remove deprecated endpoint handlers
3. Update documentation
4. Celebrate 110 fewer endpoints!

**Deliverable**: Clean v2-only API

## Success Metrics

### Technical Metrics
- Endpoints: 218 → 108 (-50%)
- Avg URL depth: 3.8 → 2.9 levels
- Filter endpoints: 26 → 0 (query params)
- Action endpoints: 15 → 0 (PATCH)
- Response time: <300ms maintained

### Migration Metrics
- v2 adoption: Track % of traffic
- v1 usage: Monitor deprecated endpoint calls
- Error rate: Compare v1 vs v2
- Frontend migration: Track % of pages migrated

### Business Metrics
- API documentation time: -40%
- Frontend API confusion tickets: -60%
- New developer onboarding time: -30%
- API test suite size: -50%

## Open Questions

### Q1: Should we version auth endpoints too?
**Recommendation**: Yes - `/auth/v2/login` for consistency

**Reasoning**: Keeps versioning strategy uniform across all endpoints

### Q2: What deprecation timeline for v1?
**Recommendation**: 3 months after v2 launch

**Reasoning**: Industry standard, gives time for external clients

### Q3: Should we create automated migration script for frontend?
**Recommendation**: Yes - regex-based find/replace for common patterns

**Example**:
```bash
# Migrate by-category patterns
find src -name "*.ts" -exec sed -i 's|/rooms/by-category/\${cat}|/rooms?category=\${cat}|g' {} \;
```

### Q4: Keep any v1 endpoints permanently?
**Recommendation**: No - full cutover to v2

**Reasoning**: Maintaining two versions doubles API surface (defeats purpose)

## References

- Current routes.md: 218 endpoints documented
- REST best practices: Richardson Maturity Model Level 2-3 target
- Industry benchmarks: Similar systems have 80-120 endpoints
- API versioning patterns: Stripe, GitHub, Twilio use path-based versioning
