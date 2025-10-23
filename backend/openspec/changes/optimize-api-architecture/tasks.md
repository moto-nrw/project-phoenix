# API Architecture Optimization - Implementation Tasks

## Phase 1: Endpoint Audit & Deletion (Weeks 1-2)

### 1.1 Endpoint Usage Tracking
- [ ] 1.1.1 Create `middleware/usage_tracking.go` with async tracking logic
- [ ] 1.1.2 Create `models/metrics/endpoint_usage.go` model
- [ ] 1.1.3 Add database migration for `metrics.endpoint_usage` table
- [ ] 1.1.4 Register usage tracking middleware in `api/base.go`
- [ ] 1.1.5 Deploy to production and monitor for 7 days
- [ ] 1.1.6 Verify tracking via `SELECT * FROM metrics.endpoint_usage` query

### 1.2 Unused Endpoint Identification
- [ ] 1.2.1 Create SQL query to identify endpoints with <10 calls in 14 days
- [ ] 1.2.2 Cross-reference with frontend source code (grep for endpoint paths)
- [ ] 1.2.3 Generate deletion candidate report (markdown file)
- [ ] 1.2.4 Review report with team and confirm safe to delete

### 1.3 Endpoint Deletion
- [ ] 1.3.1 Delete unused endpoint handlers from `api/` domain routers
- [ ] 1.3.2 Remove corresponding service methods (if only used by deleted endpoint)
- [ ] 1.3.3 Update OpenAPI spec via `go run main.go gendoc`
- [ ] 1.3.4 Run Bruno API tests to ensure no regressions: `cd bruno && bru run --env Local 0*.bru`
- [ ] 1.3.5 Create PR with detailed list of deleted endpoints
- [ ] 1.3.6 Deploy and monitor error rates for 48 hours

## Phase 2: Performance Quick Wins (Weeks 3-4)

### 2.1 Redis Integration
- [ ] 2.1.1 Add Redis service to `docker-compose.yml`
- [ ] 2.1.2 Create `cache/redis.go` wrapper with Get/Set/Delete methods
- [ ] 2.1.3 Add Redis configuration to `dev.env` (REDIS_URL, REDIS_PASSWORD)
- [ ] 2.1.4 Initialize Redis client in `services/factory.go`
- [ ] 2.1.5 Add unit tests for cache wrapper (mock Redis)
- [ ] 2.1.6 Test Redis connection on service startup

### 2.2 Dashboard Analytics Caching
- [ ] 2.2.1 Update `ActiveService.GetDashboardAnalytics()` to check cache first
- [ ] 2.2.2 Set cache TTL to 30 seconds for dashboard data
- [ ] 2.2.3 Add cache invalidation on `EndActiveGroup()` and `CreateVisit()`
- [ ] 2.2.4 Implement fallback to database if Redis unavailable
- [ ] 2.2.5 Add logging for cache hits/misses
- [ ] 2.2.6 Load test: Verify 200ms → <50ms for dashboard endpoint

### 2.3 N+1 Query Fixes (Active Domain)
- [ ] 2.3.1 Fix `VisitRepository.ListByGroup()` to use `.Relation("Student.Person")`
- [ ] 2.3.2 Fix `ActiveGroupRepository.ListWithOptions()` to eager load Room
- [ ] 2.3.3 Fix `SupervisorRepository.ListByGroup()` to eager load Staff.Person
- [ ] 2.3.4 Add query counting test: Assert max 5 queries per endpoint
- [ ] 2.3.5 Run Bruno API tests to verify response format unchanged
- [ ] 2.3.6 Performance test: Verify 50 visits load in <100ms (previously ~500ms)

### 2.4 Database Index Creation
- [ ] 2.4.1 Create migration to add `idx_visits_student_id` on `active.visits(student_id)`
- [ ] 2.4.2 Create migration to add `idx_visits_group_id` on `active.visits(active_group_id)`
- [ ] 2.4.3 Create migration to add `idx_visits_checkin_time` on `active.visits(check_in_time)`
- [ ] 2.4.4 Run `EXPLAIN ANALYZE` on visit queries to verify index usage
- [ ] 2.4.5 Deploy indexes during low-traffic window (indexes created concurrently)
- [ ] 2.4.6 Verify query performance <10ms for filtered visit queries

### 2.5 Performance Monitoring
- [ ] 2.5.1 Add slow query logging middleware (threshold: 200ms)
- [ ] 2.5.2 Create metrics endpoint `/api/metrics/performance` (admin only)
- [ ] 2.5.3 Add cache hit rate tracking to Redis wrapper
- [ ] 2.5.4 Set up alert for cache hit rate <80%
- [ ] 2.5.5 Document performance baseline (p50, p95, p99 for each endpoint)

