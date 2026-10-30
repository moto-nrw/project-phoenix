package timetablehttp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Wochenprotokolle (#4769) — HTTP-Fläche unter /api/week-protocols im
// OGS-Portal. Lesen und Ergänzen dürfen die Fachkräfte der Gruppe (eigene
// und vertretene Gruppen) und die Leitung (admin:* oder groups:manage); das
// Protokoll einer Schule ohne feste Gruppen jede Fachkraft. Eltern und
// moto schule erreichen die Route nicht: sie hängt nur im OGS-Portal.

// WeekProtocolCaller is the consumer-owned port to the caller context of
// Identity & Access: whether the caller is staff, and their own and
// substituted OGS groups.
type WeekProtocolCaller interface {
	HasCurrentStaff(ctx context.Context) (bool, error)
	MyGroupIDs(ctx context.Context) ([]int64, error)
}

// WeekProtocolAccess is one opening of a protocol or its PDF: the protocol
// names the missing children of a whole group for a week.
type WeekProtocolAccess struct {
	ActorAccountID int64
	ActorRole      string
	ProtocolID     int64
	GroupID        *int64
	WeekStart      calendar.Date
	At             time.Time
}

// WeekProtocolAccessLog records the opening in the data access log; the root
// binds the Audit Platform.
type WeekProtocolAccessLog interface {
	RecordWeekProtocolAccess(ctx context.Context, access WeekProtocolAccess) error
}

// WeekProtocolResourceConfig wires the resource.
type WeekProtocolResourceConfig struct {
	Service   timetable.WeekProtocols
	Caller    WeekProtocolCaller
	AccessLog WeekProtocolAccessLog
	Renderer  timetable.WeekProtocolRenderer
	// Names resolves the display names of accounts; the closing person is
	// kept by name, so a later account change does not rewrite the record.
	Names  func(ctx context.Context, accountIDs []int64) (map[int64]string, error)
	Logger *slog.Logger
	// Seed runs the local demo's Worker lifecycle. It and IsLocalSeed are
	// optional and supplied only by the development root.
	Seed        func(context.Context) error
	IsLocalSeed func(*http.Request) bool
}

// WeekProtocolResource is the HTTP resource of the weekly protocols.
type WeekProtocolResource struct{ cfg WeekProtocolResourceConfig }

// NewWeekProtocolResource builds the resource; every dependency is required.
func NewWeekProtocolResource(cfg WeekProtocolResourceConfig) *WeekProtocolResource {
	if cfg.Service == nil || cfg.Caller == nil || cfg.AccessLog == nil || cfg.Renderer == nil || cfg.Names == nil {
		panic("week protocol resource: service, caller, access log, renderer and names are required")
	}
	if (cfg.Seed == nil) != (cfg.IsLocalSeed == nil) {
		panic("week protocol resource: seed and local seed check must be set together")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &WeekProtocolResource{cfg: cfg}
}

// Router liefert den Router unter /api/week-protocols.
func (rs *WeekProtocolResource) Router(protectedTenantRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTenantRoutes(r, func(r chi.Router, withTx common.Middleware) {
		read := common.RequiresAnyPermission(permissions.GroupsRead, permissions.GroupsUpdate, permissions.GroupsManage)
		// Ergänzen und Abschließen ist Arbeit an der Gruppe: Gäste haben nur
		// groups:read und schreiben nichts.
		write := common.RequiresAnyPermission(permissions.GroupsUpdate, permissions.GroupsManage)
		r.With(read, withTx).Get("/", rs.list)
		r.With(read, withTx).Get("/{protocolId}", rs.get)
		r.With(write, withTx).Put("/{protocolId}/notes", rs.saveNotes)
		r.With(write, withTx).Post("/{protocolId}/close", rs.close)
		r.With(read, withTx).Get("/{protocolId}/pdf", rs.pdf)
		if rs.cfg.Seed != nil {
			r.With(common.RequiresPermission(permissions.GroupsManage), withTx).Post("/seed", rs.seed)
		}
	})
	return r
}

func (rs *WeekProtocolResource) seed(w http.ResponseWriter, r *http.Request) {
	if !rs.cfg.IsLocalSeed(r) {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("not found")))
		return
	}
	if err := rs.cfg.Seed(r.Context()); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("seed week protocols", err))
		return
	}
	render.JSON(w, r, map[string]any{"data": map[string]bool{"seeded": true}})
}

