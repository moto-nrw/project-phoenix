package timetracking

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// ErrAuditLogInvalid marks a caller mistake in an audit log request — HTTP 400.
var ErrAuditLogInvalid = errors.New("invalid audit log request")

const (
	auditLogMaxLimit             = 200
	auditLogDefaultPageSize      = 50
	auditLogRetentionDefaultDays = 730
)

// AuditLogPerson names a staff member in the feed.
type AuditLogPerson struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// AuditLogActor is the acting person of an event. StaffID is nil for system
// actions (auto checkout) and for absence-trail actors whose account no
// longer maps to a staff row.
type AuditLogActor struct {
	StaffID  *int64 `json:"staff_id,omitempty"`
	Name     string `json:"name"`
	IsSystem bool   `json:"is_system"`
	IsSelf   bool   `json:"is_self"`
}

// AuditLogEvent is one feed entry: common envelope plus source-typed detail.
type AuditLogEvent struct {
	OccurredAt time.Time       `json:"occurred_at"`
	Source     string          `json:"source"`
	EntryID    int64           `json:"entry_id"`
	Staff      *AuditLogPerson `json:"staff,omitempty"`
	Actor      AuditLogActor   `json:"actor"`
	Reason     string          `json:"reason"`
	Detail     json.RawMessage `json:"detail"`
}

// AuditLogPage is one keyset page. NextCursor is empty on the last page.
// RetentionCutoff is the oldest calendar day session edits can still exist
// for (gdpr.time_tracking_retention_days); ledger sources reach further back.
type AuditLogPage struct {
	Events          []*AuditLogEvent `json:"events"`
	NextCursor      string           `json:"next_cursor,omitempty"`
	RetentionCutoff string           `json:"retention_cutoff"`
}

// AuditLogListRequest carries the parsed query filters.
type AuditLogListRequest struct {
	From         *timezone.Date
	To           *timezone.Date
	StaffID      int64
	ActorStaffID int64
	Sources      []string
	Cursor       string
	Limit        int
}

// TimeTrackingAuditLogService serves the cross-staff audit feed (#1417).
// Every event is personal data (affected person, acting person, free-text
// reasons); the route gates the whole feed on time_tracking:manage.
type TimeTrackingAuditLogService interface {
	ListAuditLog(ctx context.Context, req AuditLogListRequest) (*AuditLogPage, error)
}

// StaffDisplayNameQuery resolves tenant-visible staff names in one batch.
type StaffDisplayNameQuery interface {
	StaffDisplayNames(context.Context, []int64) (map[int64]string, error)
}

type timeTrackingAuditLogService struct {
	repo       TimeTrackingAuditReader
	staffNames StaffDisplayNameQuery
	settings   settingsResolver
}

func NewTimeTrackingAuditLogService(repo TimeTrackingAuditReader, staffNames StaffDisplayNameQuery, settings settingsResolver) TimeTrackingAuditLogService {
	return &timeTrackingAuditLogService{repo: repo, staffNames: staffNames, settings: settings}
}

type auditLogCursorPayload struct {
	OccurredAt time.Time `json:"o"`
	Source     string    `json:"s"`
	EntryID    int64     `json:"i"`
}