## Phase 3: Mobile BFF Layer (Weeks 5-8)

### 3.1 BFF Package Structure
- [ ] 3.1.1 Create `api/bff/mobile/` package directory
- [ ] 3.1.2 Create `api/bff/mobile/factory.go` with service dependencies
- [ ] 3.1.3 Create `api/bff/mobile/api.go` with Chi router
- [ ] 3.1.4 Register BFF router in `api/base.go` under `/mobile/v1`
- [ ] 3.1.5 Add integration test for BFF router registration

### 3.2 Dashboard Aggregation Endpoint
- [ ] 3.2.1 Create `dashboard.go` with `GetDashboard()` handler
- [ ] 3.2.2 Implement parallel service calls using goroutines + WaitGroup
- [ ] 3.2.3 Aggregate analytics, active groups, and students into single response
- [ ] 3.2.4 Add error handling for partial failures (return available data + error indicators)
- [ ] 3.2.5 Add integration test: Assert response includes all sections
- [ ] 3.2.6 Performance test: Assert <200ms p95 for dashboard aggregation

### 3.3 Batch Query Endpoint
- [ ] 3.3.1 Create `batch.go` with `ExecuteBatchQuery()` handler
- [ ] 3.3.2 Parse batch query request: `{"queries": [...]}`
- [ ] 3.3.3 Execute queries in parallel with 10-query limit
- [ ] 3.3.4 Return results in same order as request
- [ ] 3.3.5 Handle individual query failures gracefully
- [ ] 3.3.6 Add integration test with 5 queries in one batch

### 3.4 Sparse Fieldsets Implementation
- [ ] 3.4.1 Create `fieldsets.go` with field parsing logic
- [ ] 3.4.2 Update `students.go` BFF endpoint to support `?fields[students]=id,name`
- [ ] 3.4.3 Implement database-level column selection (BUN `.Column()`)
- [ ] 3.4.4 Add validation for invalid field names (return 400)
- [ ] 3.4.5 Add integration test: Verify only requested fields in response
- [ ] 3.4.6 Payload size test: Assert ~90% reduction with minimal fieldset

### 3.5 Cursor Pagination
- [ ] 3.5.1 Create `pagination.go` with cursor encoding/decoding logic
- [ ] 3.5.2 Implement `?cursor=...&limit=50` support in list endpoints
- [ ] 3.5.3 Return `meta.next_cursor` and `meta.has_more` in response
- [ ] 3.5.4 Handle first page without cursor (default to first 50 results)
- [ ] 3.5.5 Add integration test: Verify cursor pagination across 3 pages
- [ ] 3.5.6 Edge case test: Verify `has_more=false` when no more results

### 3.6 OpenAPI Spec for Mobile
- [ ] 3.6.1 Document BFF endpoints in OpenAPI spec (separate section)
- [ ] 3.6.2 Include request/response examples for dashboard aggregation
- [ ] 3.6.3 Document sparse fieldsets syntax in OpenAPI
- [ ] 3.6.4 Generate TypeScript types for mobile team: `openapi-typescript`
- [ ] 3.6.5 Create mobile API documentation README

## Phase 4: Selective Consolidation (Weeks 9-12)

### 4.1 V2 Router Structure
- [ ] 4.1.1 Create `api/v2/` package directory
- [ ] 4.1.2 Create `api/v2/active/` with consolidated router
- [ ] 4.1.3 Register V2 router in `api/base.go` under `/api/v2`
- [ ] 4.1.4 Add deprecation headers middleware for V1 endpoints
- [ ] 4.1.5 Integration test: Verify V1 and V2 both functional

### 4.2 Query Parameter Filtering (Active Domain)
- [ ] 4.2.1 Create `middleware/query_filter_auth.go` for permission validation
- [ ] 4.2.2 Update `ActiveGroupRepository` to support `status`, `room_id`, `group_id` filters
- [ ] 4.2.3 Implement V2 endpoint: `GET /api/v2/active/groups` with query params
- [ ] 4.2.4 Add permission check for each query parameter (e.g., validate room access)
- [ ] 4.2.5 Integration test: Verify filtering works correctly
- [ ] 4.2.6 Security test: Attempt unauthorized filter parameter (should return 403)

