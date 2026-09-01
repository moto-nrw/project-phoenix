package sse

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const (
	// schoolAccessRecheckInterval is how long a verified school session is
	// trusted before the next delivery re-checks the role. One minute is short
	// enough that a revoked Lehrkraft stops receiving wake-ups long before her
	// access token expires, and long enough that a chat burst costs one query.
	schoolAccessRecheckInterval = time.Minute

	// schoolAccessGracePeriod bounds how long a stream may keep serving on the
	// last SUCCESSFUL answer while the re-check itself fails. A database blip
	// must not log a Lehrkraft out mid-lesson; an outage must not hold a
	// revoked session open indefinitely either.
	schoolAccessGracePeriod = 5 * time.Minute
)

// SchoolAccessChecker re-checks whether an open school session is still
// authorized. Implemented by services/auth.Service.
type SchoolAccessChecker interface {
	HasSchoolPortalAccess(ctx context.Context, accountID, tenantID int64) (bool, error)
}

// SetSchoolAccess wires the authorization re-check for the school SSE stream.
// Without it the school stream refuses to open — a long-lived connection that
// nobody re-authorizes is exactly what this guard exists to prevent.
func (rs *Resource) SetSchoolAccess(checker SchoolAccessChecker) {
	rs.schoolAccess = checker
}

// SchoolRouter returns the SSE router for the school portal ("moto schule",
// #2208). It mirrors ParentRouter: same streaming loop, but authenticated with
// SchoolMiddleware (scope=school only) and registered per account, so the
// connection is woken only by fan-outs addressed to this Lehrkraft's account —
// today the Team-Chat trigger (staff_message). Mounted at /school-sse.
func (rs *Resource) SchoolRouter() chi.Router {
	r := chi.NewRouter()

	tokenAuth := jwt.MustNewTokenAuth()
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth.JwtAuth))
		r.Use(jwt.Authenticator)
		r.Use(jwt.SchoolMiddleware)
		r.Get("/events", rs.schoolEventsHandler)
	})

	return r
}

// schoolEventsHandler streams account-addressed triggers to a Lehrkraft. No
// subscription resolution (no groups, no supervision topics): the school
// client is indexed by account only (Hub.RegisterSchool), so tenant-wide staff
// refreshes never reach it.
//
// The JWT is not the last word on access here. The stream lives as long as the
// access token, so the school-portal role is verified before the first byte and
// again while the connection is open; a revoked Lehrkraft is disconnected
// instead of being woken until her token runs out.
func (rs *Resource) schoolEventsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, ok := withSSETokenDeadline(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	defer cancel()

	logger := rs.getLogger()

	if rs.schoolAccess == nil {
		logger.ErrorContext(ctx, "school SSE has no access checker wired")
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	conn, statusCode := rs.setupSSEConnection(w)
	if conn == nil {
		logger.WarnContext(ctx, "school SSE streaming unsupported by client")
		http.Error(w, "Streaming unsupported", statusCode)
		return
	}

	accountID := int64(jwt.ClaimsFromCtx(ctx).ID)
	tenantID := tenant.FromContext(ctx)
	if accountID <= 0 || tenantID <= 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	allowed, err := rs.schoolAccess.HasSchoolPortalAccess(ctx, accountID, tenantID)
	if err != nil {
		logger.ErrorContext(ctx, "school SSE failed to verify portal access",
			slog.String("error", err.Error()))
		http.Error(w, "Failed to verify access", http.StatusInternalServerError)
		return
	}
	if !allowed {
		logger.WarnContext(ctx, "school SSE rejected: no school portal role",
			slog.Int64("account_id", accountID),
			slog.Int64("tenant_id", tenantID))
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	conn.accountID = accountID
	conn.tenantID = tenantID
	conn.staffID = accountID // connection user id for logging only

	if err := conn.sendConnectedEvent(&sseTopics{}); err != nil {
		logger.ErrorContext(ctx, "school SSE failed to send connected event",
			slog.String("error", err.Error()))
		http.Error(w, "Failed to initialize SSE stream", http.StatusInternalServerError)
		return
	}

	conn.client = &realtime.Client{
		Channel:          make(chan realtime.Event, 32),
		UserID:           accountID,
		AccountID:        accountID,
		SubscribedGroups: make(map[string]bool),
	}
	rs.hub.RegisterSchool(conn.client, tenantID)
	rs.runSchoolEventLoop(ctx, conn, &schoolAccessGate{
		checker:    rs.schoolAccess,
		accountID:  accountID,
		tenantID:   tenantID,
		verifiedAt: time.Now(),
		logger:     logger,
	})
}

// schoolAccessGate keeps an open school stream tied to a live authorization.
// Not safe for concurrent use — one gate belongs to one connection and is
// consulted only from that connection's event loop.
type schoolAccessGate struct {
	checker    SchoolAccessChecker
	accountID  int64
	tenantID   int64
	verifiedAt time.Time
	logger     *slog.Logger
}

// allow reports whether the stream may keep serving. Within
// schoolAccessRecheckInterval of the last successful verification it answers
// from that result; afterwards it re-checks. A failing check keeps the stream
// alive only inside schoolAccessGracePeriod, so an outage delays the decision
// instead of either logging everyone out or holding revoked sessions open.
func (g *schoolAccessGate) allow(ctx context.Context, now time.Time) bool {
	age := now.Sub(g.verifiedAt)
	if age < schoolAccessRecheckInterval {
		return true
	}

	allowed, err := g.checker.HasSchoolPortalAccess(ctx, g.accountID, g.tenantID)
	if err != nil {
		if age < schoolAccessGracePeriod {
			g.logger.WarnContext(ctx, "school SSE access re-check failed, serving on last result",
				slog.String("error", err.Error()),
				slog.Int64("account_id", g.accountID),
				slog.Int64("tenant_id", g.tenantID),
			)
			return true
		}
		g.logger.ErrorContext(ctx, "school SSE access re-check kept failing, closing stream",
			slog.String("error", err.Error()),
			slog.Int64("account_id", g.accountID),
			slog.Int64("tenant_id", g.tenantID),
		)
		return false
	}
	if !allowed {
		return false
	}
	g.verifiedAt = now
	return true
}

// runSchoolEventLoop is runEventLoop with the authorization gate in front of
// every delivery and every heartbeat: an idle connection is cut within a minute
// of the role being revoked, without waiting for traffic.
func (rs *Resource) runSchoolEventLoop(ctx context.Context, conn *sseConnection, gate *schoolAccessGate) {
	defer rs.hub.Unregister(conn.client)

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case event := <-conn.client.Channel:
			if !schoolDeliver(ctx, gate, func() error { return conn.sendEvent(event) }) {
				return
			}

		case <-heartbeat.C:
			if !schoolDeliver(ctx, gate, conn.sendHeartbeat) {
				return
			}
		}
	}
}

// schoolDeliver gates one write to a school stream. It reports false when the
// loop must stop — cancelled context, revoked access, or a disconnected client.
func schoolDeliver(ctx context.Context, gate *schoolAccessGate, write func() error) bool {
	if ctx.Err() != nil {
		return false
	}
	if !gate.allow(ctx, time.Now()) {
		gate.logger.InfoContext(ctx, "school SSE closed: portal access no longer valid",
			slog.Int64("account_id", gate.accountID),
			slog.Int64("tenant_id", gate.tenantID),
		)
		return false
	}
	return write() == nil
}
