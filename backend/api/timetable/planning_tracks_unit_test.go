package timetable

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	model "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

type planningTrackServiceStub struct {
	created scheduleSvc.PlanningTrackInput
	ordered []int64
}

func (s *planningTrackServiceStub) ListPlanningTracks(context.Context) ([]*model.PlanningTrack, error) {
	return []*model.PlanningTrack{{
		Model: modelBase.Model{ID: 4}, Name: "Früh", Color: "#5080D8", SortOrder: 0,
	}}, nil
}

func (s *planningTrackServiceStub) GetPlanningTrack(context.Context, int64) (*model.PlanningTrack, error) {
	return nil, scheduleSvc.ErrPlanningTrackNotFound
}

func (s *planningTrackServiceStub) CreatePlanningTrack(_ context.Context, input scheduleSvc.PlanningTrackInput) (*model.PlanningTrack, error) {
	s.created = input
	return &model.PlanningTrack{
		Model: modelBase.Model{ID: 5}, Name: input.Name, Color: input.Color, SortOrder: input.SortOrder,
	}, nil
}

func (s *planningTrackServiceStub) UpdatePlanningTrack(context.Context, int64, scheduleSvc.PlanningTrackInput) (*model.PlanningTrack, error) {
	return nil, scheduleSvc.ErrPlanningTrackNotFound
}

func (s *planningTrackServiceStub) ReorderPlanningTracks(_ context.Context, ids []int64) error {
	s.ordered = ids
	return nil
}

func (s *planningTrackServiceStub) ArchivePlanningTrack(context.Context, int64) (*model.PlanningTrack, error) {
	return nil, scheduleSvc.ErrPlanningTrackNotFound
}

func (s *planningTrackServiceStub) RestorePlanningTrack(context.Context, int64) (*model.PlanningTrack, error) {
	return nil, scheduleSvc.ErrPlanningTrackNotFound
}

func (s *planningTrackServiceStub) ValidatePlanningTrackAssignment(context.Context, *int64) error {
	return nil
}

func TestPlanningTrackHandlersCreateAndReorder(t *testing.T) {
	t.Parallel()

	service := new(planningTrackServiceStub)
	resource := NewResource(Dependencies{PlanningTrackService: service})

	createRequest := httptest.NewRequest(http.MethodPost, "/planning-tracks", strings.NewReader(`{"name":"Mittag","color":"#F78C10","sort_order":2}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	resource.createPlanningTrack(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	if service.created.Name != "Mittag" || service.created.Color != "#F78C10" || service.created.SortOrder != 2 {
		t.Fatalf("unexpected create input: %#v", service.created)
	}

	orderRequest := httptest.NewRequest(http.MethodPut, "/planning-tracks/order", strings.NewReader(`{"ids":[9,4]}`))
	orderRequest.Header.Set("Content-Type", "application/json")
	orderRecorder := httptest.NewRecorder()
	resource.reorderPlanningTracks(orderRecorder, orderRequest)
	if orderRecorder.Code != http.StatusOK {
		t.Fatalf("order status = %d, body = %s", orderRecorder.Code, orderRecorder.Body.String())
	}
	if len(service.ordered) != 2 || service.ordered[0] != 9 || service.ordered[1] != 4 {
		t.Fatalf("unexpected order input: %#v", service.ordered)
	}
}

func TestPlanningTrackHandlerMapsNotFound(t *testing.T) {
	t.Parallel()

	resource := NewResource(Dependencies{PlanningTrackService: new(planningTrackServiceStub)})
	request := httptest.NewRequest(http.MethodDelete, "/planning-tracks/99", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", "99")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
	recorder := httptest.NewRecorder()

	resource.archivePlanningTrack(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] == nil {
		t.Fatalf("expected error response, got %#v", body)
	}
}

func TestPlanningTrackHandlersReturnInternalErrorWithoutService(t *testing.T) {
	t.Parallel()

	resource := NewResource(Dependencies{})
	tests := []struct {
		name    string
		handler http.HandlerFunc
		method  string
		path    string
	}{
		{name: "list", handler: resource.listPlanningTracks, method: http.MethodGet, path: "/planning-tracks"},
		{name: "create", handler: resource.createPlanningTrack, method: http.MethodPost, path: "/planning-tracks"},
		{name: "update", handler: resource.updatePlanningTrack, method: http.MethodPut, path: "/planning-tracks/1"},
		{name: "reorder", handler: resource.reorderPlanningTracks, method: http.MethodPut, path: "/planning-tracks/order"},
		{name: "archive", handler: resource.archivePlanningTrack, method: http.MethodDelete, path: "/planning-tracks/1"},
		{name: "restore", handler: resource.restorePlanningTrack, method: http.MethodPost, path: "/planning-tracks/1/restore"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			recorder := httptest.NewRecorder()

			tc.handler(recorder, request)

			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