### 4.3 Visit Endpoint Consolidation
- [ ] 4.3.1 Create V2 visit endpoint: `GET /api/v2/visits?group_id=...&student_id=...`
- [ ] 4.3.2 Replace 3 specialized V1 endpoints with single V2 parameterized endpoint
- [ ] 4.3.3 Add database-level filtering (not application-level)
- [ ] 4.3.4 Maintain V1 endpoints as wrappers calling V2 logic (backward compat)
- [ ] 4.3.5 Bruno test: Verify V2 endpoint response matches V1 format

### 4.4 Supervisor Endpoint Consolidation
- [ ] 4.4.1 Create V2 supervisor endpoint: `GET /api/v2/supervisors?group_id=...&staff_id=...`
- [ ] 4.4.2 Consolidate 3 specialized V1 endpoints
- [ ] 4.4.3 Add permission validation for query parameters
- [ ] 4.4.4 Integration test: Verify filtering correctness

### 4.5 Frontend Migration (Optional)
- [ ] 4.5.1 Update `lib/active-api.ts` to use V2 endpoints
- [ ] 4.5.2 Update `lib/student-api.ts` to use query param filtering
- [ ] 4.5.3 Test in development environment
- [ ] 4.5.4 Deploy frontend changes
- [ ] 4.5.5 Monitor error rates for 48 hours
- [ ] 4.5.6 Roll back to V1 if issues detected

### 4.6 V1 Deprecation Notice
- [ ] 4.6.1 Add `X-API-Deprecated: true` header to all V1 endpoints
- [ ] 4.6.2 Add `X-API-Sunset-Date: 2025-07-01` header (6 months from now)
- [ ] 4.6.3 Update API documentation with migration guide
- [ ] 4.6.4 Send email notification to API consumers about V1 deprecation
- [ ] 4.6.5 Monitor V1 vs V2 usage metrics

## Validation & Testing

### Bruno API Test Updates
- [ ] V.1 Update Bruno tests to cover new BFF endpoints
- [ ] V.2 Add tests for cursor pagination edge cases
- [ ] V.3 Add tests for sparse fieldsets validation
- [ ] V.4 Add tests for query parameter filtering
- [ ] V.5 Verify all 59 existing scenarios still pass
- [ ] V.6 Run full test suite after each phase: `cd bruno && bru run --env Local 0*.bru`

### Load Testing
- [ ] L.1 Set up JMeter test plan with 150 concurrent users
- [ ] L.2 Simulate realistic endpoint mix (80% reads, 20% writes)
- [ ] L.3 Run load test against dashboard analytics (verify <50ms p95)
- [ ] L.4 Run load test against visit list endpoint (verify <100ms p95)
- [ ] L.5 Monitor database connection pool utilization under load

### Performance Benchmarking
- [ ] P.1 Baseline measurement: Record current p50, p95, p99 for all endpoints
- [ ] P.2 Post-Phase 2 measurement: Verify 10x improvement for dashboard
- [ ] P.3 Post-Phase 3 measurement: Verify BFF <200ms p95
- [ ] P.4 Post-Phase 4 measurement: Verify consolidated endpoints maintain <100ms
- [ ] P.5 Compare before/after database query counts (assert reduction)

### Security Testing
- [ ] S.1 Attempt SQL injection via query parameters (should be blocked)
- [ ] S.2 Attempt to bypass permissions via query params (should return 403)
- [ ] S.3 Attempt to access unauthorized student data (should be blocked by policy)
- [ ] S.4 Verify cache doesn't leak data between users (different JWT tokens)
- [ ] S.5 Verify IoT endpoints remain unchanged (no accidental modifications)

## Documentation

- [ ] D.1 Update `README.md` with API versioning information
- [ ] D.2 Update `CLAUDE.md` with new performance patterns
- [ ] D.3 Create migration guide from V1 to V2 API
- [ ] D.4 Update OpenAPI spec via `go run main.go gendoc`
- [ ] D.5 Document Redis caching strategy and TTL values
- [ ] D.6 Create mobile BFF integration guide for mobile team

## Deployment

- [ ] Deploy.1 Phase 1: Backend changes only (invisible to users, no frontend changes)
- [ ] Deploy.2 Phase 2: Redis + performance fixes (backward compatible)
- [ ] Deploy.3 Phase 3: Mobile BFF available (additive, no breaking changes)
- [ ] Deploy.4 Phase 4: V2 API available (V1 stays functional)
- [ ] Deploy.5 Monitor error rates after each deployment (rollback if >1% error rate)
- [ ] Deploy.6 Document rollback procedures for each phase
