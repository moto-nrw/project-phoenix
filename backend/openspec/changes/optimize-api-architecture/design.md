# API Architecture Optimization - Technical Design

## Context

Project Phoenix has evolved organically to 220 API endpoints, but analysis shows
significant opportunities for optimization:

**Current State:**

- 220 endpoints across 17 domains
- ~70 unused endpoints (identified via frontend usage analysis)
- No caching layer (dashboard queries: 200ms)
- Inconsistent N+1 query patterns
- Single API tier serves web, mobile (future), and IoT clients equally
- SSE real-time updates on 2 pages only

**Stakeholders:**

- Backend developers (maintenance burden)
- Frontend developers (API complexity)
- Future mobile app developers (need optimized responses)
- IoT device firmware (can't be updated remotely - API must stay stable)
- End users (want faster page loads)

**Constraints:**

- GDPR compliance (SSL, audit logs, data retention)
- Small scale (120 students, 30 staff - not web-scale)
- IoT firmware frozen (ESP32 devices in production)
- Zero tolerance for data integrity issues
- Must maintain <100ms API response time p95

## Goals / Non-Goals

### Goals

1. **Reduce maintenance burden:** 220 → 120 endpoints (45% reduction)
2. **Improve performance:** 10x speedup for dashboard analytics
3. **Enable mobile app:** BFF layer with aggregated responses
4. **Preserve stability:** Zero breaking changes for IoT devices
5. **Maintain GDPR compliance:** All changes audited for privacy impact

### Non-Goals

- ❌ WebSocket migration (SSE works fine for 2 pages)
- ❌ GraphQL/gRPC adoption (overkill for scale)
- ❌ Microservices architecture (premature for 150 concurrent users)
- ❌ Full API rewrite (too risky, unnecessary)
- ❌ Complex sparse fieldsets (add to mobile BFF only when needed)

## Decisions

### Decision 1: 3-Tier API Architecture

**Chosen:** Separate API tiers for different client needs

```
┌──────────────────────────────────────────────────────┐
│  Tier 1: Core REST API (/api/v1, /api/v2)           │
│  - Used by: Web frontend, Admin tools                │
│  - Changes: Delete unused, selective consolidation   │
│  - Stability: High (versioned endpoints)             │
└──────────────────────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────┐
│  Tier 2: Mobile BFF (/mobile/v1)                     │
│  - Used by: Future mobile app                        │
│  - Features: Aggregated responses, sparse fields     │
│  - Stability: Medium (mobile-first iteration)        │
└──────────────────────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────┐
│  Tier 3: IoT Gateway (/iot)                          │
│  - Used by: RFID devices (ESP32 firmware)            │
│  - Changes: NONE (frozen forever)                    │
│  - Stability: Absolute (can't update firmware)       │
└──────────────────────────────────────────────────────┘
```

**Why:**

- Decouples client evolution (mobile can iterate without affecting web)
- Protects IoT stability (no accidental breaking changes)
- Enables client-specific optimization without core API complexity

**Alternatives Considered:**

1. **Single unified API** - Rejected: Leads to complex query params to satisfy
   all clients
2. **GraphQL for all clients** - Rejected: Overkill for scale, IoT can't use it
3. **Microservices** - Rejected: Premature optimization for small team

### Decision 2: Hybrid Consolidation Strategy

**Chosen:** Keep complex operations hierarchical, flatten only simple CRUD

```go
// ✅ KEEP: Complex domain operations with clear boundaries
POST /api/active/groups/:id/end           // Transaction: end group + visits + audit
POST /api/active/combined/:id/end         // Cascade: end all member groups
POST /api/scheduled-checkouts/process     // Background job with batch operations

// ✅ FLATTEN: Simple resource queries
GET /api/active/groups?status=unclaimed&room_id=123  // Simple filtering
GET /api/visits?group_id=123&student_id=456           // Query param filtering

// ❌ DELETE: Redundant specialized endpoints
GET /api/active/groups/unclaimed          // Replaced by ?status=unclaimed
GET /api/active/groups/room/:roomId       // Replaced by ?room_id=:roomId
```

**Why:**

- Preserves domain boundaries for complex operations
- Reduces duplication for simple list variations
- Maintains permission model clarity (complex ops have dedicated middleware)
- Avoids service layer "god classes" from over-consolidation

**Alternatives Considered:**

1. **Flatten everything** - Rejected: Violates domain boundaries, increases
   complexity
2. **Keep everything as-is** - Rejected: Doesn't solve maintenance burden
3. **GraphQL** - Rejected: Adds framework complexity for marginal benefit

### Decision 3: Redis Caching Strategy

**Chosen:** Strategic caching for read-heavy analytics endpoints only

```go
type CacheConfig struct {
    DashboardAnalytics: 30 * time.Second  // Aggregates across 8 tables
    RoomUtilization:    60 * time.Second  // Live room occupancy
    StudentLocations:   10 * time.Second  // Real-time student tracking

    // No caching (real-time writes):
    Visits:            0                  // Check-in/out must be immediate
    Supervisors:       0                  // Assignment changes immediate
}
```

**Why:**

- Dashboard loads 8+ table aggregation (200ms → 5ms with cache)
- 30-second TTL acceptable for analytics (not real-time critical)
- Visit writes bypass cache (GDPR requires immediate audit trail)

**Alternatives Considered:**

1. **No caching** - Rejected: 200ms dashboard load is poor UX
2. **Cache everything** - Rejected: Real-time writes need immediate visibility
3. **PostgreSQL materialized views** - Rejected: Adds schema complexity, harder
   to invalidate

### Decision 4: API Versioning via URL Path

**Chosen:** `/api/v1` and `/api/v2` coexist with shared service layer

```go
// api/base.go
func (a *API) registerRoutes() {
    // V1: Legacy API (current endpoints, no changes)
    a.Router.Route("/api", func(r chi.Router) {
        r.Mount("/active", a.Active.Router())      // Existing structure
    })

    // V2: Optimized API (consolidated endpoints)
    a.Router.Route("/api/v2", func(r chi.Router) {
        r.Mount("/active", a.ActiveV2.Router())    // New structure
    })

    // IoT: Frozen forever
    a.Router.Route("/iot", func(r chi.Router) {
        r.Mount("/", a.IoT.Router())               // Never changes
    })

    // Mobile BFF: Separate tier
    a.Router.Route("/mobile/v1", func(r chi.Router) {
        r.Mount("/", a.MobileBFF.Router())         // Mobile-optimized
    })
}

// Both API versions use same services (no duplication)
```

**Why:**

- Clear versioning in URL (no header inspection needed)
- V1 and V2 coexist during migration (6-month overlap)
- Service layer shared (zero duplication of business logic)
- Frontend can migrate gradually (no big bang deployment)

**Alternatives Considered:**

1. **Accept header versioning** - Rejected: Harder to test, worse DX
2. **Breaking changes in-place** - Rejected: Too risky for IoT devices
3. **Query param versioning** - Rejected: Conflicts with filtering

### Decision 5: Mobile BFF Implementation Pattern

**Chosen:** Aggregation layer with parallel service calls

```go
// api/bff/mobile/dashboard.go
func (bff *MobileBFF) GetDashboard(w http.ResponseWriter, r *http.Request) {
    userID := jwt.GetUserID(r.Context())

    // Parallel service calls (reduce round trips: 3 → 1)
    var (
        analytics *Analytics
        myGroups  []*ActiveGroup
        students  []*Student
        wg        sync.WaitGroup
        errs      []error
    )

    wg.Add(3)
    go func() {
        defer wg.Done()
        analytics, errs[0] = bff.analyticsService.GetDashboard(r.Context())
    }()
    go func() {
        defer wg.Done()
        myGroups, errs[1] = bff.activeService.GetMyActiveGroups(r.Context(), userID)
    }()
    go func() {
        defer wg.Done()
        students, errs[2] = bff.studentService.GetMyStudents(r.Context(), userID)
    }()

    wg.Wait()

    // Single response with all data mobile needs
    render.JSON(w, r, MobileDashboard{
        Analytics: analytics,
        MyGroups:  myGroups,
        Students:  students,
    })
}
```

**Why:**

- Mobile apps prefer fewer requests (battery life, latency)
- Parallel execution keeps response time low (<200ms for 3 calls)
- No business logic duplication (reuses existing services)
- Can evolve independently of core API

**Alternatives Considered:**

1. **GraphQL** - Rejected: Adds framework complexity, type generation overhead
2. **Multiple sequential calls from mobile** - Rejected: Slow, wastes battery
3. **Server-side composition in core API** - Rejected: Couples core API to
   mobile needs

## Technical Implementation Details

### Phase 1: Endpoint Usage Tracking

```go
// middleware/usage_tracking.go
type EndpointUsage struct {
    Path       string
    Method     string
    Count      int64
    LastCalled time.Time
}

func UsageTrackingMiddleware(db *bun.DB) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Track endpoint usage
            go func() {
                ctx := context.Background()
                _, _ = db.NewInsert().
                    Model(&EndpointUsage{
                        Path:       r.URL.Path,
                        Method:     r.Method,
                        Count:      1,
                        LastCalled: time.Now(),
                    }).
                    On("CONFLICT (path, method) DO UPDATE").
                    Set("count = endpoint_usage.count + 1").
                    Set("last_called = EXCLUDED.last_called").
                    Exec(ctx)
            }()

            next.ServeHTTP(w, r)
        })
    }
}
```

### Phase 2: Redis Caching Integration

```go
// services/active/active_service.go
func (s *ActiveService) GetDashboardAnalytics(ctx context.Context) (*DashboardAnalytics, error) {
    cacheKey := "dashboard:analytics"

    // Check cache first
    if s.cache != nil {
        cached, err := s.cache.Get(ctx, cacheKey).Result()
        if err == nil {
            var analytics DashboardAnalytics
            if err := json.Unmarshal([]byte(cached), &analytics); err == nil {
                return &analytics, nil
            }
        }
    }

    // Expensive aggregation query (8+ table joins)
    analytics, err := s.computeDashboardAnalytics(ctx)
    if err != nil {
        return nil, err
    }

    // Cache for 30 seconds
    if s.cache != nil {
        data, _ := json.Marshal(analytics)
        s.cache.Set(ctx, cacheKey, data, 30*time.Second)
    }

    return analytics, nil
}
```

### Phase 2: N+1 Query Fixes

```go
// database/repositories/active/visit_repository.go

// ❌ BEFORE: N+1 queries (1 + 50 + 50 = 101 queries for 50 visits)
func (r *VisitRepository) ListByGroup(ctx context.Context, groupID int64) ([]*Visit, error) {
    query := r.db.NewSelect().
        Model((*active.Visit)(nil)).
        Where("active_group_id = ?", groupID)

    var visits []*active.Visit
    if err := query.Scan(ctx, &visits); err != nil {
        return nil, err
    }

    // N queries: Load students
    for i := range visits {
        student, _ := r.studentRepo.Get(ctx, visits[i].StudentID)  // N queries!
        visits[i].Student = student
    }

    return visits, nil
}

// ✅ AFTER: Single query with eager loading
func (r *VisitRepository) ListByGroup(ctx context.Context, groupID int64) ([]*Visit, error) {
    query := r.db.NewSelect().
        Model((*active.Visit)(nil)).
        ModelTableExpr(`active.visits AS "visit"`).
        Relation("Student.Person", func(q *bun.SelectQuery) *bun.SelectQuery {
            // Only load needed columns (sparse fieldset at DB level)
            return q.Column("id", "first_name", "last_name")
        }).
        Relation("ActiveGroup.Room", func(q *bun.SelectQuery) *bun.SelectQuery {
            return q.Column("id", "name", "capacity")
        }).
        Where(`"visit".active_group_id = ?`, groupID)

    var visits []*active.Visit
    if err := query.Scan(ctx, &visits); err != nil {
        return nil, err
    }

    return visits, nil  // 1 query instead of 101!
}
```

### Phase 3: Mobile BFF Service Factory

```go
// api/bff/mobile/factory.go
type MobileBFF struct {
    activeService    active.Service
    studentService   users.StudentService
    analyticsService analytics.Service
}

func NewMobileBFF(serviceFactory *services.Factory) *MobileBFF {
    return &MobileBFF{
        activeService:    serviceFactory.NewActiveService(),
        studentService:   serviceFactory.NewStudentService(),
        analyticsService: serviceFactory.NewAnalyticsService(),
    }
}

// Router with mobile-specific endpoints
func (bff *MobileBFF) Router() chi.Router {
    r := chi.NewRouter()

    tokenAuth, _ := jwt.NewTokenAuth()
    r.Use(tokenAuth.Verifier())
    r.Use(jwt.Authenticator)

    // Aggregated endpoints
    r.Get("/dashboard", bff.GetDashboard)
    r.Post("/batch-query", bff.ExecuteBatchQuery)

    // Optimized list endpoints with sparse fieldsets
    r.Get("/students", bff.ListStudentsOptimized)
    r.Get("/groups", bff.ListGroupsOptimized)

    return r
}
```

### Phase 4: Query Param Validation Middleware

```go
// middleware/query_filter_auth.go
type QueryFilterAuthConfig struct {
    AllowedParams map[string]PermissionValidator
}

type PermissionValidator func(ctx context.Context, userID int64, value string) error

func QueryFilterAuth(config QueryFilterAuthConfig) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            userID := jwt.GetUserID(r.Context())

            // Validate each query param for authorization
            for param, values := range r.URL.Query() {
                validator, exists := config.AllowedParams[param]
                if !exists {
                    // Unknown parameter - log warning but allow (for flexibility)
                    logging.Logger.Warnf("Unknown query parameter: %s", param)
                    continue
                }

                if err := validator(r.Context(), userID, values[0]); err != nil {
                    http.Error(w, err.Error(), http.StatusForbidden)
                    return
                }
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

## Risks / Trade-offs

### Risk: Redis Cache Staleness

**Impact:** Dashboard shows outdated data for up to 30 seconds **Mitigation:**

- Acceptable for analytics (not real-time critical)
- Cache invalidation on write operations (e.g., ending a session clears
  dashboard cache)
- Monitoring: Alert if cache hit rate <80%

### Risk: N+1 Query Fixes Break Existing Code

**Impact:** Changes to repository methods could break service layer assumptions
**Mitigation:**

- Comprehensive integration tests before/after
- Bruno API tests run after each repository change
- Gradual rollout: One domain at a time (start with Active, highest impact)

### Risk: V1/V2 Coexistence Maintenance Burden

**Impact:** Two API versions to maintain for 6 months **Mitigation:**

- Shared service layer (zero business logic duplication)
- V1 declared deprecated after V2 stable (3 months)
- Automated tests run against both versions
- Sunset V1 after mobile app launch (6 months max)

### Risk: Mobile BFF Becomes God Service

**Impact:** BFF aggregates too many concerns, becomes unmaintainable
**Mitigation:**

- Limit to 5 aggregated endpoints max
- Each endpoint serves single mobile screen/workflow
- No business logic in BFF (pure aggregation/transformation)
- Code review: Reject PRs that add business logic to BFF

### Trade-off: Performance vs. Real-Time Accuracy

**Chosen:** Cached analytics (30s staleness) for 10x performance
**Alternative:** Always query database for real-time data (200ms response)
**Rationale:** Dashboard analytics are not real-time critical; 30-second delay
acceptable for 40x speedup

### Trade-off: API Versioning Complexity vs. Breaking Changes

**Chosen:** V1/V2 coexistence for 6 months **Alternative:** Break API in-place,
force all clients to upgrade **Rationale:** IoT firmware can't be updated
remotely; breaking changes = bricked devices

## Migration Plan

### Phase 1: Audit & Delete (Weeks 1-2)

1. Add usage tracking middleware to production (1 day)
2. Monitor endpoint usage for 7 days (1 week)
3. Generate report of unused endpoints (1 hour)
4. Create deletion PR with list of removed endpoints (1 day)
5. Run Bruno API tests to verify no regressions (1 hour)
6. Deploy to production (1 hour)

**Rollback Plan:** Re-add deleted endpoints if usage spike detected

### Phase 2: Performance (Weeks 3-4)

1. Add Redis to Docker Compose (1 day)
2. Implement caching for dashboard analytics (2 days)
3. Fix N+1 queries in Active domain (3 days)
4. Add database indexes (1 day)
5. Load testing with production-like data (1 day)
6. Deploy with feature flag (cache can be disabled) (1 hour)

**Rollback Plan:** Disable Redis cache via environment variable

### Phase 3: Mobile BFF (Weeks 5-8)

1. Create BFF package structure (1 day)
2. Implement dashboard aggregation endpoint (2 days)
3. Add batch query endpoint (2 days)
4. Implement optimized list endpoints (3 days)
5. Integration tests for BFF (2 days)
6. OpenAPI spec for mobile team (1 day)
7. Deploy BFF tier (1 hour)

**Rollback Plan:** BFF is additive only - no rollback needed (core API
unchanged)

### Phase 4: Consolidation (Weeks 9-12)

1. Create `/api/v2` router structure (1 day)
2. Migrate 30 simple list endpoints to query param filtering (5 days)
3. Add query param validation middleware (2 days)
4. Integration tests for v2 endpoints (3 days)
5. Update frontend to use v2 endpoints (5 days)
6. Deprecation headers on v1 endpoints (1 day)

**Rollback Plan:** V1 remains available indefinitely; v2 can be removed without
impact

## Validation & Testing

### Unit Tests

- Cache service: Mock Redis, verify cache hit/miss
- Repository eager loading: Verify single query with joins
- BFF aggregation: Mock service calls, verify parallel execution

### Integration Tests

- Bruno API tests: Run after each phase (59 scenarios, ~270ms)
- Database query counting: Assert <5 queries per endpoint
- Cache invalidation: Verify stale data cleared on writes

### Load Testing

- JMeter: Simulate 150 concurrent users
- Endpoint mix: 80% reads, 20% writes (realistic ratio)
- Target: p95 <100ms for all endpoints

### Security Testing

- Query param injection: Attempt SQL injection via filters
- Permission bypass: Try accessing restricted data via query params
- Cache poisoning: Attempt to cache unauthorized data

## Open Questions

1. **Redis High Availability:** Should we use Redis Sentinel for production?
   (Decision: Defer until usage data shows need)

2. **Mobile BFF Authentication:** Same JWT as web, or separate mobile-specific
   tokens? (Decision: Reuse existing JWT infrastructure)

3. **V1 Sunset Timeline:** 6 months after v2 launch, or wait for 90% v2
   adoption? (Decision: Decide based on actual adoption metrics)

4. **GraphQL Future:** Should we add GraphQL as 4th tier for advanced clients?
   (Decision: Defer until mobile team explicitly requests it)

5. **Monitoring:** What metrics should trigger cache disable? (Decision: Cache
   hit rate <80%, p99 latency >500ms)
