# API Architecture Optimization

## Why

Project Phoenix currently has 220 REST API endpoints across 17 domains. Deep
analysis reveals that ~70 endpoints (32%) are unused by the frontend, and the
API design suffers from premature complexity that doesn't match the project's
scale (120 students, 30 staff).

**Problems Identified:**

1. **Over-Engineering:** 220 endpoints for a small institution creates
   maintenance burden
2. **No Caching:** Dashboard analytics queries take 200ms due to lack of caching
   layer
3. **N+1 Query Patterns:** BUN ORM Relation() used inconsistently, causing
   performance issues
4. **No Mobile Optimization:** Future mobile app will require different response
   shapes
5. **IoT API Instability Risk:** Mixed with web API, making it fragile for
   firmware updates

**Opportunity:** Restructure the API into a pragmatic 3-tier architecture that
reduces endpoints by 45% while improving performance 10x through strategic
caching and query optimization.

## What Changes

### Phase 1: Endpoint Audit & Deletion (Week 1-2)

- **Delete 70 unused endpoints** identified through frontend usage analysis
- Add endpoint usage tracking middleware to prevent future dead code
- **Result:** 220 → 150 endpoints with zero breaking changes for active users

### Phase 2: Performance Quick Wins (Week 3-4)

- **Add Redis caching** for dashboard analytics (200ms → 5ms, 40x speedup)
- **Fix N+1 queries** in Active domain using BUN Relation() eager loading
- **Add database indexes** for frequently queried columns
- **Result:** 10x performance improvement with minimal code changes

### Phase 3: Mobile BFF Layer (Week 5-8)

- **Create `/mobile/v1` endpoint tier** optimized for mobile app responses
- Aggregate multiple API calls into single responses (e.g., dashboard data)
- Support sparse fieldsets and cursor pagination
- Keep core API unchanged (web + IoT compatibility preserved)
- **Result:** Mobile-ready API without breaking existing clients

### Phase 4: Selective Consolidation (Week 9-12)

- **Consolidate simple list endpoints** using query param filtering (30
  endpoints → 10 endpoints)
- **Keep complex operations hierarchical** (combined groups, scheduled
  checkouts, substitutions)
- **Add API versioning** (`/api/v1` and `/api/v2` coexist) for backward
  compatibility
- **Result:** 150 → 120 endpoints through natural consolidation

## Impact

### Affected Specs (To Be Created)

- `specs/api-management/` - Core REST API architecture and versioning
- `specs/mobile-bff/` - Mobile Backend-For-Frontend patterns
- `specs/performance/` - Caching strategy and query optimization

### Affected Code

- `api/` - All domain routers (selective changes, not wholesale rewrite)
- `services/active/` - N+1 query fixes, caching integration
- `middleware/` - Usage tracking, query param validation
- `database/repositories/` - Eager loading patterns
- Frontend minimal impact (most changes backward compatible)

### Breaking Changes

**NONE in Phase 1-3**

- Phase 4 introduces `/api/v2` endpoints (v1 remains available)
- IoT endpoints frozen (never change due to firmware constraints)
- SSE endpoints unchanged (working fine, don't fix)

### Performance Impact

- **Dashboard load time:** 200ms → 20ms (10x improvement)
- **API response time:** Maintain <100ms p95 for reads
- **Memory:** +50MB for Redis cache (negligible)
- **Database queries:** N+1 patterns eliminated (151 queries → 1 query in worst
  cases)

### Migration Path

- **Week 1-4:** Backend changes only (invisible to users)
- **Week 5-8:** Mobile BFF available (web unchanged)
- **Week 9-10:** `/api/v2` endpoints available alongside `/api/v1`
- **Week 11-12:** Frontend migrates to `/api/v2` (optional, v1 stays)
- **Month 6:** Deprecate `/api/v1` (after mobile app launch)

### Non-Goals

- ❌ WebSocket migration (SSE works fine, only 2 pages use it)
- ❌ GraphQL/gRPC adoption (overkill for 150 concurrent users)
- ❌ Sparse fieldsets everywhere (add to mobile BFF only)
- ❌ Full API flattening (violates domain boundaries)
- ❌ Cursor pagination everywhere (hybrid approach instead)

## Success Metrics

### Performance (Measured at end of Phase 2)

- Dashboard analytics: <50ms p95 (currently 200ms)
- List endpoints: <100ms p95 (currently 120ms)
- Database query count: <5 queries/request average (currently 10+)

### Code Health (Measured at end of Phase 4)

- Active endpoints: 120 (down from 220)
- Test coverage: 75%+ for critical paths
- golangci-lint warnings: 0 (maintain current standard)

### Developer Experience

- API endpoint discovery: 45% fewer endpoints to navigate
- Mobile dev time: 50% faster (aggregated responses)
- Frontend migration: <1 week per domain (v1/v2 coexistence)

## Risk Assessment

### High Risk Mitigation

- **IoT firmware compatibility:** Freeze `/iot/*` endpoints completely
- **Data integrity:** Use transactions for all multi-table operations
- **GDPR compliance:** Audit all changes for privacy policy impact

### Medium Risk Mitigation

- **N+1 query fixes:** Test with production-like dataset (120 students)
- **Redis failure:** Graceful fallback to database queries
- **V1/V2 coexistence:** Comprehensive integration tests

### Low Risk Areas

- Endpoint deletion (unused endpoints have no impact)
- Performance caching (can be disabled if issues arise)
- Mobile BFF (additive only, no changes to core API)

## Timeline

**Total Duration:** 12 weeks (3 months)

- **Phase 1:** Weeks 1-2 (Audit & Delete)
- **Phase 2:** Weeks 3-4 (Performance)
- **Phase 3:** Weeks 5-8 (Mobile BFF)
- **Phase 4:** Weeks 9-12 (Consolidation)

**Effort:** 1 backend developer full-time, 0.5 frontend developer for Phase 4

**Validation:** Bruno API tests run after each phase to ensure no regressions
