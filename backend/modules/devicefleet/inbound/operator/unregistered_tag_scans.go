// Package operator serves the operator review of unregistered RFID scans of
// Device Fleet (#3232): listing the scans of every school, optionally
// narrowed to one school or one Träger, and marking a scan as handled. The
// root composition builds these routes and the operator router mounts them
// behind its middleware chain. The operator surface lends the handlers its
// error bodies, response envelope and authenticated operator, so the wire
// format and authorization stay unchanged.
package operator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Scans is the Device Fleet capability the review reads and resolves through.
type Scans interface {
	ListUnregisteredTagScans(context.Context, devicefleet.UnregisteredTagScanFilter) ([]devicefleet.UnregisteredTagScan, error)
	ResolveUnregisteredTagScan(context.Context, devicefleet.ResolveUnregisteredTagScan) (devicefleet.UnregisteredTagScan, error)
}

// School is the school a scan's tenant is.
type School struct {
	ID             int64
	OrganizationID int64
	Name           string
}

// Organization is the Träger a school belongs to.
type Organization struct {
	ID   int64
	Name string
}

// Directory resolves the schools and Träger that label and narrow the
// review. Deleted schools stay listed, so their scan history stays
// reviewable.
type Directory interface {
	ListSchoolsByID(context.Context, []int64) ([]School, error)
	ListSchoolsByOrganization(context.Context, int64) ([]School, error)
	ListOrganizationsByID(context.Context, []int64) ([]Organization, error)
}

// Surface is what the operator router lends the review routes.
type Surface struct {
	InvalidRequest func(err error) render.Renderer
	Internal       func(message string) render.Renderer
	// ResolveFallback renders a failed resolution.
	ResolveFallback func(err error) render.Renderer
	RenderError     func(w http.ResponseWriter, r *http.Request, renderer render.Renderer)
	Respond         func(w http.ResponseWriter, r *http.Request, status int, data any, message string)
	// OperatorID returns the authenticated operator, or zero.
	OperatorID func(ctx context.Context) int64
}

// Config holds the review routes' dependencies.
type Config struct {
	Scans     Scans
	Directory Directory
	Surface   Surface
	// WithinAdmin runs the cross-school reads and the resolution. Nil uses
	// the tenant runtime's administrative transaction.
	WithinAdmin func(ctx context.Context, fn func(context.Context) error) error
}

// Resource holds the review handlers.
type Resource struct {
	scans       Scans
	directory   Directory
	surface     Surface
	withinAdmin func(ctx context.Context, fn func(context.Context) error) error
}

// NewResource binds the review handlers.
func NewResource(cfg Config) *Resource {
	withinAdmin := cfg.WithinAdmin
	if withinAdmin == nil {
		withinAdmin = tenant.WithinAdmin
	}
	return &Resource{scans: cfg.Scans, directory: cfg.Directory, surface: cfg.Surface, withinAdmin: withinAdmin}
}

// Router serves GET / and POST /{id}/resolve.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Get("/", rs.ListUnregisteredTagScans)
	r.Post("/{id}/resolve", rs.ResolveUnregisteredTagScan)
	return r
}

