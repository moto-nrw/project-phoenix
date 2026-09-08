package application

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type staffFeedOwner struct {
	accountID int64
	tenantID  int64
	email     string
}

func (s *service) StaffCalendarAccess(ctx context.Context) (StaffCalendarAccessInfo, error) {
	url, webcalURL, err := s.StaffCalendarFeedURL(ctx)
	if err != nil {
		return StaffCalendarAccessInfo{}, err
	}
	owner, err := s.currentStaffFeedOwner(ctx)
	if err != nil {
		return StaffCalendarAccessInfo{}, err
	}
	return s.staffCalendarAccessInfo(ctx, owner, url, webcalURL), nil
}

func (s *service) RotateStaffCalendarAccess(ctx context.Context) (StaffCalendarAccessInfo, error) {
	url, webcalURL, err := s.RotateStaffCalendarFeed(ctx)
	if err != nil {
		return StaffCalendarAccessInfo{}, err
	}
	owner, err := s.currentStaffFeedOwner(ctx)
	if err != nil {
		return StaffCalendarAccessInfo{}, err
	}
	return s.staffCalendarAccessInfo(ctx, owner, url, webcalURL), nil
}

// staffCalendarAccessInfo deliberately keeps the iCalendar result available
// when the optional CalDAV setting cannot be read. The failure is handled by
// logging it and failing the CalDAV capability closed; it never invents an
// enabled value or breaks the existing subscription feature.
func (s *service) staffCalendarAccessInfo(ctx context.Context, owner staffFeedOwner, url, webcalURL string) StaffCalendarAccessInfo {
	result := StaffCalendarAccessInfo{URL: url, WebcalURL: webcalURL}
	if s.cfg.CalDAVPolicy == nil {
		s.logger().ErrorContext(ctx, "caldav settings resolver is not configured", "tenant_id", owner.tenantID)
		return result
	}
	enabled, err := s.cfg.CalDAVPolicy.Enabled(ctx)
	if err != nil {
		s.logger().ErrorContext(
			ctx,
			"resolve caldav setting",
			"tenant_id", owner.tenantID,
			"error", err,
		)
		return result
	}
	if !enabled {
		return result
	}
	calDAVURL := strings.TrimRight(strings.TrimSpace(s.cfg.CalDAVURL), "/")
	if calDAVURL == "" {
		s.logger().ErrorContext(ctx, "caldav public URL is not configured", "tenant_id", owner.tenantID)
		return result
	}
	result.CalDAV = &StaffCalDAVCredentials{
		ServerURL:   calDAVURL + "/api/caldav/",
		Username:    owner.email,
		AppPassword: feedTokenFromURL(url),
	}
	return result
}

func feedTokenFromURL(value string) string {
	if value == "" {
		return ""
	}
	index := strings.LastIndexByte(value, '/')
	if index < 0 || index == len(value)-1 {
		return ""
	}
	return value[index+1:]
}

// AuthenticateStaffCalDAV resolves the shared app password, verifies the
// account email and feature gate, and returns one request-local calendar
// snapshot. Every later PROPFIND/REPORT/GET operation reads this same snapshot,
// avoiding multiget N+1 queries and preventing an authorization/data mismatch.
func (s *service) AuthenticateStaffCalDAV(ctx context.Context, username, appPassword string) (*StaffCalDAVCalendar, error) {
	owner, err := s.staffCalendarOwnerByToken(ctx, appPassword)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(strings.TrimSpace(username), owner.email) {
		return nil, ErrNotFound
	}
	if s.cfg.CalDAVPolicy == nil {
		return nil, fmt.Errorf("%w: caldav settings not configured", ErrInvalidRequest)
	}
	enabled, err := s.cfg.CalDAVPolicy.EnabledForTenant(ctx, owner.tenantID)
	if err != nil {
		return nil, fmt.Errorf("resolve caldav setting: %w", err)
	}
	if !enabled {
		return nil, ErrNotFound
	}
	events, err := s.projectStaffCalendarEvents(ctx, owner)
	if err != nil {
		return nil, err
	}
	items := make([]StaffCalDAVItem, 0, len(events))
	for _, event := range events {
		content, err := s.cfg.CalendarRenderer.RenderCalendarObject(ctx, event)
		if err != nil {
			return nil, err
		}
		modifiedAt := event.LastModified
		if modifiedAt.IsZero() {
			modifiedAt = event.Stamp
		}
		items = append(items, StaffCalDAVItem{
			Name:       s.calDAVObjectName(event.UID),
			UID:        event.UID,
			Content:    []byte(content),
			ETag:       s.calDAVContentETag([]byte(content)),
			ModifiedAt: modifiedAt,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return &StaffCalDAVCalendar{
		AccountID: fmt.Sprintf("staff:%d:%d", owner.tenantID, owner.accountID),
		TenantID:  owner.tenantID,
		Revision:  s.calDAVRevision(items),
		Items:     items,
	}, nil
}

func (s *service) staffCalendarOwnerByToken(ctx context.Context, token string) (staffFeedOwner, error) {
	if s.cfg.StaffFeedRepo == nil || s.cfg.AccountRepo == nil {
		return staffFeedOwner{}, fmt.Errorf("%w: staff calendar feed not configured", ErrInvalidRequest)
	}
	if strings.TrimSpace(token) == "" {
		return staffFeedOwner{}, ErrNotFound
	}
	owner, err := s.cfg.StaffFeedRepo.FindOwnerByTokenHash(ctx, s.feedTokenHash(token))
	if err != nil {
		return staffFeedOwner{}, err
	}
	if owner == nil {
		return staffFeedOwner{}, ErrNotFound
	}
	account, err := s.cfg.AccountRepo.FindByID(ctx, owner.AccountID)
	if err != nil {
		return staffFeedOwner{}, err
	}
	if account == nil || !account.IsActive() {
		return staffFeedOwner{}, ErrNotFound
	}
	return staffFeedOwner{accountID: owner.AccountID, tenantID: owner.TenantID, email: account.Email}, nil
}

func (s *service) calDAVObjectName(uid string) string {
	return s.cfg.Digest([]byte(uid)) + ".ics"
}

func (s *service) calDAVContentETag(content []byte) string {
	return `"` + s.cfg.Digest(content) + `"`
}

func (s *service) calDAVRevision(items []StaffCalDAVItem) string {
	var hash strings.Builder
	for _, item := range items {
		_, _ = fmt.Fprintf(&hash, "%s\x00%s\x00", item.Name, item.ETag)
	}
	return s.cfg.Digest([]byte(hash.String()))
}
