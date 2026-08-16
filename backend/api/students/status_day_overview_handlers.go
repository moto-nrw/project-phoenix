package students

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
)

// Absence overview (#2288): one forward-looking list of every entered
// absence day (sick / excused / class trip) across the children of the
// caller's permitted groups. v1 is future-only because that is the daily
// need (who is signed off in the coming weeks); the retrospective
// supervision-proof case is a separate feature. Conveniently this also
// sidesteps the undated-group-assignment restriction the day log documents —
// past absences could not be attributed to the group of that day reliably.
const maxStatusDayOverviewRangeDays = 366

type statusDayOverviewResponse struct {
	From    string                   `json:"from"`
	To      string                   `json:"to"`
	Entries []statusDayOverviewEntry `json:"entries"`
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
	Note        *string   `json:"note,omitempty"`
	sortDate    timezone.Date
}

// getStudentStatusDaysOverview handles GET /students/status-days?from=&to=&group_id=N.
func (rs *Resource) getStudentStatusDaysOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := rs.dayLogLogger()

	from, to, err := parseStatusDayOverviewRange(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	groups, err := rs.resolveDayLogGroups(r, from, logger)
	if err != nil {
		renderDayLogGroupError(w, r, err, logger)
		return
	}

	entries := []statusDayOverviewEntry{}
	if rs.StudentStatusDayService != nil {
		entries, err = rs.loadStatusDayOverviewEntries(ctx, groups, from, to)
		if err != nil {
			renderError(w, r, common.ErrorInternalServerWrap("failed to load absence overview", err))
			return
		}
	}
	if err := rs.writeStatusDayOverviewAudit(r, from, to, groups, logger); err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to record audit trail", err))
		return
	}

	common.Respond(w, r, http.StatusOK, statusDayOverviewResponse{From: from.String(), To: to.String(), Entries: entries}, "Student status days retrieved successfully")
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

// loadStatusDayOverviewEntries bulk-loads the permitted groups' rosters, the
// active status days of the range, and the person names, and folds them into
// sorted response entries.
func (rs *Resource) loadStatusDayOverviewEntries(ctx context.Context, groups []*educationModel.Group, from, to timezone.Date) ([]statusDayOverviewEntry, error) {
	groupIDs := make([]int64, 0, len(groups))
	groupsByID := make(map[int64]*educationModel.Group, len(groups))
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
		groupsByID[group.ID] = group
	}

	students, err := rs.PersonService.GetStudentsByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	studentsByID := make(map[int64]*usersModel.Student, len(students))
	studentIDs := make([]int64, 0, len(students))
	personIDs := make([]int64, 0, len(students))
	for _, student := range students {
		studentsByID[student.ID] = student
		studentIDs = append(studentIDs, student.ID)
		personIDs = append(personIDs, student.PersonID)
	}

	rows, err := rs.StudentStatusDayService.GetActiveByStudentIDsAndDateRange(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}

	persons, err := rs.PersonService.GetByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}

	entries := make([]statusDayOverviewEntry, 0, len(rows))
	for _, row := range rows {
		student := studentsByID[row.StudentID]
		if student == nil || student.GroupID == nil {
			continue
		}
		entry := statusDayOverviewEntry{
			ID:          strconv.FormatInt(row.ID, 10),
			StudentID:   strconv.FormatInt(row.StudentID, 10),
			SchoolClass: student.SchoolClass,
			GroupID:     strconv.FormatInt(*student.GroupID, 10),
			Date:        row.Date.String(),
			Status:      row.Status,
			Label:       studentStatusDayLabel(row.Status),
			ReportedAt:  row.ReportedAt,
			Source:      row.Source,
			Note:        row.Note,
			sortDate:    row.Date,
		}
		if person := persons[student.PersonID]; person != nil {
			entry.FirstName = person.FirstName
			entry.LastName = person.LastName
		}
		if group := groupsByID[*student.GroupID]; group != nil {
			entry.GroupName = group.Name
		}
		entries = append(entries, entry)
	}
	sortStatusDayOverviewEntries(entries)
	return entries, nil
}

// parseStatusDayOverviewRange reuses the per-child defaults (today up to two
// months ahead) and pins the v1 contract: no past days, bounded range.
func parseStatusDayOverviewRange(r *http.Request) (timezone.Date, timezone.Date, error) {
	from, to, err := parseStatusDayRange(r)
	if err != nil {
		return timezone.Date{}, timezone.Date{}, err
	}
	if from.Before(timezone.TodayDate()) {
		return timezone.Date{}, timezone.Date{}, errors.New("from must not be in the past")
	}
	if to.After(from.AddDays(maxStatusDayOverviewRangeDays - 1)) {
		return timezone.Date{}, timezone.Date{}, errors.New("date range cannot exceed 366 days")
	}
	return from, to, nil
}

func sortStatusDayOverviewEntries(entries []statusDayOverviewEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].sortDate != entries[j].sortDate {
			return entries[i].sortDate.Before(entries[j].sortDate)
		}
		if entries[i].LastName != entries[j].LastName {
			return entries[i].LastName < entries[j].LastName
		}
		return entries[i].FirstName < entries[j].FirstName
	})
}