var weekProtocolErrorRules = []common.ErrorRule{
	{Target: timetable.ErrWeekProtocolNotFound, Render: func(err error) render.Renderer {
		return common.ErrorNotFoundWithCode(err, common.CodeTimetableWeekProtocolNotFound)
	}},
	{Target: timetable.ErrWeekProtocolNotDraft, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeTimetableWeekProtocolNotDraft)
	}},
	{Target: timetable.ErrWeekProtocolInvalid, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeTimetableWeekProtocolNotesInvalid)
	}},
	{Target: timetable.ErrWeekProtocolNotClosed, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeTimetableWeekProtocolNotClosed)
	}},
}

var renderWeekProtocolError = common.RulesRenderer(weekProtocolErrorRules, common.ErrorInternalServer)

// reach resolves which protocols the caller sees. A failed lookup is an
// error, never a narrower reach. Without groups:update (the view-only guest
// role, ADR 0091) the caller is no group staff and sees no protocol: the
// protocols name missing children and their reason.
func (rs *WeekProtocolResource) reach(ctx context.Context) (timetable.WeekProtocolReach, error) {
	granted := sessiontoken.PermissionsFromCtx(ctx)
	if securityruntime.HasAdminWildcard(granted) || securityruntime.HasPermission(permissions.GroupsManage, granted) {
		return timetable.WeekProtocolReach{Leadership: true, Staff: true}, nil
	}
	if !securityruntime.HasPermission(permissions.GroupsUpdate, granted) {
		return timetable.WeekProtocolReach{}, nil
	}
	staff, err := rs.cfg.Caller.HasCurrentStaff(ctx)
	if err != nil {
		return timetable.WeekProtocolReach{}, fmt.Errorf("resolve staff: %w", err)
	}
	if !staff {
		return timetable.WeekProtocolReach{}, nil
	}
	groupIDs, err := rs.cfg.Caller.MyGroupIDs(ctx)
	if err != nil {
		return timetable.WeekProtocolReach{}, fmt.Errorf("resolve groups: %w", err)
	}
	return timetable.WeekProtocolReach{Staff: true, GroupIDs: groupIDs}, nil
}

func (rs *WeekProtocolResource) list(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	filter, err := weekProtocolFilterOf(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	reach, err := rs.reach(ctx)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	protocols, err := rs.cfg.Service.ListWeekProtocols(ctx, reach, filter)
	if err != nil {
		common.RenderError(w, r, renderWeekProtocolError(err))
		return
	}
	out := make([]weekProtocolResponse, 0, len(protocols))
	for _, protocol := range protocols {
		out = append(out, weekProtocolResponseOf(protocol, false))
	}
	render.JSON(w, r, map[string]any{"data": out})
}

func (rs *WeekProtocolResource) get(w http.ResponseWriter, r *http.Request) {
	protocol, ok := rs.load(w, r)
	if !ok {
		return
	}
	if err := rs.recordAccess(r.Context(), protocol); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to record audit trail", err))
		return
	}
	render.JSON(w, r, map[string]any{"data": weekProtocolResponseOf(protocol, true)})
}

func (rs *WeekProtocolResource) saveNotes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := weekProtocolIDFromURL(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	notes, err := decodeWeekProtocolNotes(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeTimetableWeekProtocolNotesInvalid))
		return
	}
	reach, err := rs.reach(ctx)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	protocol, err := rs.cfg.Service.SaveWeekProtocolNotes(ctx, reach, id, notes)
	if err != nil {
		common.RenderError(w, r, renderWeekProtocolError(err))
		return
	}
	render.JSON(w, r, map[string]any{"data": weekProtocolResponseOf(protocol, true)})
}

