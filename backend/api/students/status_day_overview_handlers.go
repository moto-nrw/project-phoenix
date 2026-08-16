package students

import (
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

// Absence overview (#2288): one forward-looking list of every entered
// absence day (sick / excused / class trip) across the children of the
// caller's permitted groups. v1 is future-only because that is the daily
// need (who is signed off in the coming weeks); the retrospective
// supervision-proof case is a separate feature. Conveniently this also
// sidesteps the undated-group-assignment restriction the day log documents —
// past absences could not be attributed to the group of that day reliably.
const maxStatusDayOverviewRangeDays = 366
const maxStatusDayOverviewPageSize = 100

type statusDayOverviewResponse struct {
	From     string                   `json:"from"`
	To       string                   `json:"to"`
	Groups   []statusDayOverviewGroup `json:"groups"`
	Entries  []statusDayOverviewEntry `json:"entries"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
	HasMore  bool                     `json:"has_more"`
}

type statusDayOverviewGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type statusDayOverviewEntry struct {
	ID          string    `json:"id"`
	StudentID   string    `json:"student_id"`
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	SchoolClass string    `json:"school_class"`
	GroupID     string    `json:"group_id"`
	GroupName   string    `json:"group_name"`
	Date        string    `json:"date"`
	Status      string    `json:"status"`
	Label       string    `json:"label"`
	ReportedAt  time.Time `json:"reported_at"`
	Source      string    `json:"source"`
}

// getStudentStatusDaysOverview handles GET /students/status-days?from=&to=&group_id=N.
func (rs *Resource) getStudentStatusDaysOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := rs.dayLogLogger()
	today := timezone.TodayDate()

	from, to, err := parseStatusDayOverviewRange(r, today)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	groups, err := rs.permittedDayLogGroups(ctx, from, logger)
	if err != nil {
		renderDayLogGroupError(w, r, err, logger)
		return
	}
	queryGroups, err := filterDayLogGroups(r, groups)
	if err != nil {
		renderDayLogGroupError(w, r, err, logger)
		return
	}
	page, pageSize := common.ParsePagination(r)
	pageSize = min(pageSize, maxStatusDayOverviewPageSize)
	filters, err := parseStatusDayOverviewFilters(r, page, pageSize)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	entries := []statusDayOverviewEntry{}
	hasMore := false
	if rs.StudentStatusDayService != nil {
		overview, err := rs.StudentStatusDayService.GetOverview(ctx, queryGroups, from, to, today, filters)
		if err != nil {
			renderError(w, r, common.ErrorInternalServerWrap("failed to load absence overview", err))
			return
		}
		entries = mapStatusDayOverviewEntries(overview)
		hasMore = overview.HasMore
	}
	if err := rs.writeStatusDayOverviewAudit(r, from, to, queryGroups, logger); err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to record audit trail", err))
		return
	}

	responseGroups := make([]statusDayOverviewGroup, 0, len(groups))
	for _, group := range groups {
		responseGroups = append(responseGroups, statusDayOverviewGroup{
			ID:   strconv.FormatInt(group.ID, 10),
			Name: group.Name,
		})
	}
	common.Respond(w, r, http.StatusOK, statusDayOverviewResponse{From: from.String(), To: to.String(), Groups: responseGroups, Entries: entries, Page: page, PageSize: pageSize, HasMore: hasMore}, "Student status days retrieved successfully")
}

func parseStatusDayOverviewFilters(r *http.Request, page, pageSize int) (activeService.StatusDayOverviewFilters, error) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status != "" && status != "all" && !slices.Contains(activeModels.StudentStatusDayStatuses(), status) {
		return activeService.StatusDayOverviewFilters{}, errors.New("invalid status filter")
	}
	if status == "all" {
		status = ""
	}
	return activeService.StatusDayOverviewFilters{Query: r.URL.Query().Get("q"), Status: status, Page: page, PageSize: pageSize}, nil
}

func (rs *Resource) writeStatusDayOverviewAudit(r *http.Request, from, to timezone.Date, groups []*educationModel.Group, logger *slog.Logger) error {
	if rs.StudentHistoryService == nil {
		logger.Error("audit log repo not configured, refusing to serve absence overview")
		return errors.New("audit log repository not configured")
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	actorRole := strings.Join(claims.Roles, ",")
	if actorRole == "" {
		actorRole = "unknown"
	}
	groupIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
	}
	entry := &auditModels.DataAccessLog{
		ActorAccountID: int64(claims.ID),
		ActorRole:      actorRole,
		ResourceType:   auditModels.ResourceTypeStudentStatusDayOverview,
		RangeStart:     from.BerlinMidnight(),
		RangeEnd:       to.EndOfDay(),
		AccessedAt:     time.Now(),
	}
	entry.SetMetadata("group_ids", groupIDs)

	if err := rs.StudentHistoryService.RecordDataAccess(r.Context(), entry); err != nil {
		logger.Error("audit log write failed, refusing to serve absence overview",
			slog.String("resource_type", auditModels.ResourceTypeStudentStatusDayOverview),
			slog.String("error", err.Error()),
		)
		return err
	}
	return nil
}

func mapStatusDayOverviewEntries(overview *activeService.StatusDayOverview) []statusDayOverviewEntry {
	rows := overview.Entries
	entries := make([]statusDayOverviewEntry, 0, len(rows))
	for _, row := range rows {
		statusDay := row.StatusDay
		entry := statusDayOverviewEntry{
			ID:          strconv.FormatInt(statusDay.ID, 10),
			StudentID:   strconv.FormatInt(statusDay.StudentID, 10),
			SchoolClass: row.Student.SchoolClass,
			GroupID:     strconv.FormatInt(*row.Student.GroupID, 10),
			Date:        statusDay.Date.String(),
			Status:      statusDay.Status,
			Label:       studentStatusDayLabel(statusDay.Status),
			ReportedAt:  statusDay.ReportedAt,
			Source:      statusDay.Source,
		}
		if row.Person != nil {
			entry.FirstName = row.Person.FirstName
			entry.LastName = row.Person.LastName
		}
		if row.Group != nil {
			entry.GroupName = row.Group.Name
		}
		entries = append(entries, entry)
	}
	return entries
}

// parseStatusDayOverviewRange reuses the per-child defaults (today up to two
// months ahead) and pins the v1 contract: no past days, bounded range.
func parseStatusDayOverviewRange(r *http.Request, today timezone.Date) (timezone.Date, timezone.Date, error) {
	from, to, err := parseStatusDayRangeAt(r, today)
	if err != nil {
		return timezone.Date{}, timezone.Date{}, err
	}
	if from.Before(today) {
		return timezone.Date{}, timezone.Date{}, errors.New("from must not be in the past")
	}
	if to.After(from.AddDays(maxStatusDayOverviewRangeDays - 1)) {
		return timezone.Date{}, timezone.Date{}, errors.New("date range cannot exceed 366 days")
	}
	return from, to, nil
}