// scanResponse keeps the wire shape of the retired audit row.
type scanResponse struct {
	ID                   int64      `json:"id"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	TenantID             int64      `json:"tenant_id"`
	TagUID               string     `json:"tag_uid"`
	DeviceID             *int64     `json:"device_id,omitempty"`
	ScannedAt            time.Time  `json:"scanned_at"`
	ResolvedAt           *time.Time `json:"resolved_at,omitempty"`
	ResolvedByOperatorID *int64     `json:"resolved_by_operator_id,omitempty"`
	ResolutionNote       *string    `json:"resolution_note,omitempty"`
	SchoolID             int64      `json:"school_id,omitempty"`
	SchoolName           string     `json:"school_name,omitempty"`
	OrganizationID       int64      `json:"organization_id,omitempty"`
	OrganizationName     string     `json:"organization_name,omitempty"`
	DeviceIdentifier     *string    `json:"device_identifier,omitempty"`
	DeviceName           *string    `json:"device_name,omitempty"`
}

type scanQuery struct {
	schoolID       *int64
	organizationID *int64
	unresolvedOnly bool
}

type resolveUnregisteredTagScanRequest struct {
	Note string `json:"note,omitempty"`
}

func (req *resolveUnregisteredTagScanRequest) Bind(_ *http.Request) error {
	req.Note = strings.TrimSpace(req.Note)
	return nil
}

// ListUnregisteredTagScans lists the scans of every school, newest first.
func (rs *Resource) ListUnregisteredTagScans(w http.ResponseWriter, r *http.Request) {
	query, err := parseScanQuery(r)
	if err != nil {
		rs.surface.RenderError(w, r, rs.surface.InvalidRequest(err))
		return
	}
	scans, err := rs.listForOperator(r.Context(), query)
	if err != nil {
		rs.surface.RenderError(w, r, rs.surface.Internal("Failed to list unregistered RFID scans"))
		return
	}
	rs.surface.Respond(w, r, http.StatusOK, scans, "Unregistered RFID scans retrieved successfully")
}

// ResolveUnregisteredTagScan marks one open scan as handled by the operator.
func (rs *Resource) ResolveUnregisteredTagScan(w http.ResponseWriter, r *http.Request) {
	scanID, err := parseInt64Param(chi.URLParam(r, "id"), "invalid scan ID")
	if err != nil {
		rs.surface.RenderError(w, r, rs.surface.InvalidRequest(err))
		return
	}
	req := &resolveUnregisteredTagScanRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.surface.RenderError(w, r, rs.surface.InvalidRequest(err))
		return
	}
	var note *string
	if req.Note != "" {
		note = &req.Note
	}
	scan, err := rs.resolve(r.Context(), scanID, rs.surface.OperatorID(r.Context()), note)
	if err != nil {
		rs.surface.RenderError(w, r, rs.surface.ResolveFallback(err))
		return
	}
	rs.surface.Respond(w, r, http.StatusOK, scan, "Unregistered RFID scan resolved successfully")
}

func (rs *Resource) listForOperator(ctx context.Context, query scanQuery) ([]scanResponse, error) {
	var result []scanResponse
	err := rs.withinAdmin(ctx, func(adminCtx context.Context) error {
		filter := devicefleet.UnregisteredTagScanFilter{UnresolvedOnly: query.unresolvedOnly}
		if query.organizationID != nil {
			schools, err := rs.directory.ListSchoolsByOrganization(adminCtx, *query.organizationID)
			if err != nil {
				return fmt.Errorf("load organization schools for unregistered tag scans: %w", err)
			}
			if len(schools) == 0 {
				result = []scanResponse{}
				return nil
			}
			filter.TenantIDs = schoolIDs(schools)
		}
		if query.schoolID != nil {
			filter.TenantIDs = narrowToSchool(filter.TenantIDs, *query.schoolID)
		}
		scans, err := rs.scans.ListUnregisteredTagScans(adminCtx, filter)
		if err != nil {
			return err
		}
		result, err = rs.label(adminCtx, scans)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (rs *Resource) resolve(ctx context.Context, id, operatorID int64, note *string) (scanResponse, error) {
	if id <= 0 {
		return scanResponse{}, errors.New("scan ID is required")
	}
	if operatorID <= 0 {
		return scanResponse{}, errors.New("operator ID is required")
	}
	var result scanResponse
	err := rs.withinAdmin(ctx, func(adminCtx context.Context) error {
		scan, err := rs.scans.ResolveUnregisteredTagScan(adminCtx, devicefleet.ResolveUnregisteredTagScan{
			ID: id, OperatorID: operatorID, Note: note,
		})
		if err != nil {
			return err
		}
		labelled, err := rs.label(adminCtx, []devicefleet.UnregisteredTagScan{scan})
		if err != nil {
			return err
		}
		result = labelled[0]
		return nil
	})
	if err != nil {
		return scanResponse{}, err
	}
	return result, nil
}

// label attaches each scan's school and Träger. A scan whose school or
// Träger the directory does not know is an inconsistency, not an omission.
func (rs *Resource) label(ctx context.Context, scans []devicefleet.UnregisteredTagScan) ([]scanResponse, error) {
	tenantIDs := make([]int64, 0, len(scans))
	seenTenants := make(map[int64]struct{}, len(scans))
	for _, scan := range scans {
		if _, seen := seenTenants[scan.TenantID]; !seen {
			seenTenants[scan.TenantID] = struct{}{}
			tenantIDs = append(tenantIDs, scan.TenantID)
		}
	}
	schools, err := rs.directory.ListSchoolsByID(ctx, tenantIDs)
	if err != nil {
		return nil, fmt.Errorf("load schools for unregistered tag scans: %w", err)
	}
	schoolsByID := make(map[int64]School, len(schools))
	for _, school := range schools {
		schoolsByID[school.ID] = school
	}

	responses := make([]scanResponse, 0, len(scans))
	organizationIDs := make([]int64, 0, len(scans))
	seenOrganizations := make(map[int64]struct{}, len(scans))
	for _, scan := range scans {
		school, found := schoolsByID[scan.TenantID]
		if !found {
			return nil, fmt.Errorf("school %d missing from unregistered tag scan query", scan.TenantID)
		}
		responses = append(responses, newScanResponse(scan, school))
		if _, seen := seenOrganizations[school.OrganizationID]; !seen {
			seenOrganizations[school.OrganizationID] = struct{}{}
			organizationIDs = append(organizationIDs, school.OrganizationID)
		}
	}

	organizations, err := rs.directory.ListOrganizationsByID(ctx, organizationIDs)
	if err != nil {
		return nil, fmt.Errorf("load organizations for unregistered tag scans: %w", err)
	}
	names := make(map[int64]string, len(organizations))
	for _, organization := range organizations {
		names[organization.ID] = organization.Name
	}
	for i := range responses {
		name, found := names[responses[i].OrganizationID]
		if !found {
			return nil, fmt.Errorf("organization %d missing from unregistered tag scan query", responses[i].OrganizationID)
		}
		responses[i].OrganizationName = name
	}
	return responses, nil
}

func newScanResponse(scan devicefleet.UnregisteredTagScan, school School) scanResponse {
	return scanResponse{
		ID:                   scan.ID,
		CreatedAt:            scan.CreatedAt,
		UpdatedAt:            scan.UpdatedAt,
		TenantID:             scan.TenantID,
		TagUID:               scan.TagUID,
		DeviceID:             scan.DeviceID,
		ScannedAt:            scan.ScannedAt,
		ResolvedAt:           scan.ResolvedAt,
		ResolvedByOperatorID: scan.ResolvedByOperatorID,
		ResolutionNote:       scan.ResolutionNote,
		SchoolID:             scan.TenantID,
		SchoolName:           school.Name,
		OrganizationID:       school.OrganizationID,
		DeviceIdentifier:     scan.DeviceIdentifier,
		DeviceName:           scan.DeviceName,
	}
}

// narrowToSchool applies the school filter on top of a Träger's schools: a
// school outside them matches nothing.
func narrowToSchool(tenantIDs []int64, schoolID int64) []int64 {
	if tenantIDs == nil {
		return []int64{schoolID}
	}
	narrowed := make([]int64, 0, 1)
	for _, id := range tenantIDs {
		if id == schoolID {
			narrowed = append(narrowed, id)
		}
	}
	return narrowed
}

func schoolIDs(schools []School) []int64 {
	ids := make([]int64, 0, len(schools))
	for _, school := range schools {
		ids = append(ids, school.ID)
	}
	return ids
}

func parseScanQuery(r *http.Request) (scanQuery, error) {
	values := r.URL.Query()
	query := scanQuery{unresolvedOnly: values.Get("resolved") != "all"}
	if schoolID := strings.TrimSpace(values.Get("school_id")); schoolID != "" {
		id, err := parseInt64Param(schoolID, "invalid school ID")
		if err != nil {
			return query, err
		}
		query.schoolID = &id
	}
	if orgID := strings.TrimSpace(values.Get("organization_id")); orgID != "" {
		id, err := parseInt64Param(orgID, "invalid organization ID")
		if err != nil {
			return query, err
		}
		query.organizationID = &id
	}
	return query, nil
}

func parseInt64Param(value, message string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New(message)
	}
	return id, nil
}
