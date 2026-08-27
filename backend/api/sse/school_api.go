package sse

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

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
func (rs *Resource) schoolEventsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, ok := withSSETokenDeadline(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	defer cancel()

	conn, statusCode := rs.setupSSEConnection(w)
	if conn == nil {
		slog.WarnContext(ctx, "school SSE streaming unsupported by client")
		http.Error(w, "Streaming unsupported", statusCode)
		return
	}

	accountID := int64(jwt.ClaimsFromCtx(ctx).ID)
	tenantID := tenant.FromContext(ctx)
	if accountID <= 0 || tenantID <= 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	conn.accountID = accountID
	conn.tenantID = tenantID
	conn.staffID = accountID // connection user id for logging only

	if err := conn.sendConnectedEvent(&sseTopics{}); err != nil {
		slog.ErrorContext(ctx, "school SSE failed to send connected event",
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
	rs.runEventLoop(ctx, conn)
}
