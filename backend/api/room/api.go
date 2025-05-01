package room

import (
	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/uptrace/bun"
)

// API provides room management handlers.
type API struct {
	store RoomStore
}

// NewAPI configures and returns Room API.
func NewAPI(db *bun.DB) (*API, error) {
	store := NewRoomStore(db)
	api := &API{
		store: store,
	}
	return api, nil
}

// Router provides room routes.
func (a *API) Router() *chi.Mux {
	r := chi.NewRouter()

	// Use JWT authentication middleware for all room endpoints
	r.Group(func(r chi.Router) {
		// Add jwt authentication here similar to student API
		r.Use(jwt.Authenticator)

		// Room endpoints
		r.Get("/", a.handleGetRooms)
		r.Post("/", a.handleCreateRoom)
		r.Get("/grouped_by_category", a.handleGetRoomsGroupedByCategory)
		r.Get("/choose", a.handleGetRoomsForSelection)
		r.Get("/{id}", a.handleGetRoomByID)
		r.Put("/{id}", a.handleUpdateRoom)
		r.Delete("/{id}", a.handleDeleteRoom)
		r.Get("/{id}/current_occupancy", a.handleGetCurrentRoomOccupancy)
		r.Post("/{id}/register_tablet", a.handleRegisterTablet)
		r.Post("/{id}/unregister_tablet", a.handleUnregisterTablet)
		r.Get("/{id}/combined_group", a.handleGetCombinedGroupForRoom)

		// Combined groups endpoints
		r.Route("/combined_groups", func(r chi.Router) {
			r.Get("/", a.handleGetActiveCombinedGroups)
			r.Post("/merge", a.handleMergeRooms)
			r.Delete("/{id}", a.handleDeactivateCombinedGroup)
		})

		// Room occupancy endpoints
		r.Route("/occupancies", func(r chi.Router) {
			r.Get("/", a.handleGetAllRoomOccupancies)
			r.Get("/{id}", a.handleGetRoomOccupancyByID)
		})
	})

	return r
}