func (s *timeTrackingAuditLogService) ListAuditLog(ctx context.Context, req AuditLogListRequest) (*AuditLogPage, error) {
	for _, src := range req.Sources {
		if !slices.Contains(s.repo.ValidSources(), src) {
			return nil, fmt.Errorf("%w: unknown source %q", ErrAuditLogInvalid, src)
		}
	}
	if req.From != nil && req.To != nil && req.To.Before(*req.From) {
		return nil, fmt.Errorf("%w: to is before from", ErrAuditLogInvalid)
	}
	limit := req.Limit
	if limit <= 0 {
		limit = auditLogDefaultPageSize
	}
	if limit > auditLogMaxLimit {
		return nil, fmt.Errorf("%w: limit exceeds %d", ErrAuditLogInvalid, auditLogMaxLimit)
	}

	filter := TimeTrackingAuditFilter{
		From:         req.From,
		To:           req.To,
		StaffID:      req.StaffID,
		ActorStaffID: req.ActorStaffID,
		Sources:      req.Sources,
		Limit:        limit + 1, // one extra row decides whether a next page exists
	}
	if req.Cursor != "" {
		cursor, err := decodeAuditLogCursor(req.Cursor)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid cursor", ErrAuditLogInvalid)
		}
		filter.Cursor = cursor
	}

	entries, err := s.repo.ListEntries(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit log entries: %w", err)
	}

	hasMore := len(entries) > limit
	if hasMore {
		entries = entries[:limit]
	}

	events, err := s.decorate(ctx, entries)
	if err != nil {
		return nil, err
	}

	page := &AuditLogPage{Events: events, RetentionCutoff: s.retentionCutoff(ctx).String()}
	if hasMore {
		last := entries[len(entries)-1]
		page.NextCursor = encodeAuditLogCursor(last)
	}
	return page, nil
}

// decorate resolves staff and actor names in one batch query — never N+1.
func (s *timeTrackingAuditLogService) decorate(ctx context.Context, entries []*TimeTrackingAuditEntry) ([]*AuditLogEvent, error) {
	idSet := make(map[int64]struct{}, len(entries))
	for _, e := range entries {
		if e.StaffID != nil {
			idSet[*e.StaffID] = struct{}{}
		}
		if e.ActorStaffID != nil {
			idSet[*e.ActorStaffID] = struct{}{}
		}
	}
	staffNames := map[int64]string{}
	if len(idSet) > 0 {
		var err error
		staffNames, err = s.staffNames.StaffDisplayNames(ctx, slices.Collect(maps.Keys(idSet)))
		if err != nil {
			return nil, fmt.Errorf("failed to resolve names for audit log: %w", err)
		}
	}

	events := make([]*AuditLogEvent, len(entries))
	for i, e := range entries {
		event := &AuditLogEvent{
			OccurredAt: e.OccurredAt,
			Source:     e.Source,
			EntryID:    e.EntryID,
			Reason:     e.Reason,
			Detail:     e.Detail,
		}
		if e.StaffID != nil {
			event.Staff = &AuditLogPerson{ID: *e.StaffID, Name: staffNames[*e.StaffID]}
		}
		actor := AuditLogActor{IsSystem: e.ActorIsSystem}
		if e.ActorIsSystem {
			actor.Name = "System"
		} else if e.ActorStaffID != nil {
			actor.StaffID = e.ActorStaffID
			actor.Name = staffNames[*e.ActorStaffID]
			actor.IsSelf = e.StaffID != nil && *e.StaffID == *e.ActorStaffID
		}
		event.Actor = actor
		events[i] = event
	}
	return events, nil
}

// retentionCutoff mirrors the cleanup service's window: today minus the
// configured retention days. Session edits older than this are gone (their
// sessions were deleted); the ledger sources are unaffected.
func (s *timeTrackingAuditLogService) retentionCutoff(ctx context.Context) timezone.Date {
	days := auditLogRetentionDefaultDays
	if s.settings != nil {
		if v, err := s.settings.TimeTrackingRetentionDays(ctx); err == nil && v > 0 {
			days = v
		}
	}
	return timezone.TodayDate().AddDays(-days)
}

func encodeAuditLogCursor(e *TimeTrackingAuditEntry) string {
	payload, _ := json.Marshal(auditLogCursorPayload{OccurredAt: e.OccurredAt, Source: e.Source, EntryID: e.EntryID})
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeAuditLogCursor(raw string) (*TimeTrackingAuditCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}
	var payload auditLogCursorPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	if payload.OccurredAt.IsZero() || payload.Source == "" || payload.EntryID <= 0 {
		return nil, errors.New("incomplete cursor")
	}
	return &TimeTrackingAuditCursor{OccurredAt: payload.OccurredAt, Source: payload.Source, EntryID: payload.EntryID}, nil
}