func (rs *WeekProtocolResource) close(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := weekProtocolIDFromURL(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	reach, err := rs.reach(ctx)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	actor, err := rs.actor(ctx)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	protocol, err := rs.cfg.Service.CloseWeekProtocol(ctx, reach, id, actor, rs.cfg.Renderer)
	if err != nil {
		common.RenderError(w, r, renderWeekProtocolError(err))
		return
	}
	render.JSON(w, r, map[string]any{"data": weekProtocolResponseOf(protocol, true)})
}

// pdf serves the PDF stored when the protocol was closed (#4769), byte for
// byte the file of the closing.
func (rs *WeekProtocolResource) pdf(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := weekProtocolIDFromURL(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	reach, err := rs.reach(ctx)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	protocol, file, err := rs.cfg.Service.OpenWeekProtocolPDF(ctx, reach, id)
	if err != nil {
		common.RenderError(w, r, renderWeekProtocolError(err))
		return
	}
	if err := rs.recordAccess(ctx, protocol); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to record audit trail", err))
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(file.Data); err != nil {
		rs.cfg.Logger.Warn("week protocol pdf write failed", slog.String("error", err.Error()))
	}
}

func (rs *WeekProtocolResource) load(w http.ResponseWriter, r *http.Request) (timetable.WeekProtocol, bool) {
	ctx := r.Context()
	id, err := weekProtocolIDFromURL(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return timetable.WeekProtocol{}, false
	}
	reach, err := rs.reach(ctx)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return timetable.WeekProtocol{}, false
	}
	protocol, err := rs.cfg.Service.OpenWeekProtocol(ctx, reach, id)
	if err != nil {
		common.RenderError(w, r, renderWeekProtocolError(err))
		return timetable.WeekProtocol{}, false
	}
	return protocol, true
}

func (rs *WeekProtocolResource) actor(ctx context.Context) (timetable.WeekProtocolActor, error) {
	actorID := sessiontoken.ActorAccountIDFromCtx(ctx)
	if actorID == nil || *actorID <= 0 {
		return timetable.WeekProtocolActor{}, errors.New("no acting account")
	}
	accountID := *actorID
	names, err := rs.cfg.Names(ctx, []int64{accountID})
	if err != nil {
		return timetable.WeekProtocolActor{}, fmt.Errorf("resolve the closing person: %w", err)
	}
	name := strings.TrimSpace(names[accountID])
	if name == "" {
		name = timetable.StaffNoticeUnknownAcknowledgerName
	}
	return timetable.WeekProtocolActor{AccountID: accountID, Name: name}, nil
}

// recordAccess writes the disclosure of the protocol's week and group.
func (rs *WeekProtocolResource) recordAccess(ctx context.Context, protocol timetable.WeekProtocol) error {
	claims := sessiontoken.ClaimsFromCtx(ctx)
	role := strings.Join(claims.Roles, ",")
	if role == "" {
		role = "unknown"
	}
	return rs.cfg.AccessLog.RecordWeekProtocolAccess(ctx, WeekProtocolAccess{
		ActorAccountID: int64(claims.ID), ActorRole: role, ProtocolID: protocol.ID,
		GroupID: protocol.GroupID, WeekStart: protocol.WeekStart, At: time.Now(),
	})
}

func weekProtocolIDFromURL(r *http.Request) (int64, error) {
	id, err := common.ParseIDParam(r, "protocolId")
	if err != nil || id <= 0 {
		return 0, errors.New("invalid protocol id")
	}
	return id, nil
}

func weekProtocolFilterOf(r *http.Request) (timetable.WeekProtocolFilter, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("group_id"))
	if raw == "" {
		return timetable.WeekProtocolFilter{}, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return timetable.WeekProtocolFilter{}, errors.New("invalid group_id")
	}
	return timetable.WeekProtocolFilter{GroupID: &id}, nil
}

type weekProtocolNotesRequest struct {
	WeekNote string `json:"week_note"`
	Days     []struct {
		Date string `json:"date"`
		Note string `json:"note"`
	} `json:"days"`
}

func decodeWeekProtocolNotes(r *http.Request) (timetable.WeekProtocolNotes, error) {
	var req weekProtocolNotesRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		return timetable.WeekProtocolNotes{}, errors.New("invalid request body")
	}
	notes := timetable.WeekProtocolNotes{WeekNote: req.WeekNote, Days: map[calendar.Date]string{}}
	for _, day := range req.Days {
		date, err := calendar.ParseDate(day.Date)
		if err != nil {
			return timetable.WeekProtocolNotes{}, errors.New("days[].date must be a date (YYYY-MM-DD)")
		}
		notes.Days[date] = day.Note
	}
	return notes, nil
}
