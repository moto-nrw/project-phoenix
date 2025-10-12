package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/logging"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// Router returns a configured router for SSE endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// SSE endpoint requires authentication
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth.JwtAuth))
		r.Use(jwt.Authenticator)

		r.Get("/events", rs.eventsHandler)
	})

	return r
}

// eventsHandler handles Server-Sent Events connections
func (rs *Resource) eventsHandler(w http.ResponseWriter, r *http.Request) {
	// Check if response writer supports flushing (required for SSE)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*") // CORS support for development
	w.Header().Set("X-Accel-Buffering", "no")          // Disable nginx buffering

	// Extract user ID from JWT claims
	claims := jwt.ClaimsFromCtx(r.Context())
	userID := int64(claims.ID)

	// Get supervised active groups for this user
	supervisions, err := rs.activeSvc.GetStaffActiveSupervisions(r.Context(), userID)
	if err != nil {
		if logging.Logger != nil {
			logging.Logger.WithFields(map[string]interface{}{
				"error":   err.Error(),
				"user_id": userID,
			}).Error("Failed to get staff active supervisions for SSE")
		}
		http.Error(w, "Failed to determine supervised groups", http.StatusInternalServerError)
		return
	}

	// Extract active group IDs from supervisions
	activeGroupIDs := make([]string, 0, len(supervisions))
	for _, supervision := range supervisions {
		// Use supervision.GroupID (the active group ID), NOT supervision.ID (supervision record PK)
		activeGroupIDs = append(activeGroupIDs, strconv.FormatInt(supervision.GroupID, 10))
	}

	// If user has no active supervisions, return empty stream
	if len(activeGroupIDs) == 0 {
		if logging.Logger != nil {
			logging.Logger.WithFields(map[string]interface{}{
				"user_id": userID,
			}).Info("SSE connection - no active supervisions")
		}

		// Send initial comment and keep connection open
		fmt.Fprintf(w, ": no active supervisions\n\n")
		flusher.Flush()

		// Keep connection alive with heartbeat only
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				fmt.Fprintf(w, ": heartbeat\n\n")
				flusher.Flush()
			}
		}
	}

	// Create client for this connection
	client := &realtime.Client{
		Channel:          make(chan realtime.Event, 10), // Buffer up to 10 events
		UserID:           userID,
		SubscribedGroups: make(map[string]bool),
	}

	// Register client with hub for supervised groups
	rs.hub.Register(client, activeGroupIDs)
	defer rs.hub.Unregister(client)

	// Create ticker for heartbeat (keep connection alive)
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	// Stream events to client
	for {
		select {
		case <-r.Context().Done():
			// Client disconnected (uses Context.Done(), NOT CloseNotifier - deprecated)
			return

		case event := <-client.Channel:
			// Marshal event data to JSON
			eventData, err := json.Marshal(event)
			if err != nil {
				if logging.Logger != nil {
					logging.Logger.WithFields(map[string]interface{}{
						"error":      err.Error(),
						"user_id":    userID,
						"event_type": string(event.Type),
					}).Error("Failed to marshal SSE event")
				}
				continue
			}

			// Send SSE event with proper format
			fmt.Fprintf(w, "event: %s\n", event.Type)
			fmt.Fprintf(w, "data: %s\n\n", eventData)
			flusher.Flush()

		case <-heartbeat.C:
			// Send heartbeat comment to keep connection alive
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}
