package httpadapter

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

// CategoryResource serves /api/room-categories, the school's room category
// catalog (#4352). Reading needs rooms:read, every write rooms:update: the
// catalog colors rooms, so whoever may edit rooms may maintain it.
type CategoryResource struct {
	categories facilities.RoomCategories
	protected  func(chi.Router, func(chi.Router, Middleware))
}

func NewCategoryResource(categories facilities.RoomCategories, protected func(chi.Router, func(chi.Router, Middleware))) *CategoryResource {
	if categories == nil || protected == nil {
		panic("room categories HTTP: all dependencies are required")
	}
	return &CategoryResource{categories: categories, protected: protected}
}

func (rs *CategoryResource) Router() chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	rs.protected(router, func(protected chi.Router, withTx Middleware) {
		read := apiCommon.RequiresPermission(permissions.RoomsRead)
		write := apiCommon.RequiresPermission(permissions.RoomsUpdate)
		protected.With(read, withTx).Get("/", rs.list)
		protected.With(write, withTx).Post("/", rs.create)
		protected.With(write, withTx).Put("/{id}", rs.update)
		protected.With(write, withTx).Delete("/{id}", rs.delete)
	})
	return router
}

type RoomCategoryRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

func (req *RoomCategoryRequest) Bind(_ *http.Request) error { return nil }

type RoomCategoryResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	RoomCount int       `json:"room_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func newRoomCategoryResponse(category facilities.RoomCategory) RoomCategoryResponse {
	return RoomCategoryResponse{
		ID: category.ID, Name: category.Name, Color: category.Color, RoomCount: category.RoomCount,
		CreatedAt: category.CreatedAt, UpdatedAt: category.UpdatedAt,
	}
}

func (rs *CategoryResource) list(w http.ResponseWriter, r *http.Request) {
	categories, err := rs.categories.ListRoomCategories(r.Context())
	if err != nil {
		renderCategoryFailure(w, r, err)
		return
	}
	responses := make([]RoomCategoryResponse, 0, len(categories))
	for _, category := range categories {
		responses = append(responses, newRoomCategoryResponse(category))
	}
	apiCommon.Respond(w, r, http.StatusOK, responses, "Room categories retrieved successfully")
}

func (rs *CategoryResource) create(w http.ResponseWriter, r *http.Request) {
	req := new(RoomCategoryRequest)
	if err := render.Bind(r, req); err != nil {
		apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
		return
	}
	category, err := rs.categories.CreateRoomCategory(r.Context(), facilities.RoomCategoryInput{Name: req.Name, Color: req.Color})
	if err != nil {
		renderCategoryFailure(w, r, err)
		return
	}
	apiCommon.Respond(w, r, http.StatusCreated, newRoomCategoryResponse(category), "Room category created successfully")
}

func (rs *CategoryResource) update(w http.ResponseWriter, r *http.Request) {
	id, err := apiCommon.ParseID(r)
	if err != nil {
		apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(errors.New("invalid room category ID")))
		return
	}
	req := new(RoomCategoryRequest)
	if err := render.Bind(r, req); err != nil {
		apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
		return
	}
	category, err := rs.categories.UpdateRoomCategory(r.Context(), id, facilities.RoomCategoryInput{Name: req.Name, Color: req.Color})
	if err != nil {
		renderCategoryFailure(w, r, err)
		return
	}
	apiCommon.Respond(w, r, http.StatusOK, newRoomCategoryResponse(category), "Room category updated successfully")
}

func (rs *CategoryResource) delete(w http.ResponseWriter, r *http.Request) {
	id, err := apiCommon.ParseID(r)
	if err != nil {
		apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(errors.New("invalid room category ID")))
		return
	}
	if err := rs.categories.DeleteRoomCategory(r.Context(), id); err != nil {
		renderCategoryFailure(w, r, err)
		return
	}
	apiCommon.RespondNoContent(w, r)
}

func renderCategoryFailure(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, facilities.ErrInvalidRoom), errors.Is(err, facilities.ErrRoomColorReserved):
		apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
	case errors.Is(err, facilities.ErrRoomCategoryNotFound):
		apiCommon.RenderError(w, r, apiCommon.ErrorNotFoundWithCode(err, apiCommon.CodeRoomsCategoryNotFound))
	case errors.Is(err, facilities.ErrDuplicateRoomCategory):
		apiCommon.RenderError(w, r, apiCommon.ErrorConflictWithCode(err, apiCommon.CodeRoomsCategoryDuplicate))
	case errors.Is(err, facilities.ErrRoomCategoryInUse):
		apiCommon.RenderError(w, r, apiCommon.ErrorConflictWithCode(err, apiCommon.CodeRoomsCategoryInUse))
	default:
		apiCommon.RenderError(w, r, apiCommon.ErrorInternalServer(err))
	}
}
