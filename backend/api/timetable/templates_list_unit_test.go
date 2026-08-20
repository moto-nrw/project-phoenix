package timetable

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/activities"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateRequiredStaffCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		row  templateRow
		want int
	}{
		{
			name: "override wins over the derived requirement when an occurrence exists",
			row: templateRow{
				RequiredStaff:           sql.NullInt64{Int64: 7, Valid: true},
				CapacityEnrollmentCount: 12,
				CapacityOccurrenceFound: true,
			},
			want: 7,
		},
		{
			name: "no override derives from the Betreuungsschlüssel",
			row: templateRow{
				CapacityEnrollmentCount: 12,
				CapacityOccurrenceFound: true,
			},
			want: 2,
		},
		{
			name: "override is suppressed when no occurrence exists in the period",
			row: templateRow{
				RequiredStaff:           sql.NullInt64{Int64: 7, Valid: true},
				CapacityOccurrenceFound: false,
			},
			want: 0,
		},
		{
			name: "no override and no occurrence stays zero",
			row: templateRow{
				CapacityOccurrenceFound: false,
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, templateRequiredStaffCount(tt.row, 10))
		})
	}
}

func TestBuildTemplateWeekdayAssignments_PreservesEmptyDays(t *testing.T) {
	t.Parallel()

	rows := []activities.TemplateWeekdayRosterRow{
		{TemplateID: 7, Weekday: activities.WeekdayMonday, Kind: activities.TemplateWeekdayRosterKindEmpty},
		{TemplateID: 7, Weekday: activities.WeekdayMonday, Kind: activities.TemplateWeekdayRosterKindStaff, PersonID: 11, IsPrimary: true},
		{TemplateID: 7, Weekday: activities.WeekdayMonday, Kind: activities.TemplateWeekdayRosterKindStudent, PersonID: 21},
		{TemplateID: 7, Weekday: activities.WeekdayMonday, Kind: activities.TemplateWeekdayRosterKindProtectedStudent, PersonID: 22},
		{TemplateID: 7, Weekday: activities.WeekdayTuesday, Kind: activities.TemplateWeekdayRosterKindEmpty},
	}

	byTemplate := buildTemplateWeekdayAssignments(rows)
	templateID := rows[0].TemplateID
	require.Contains(t, byTemplate, templateID)
	assert.Equal(t, []templateWeekdayAssignmentResponse{
		{
			Weekday:        activities.WeekdayMonday,
			StaffIDs:       []int64{11},
			StudentIDs:     []int64{21},
			PrimaryStaffID: ptrInt64(11),
		},
		{
			Weekday:    activities.WeekdayTuesday,
			StaffIDs:   []int64{},
			StudentIDs: []int64{},
		},
	}, byTemplate[templateID])

	protectedByTemplate := buildTemplateProtectedStudentAssignments(rows)
	assert.Equal(t, []templateProtectedStudentAssignmentResponse{{
		Weekday:    activities.WeekdayMonday,
		StudentIDs: []int64{22},
	}}, protectedByTemplate[templateID])
}

func TestListTemplates_WithoutPeriodSkipsWeekdayRosterRead(t *testing.T) {
	t.Parallel()

	repo := &catalogTemplateGroupRepo{}
	resource := NewResource(Dependencies{
		TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{
			ActivityGroupRepo: repo,
		}),
	})
	request := httptest.NewRequest(http.MethodGet, "/templates", nil)
	request = request.WithContext(tenant.WithTenantID(request.Context(), 42))
	response := httptest.NewRecorder()

	resource.listTemplates(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, 0, repo.weekdayRosterReads)
}

type catalogTemplateGroupRepo struct {
	activities.GroupRepository
	weekdayRosterReads int
}

func (r *catalogTemplateGroupRepo) ListTemplateRowsForPeriod(
	context.Context,
	*int64,
) ([]activities.TemplateListRow, error) {
	return []activities.TemplateListRow{}, nil
}

func (r *catalogTemplateGroupRepo) ListTemplateWeekdayRoster(
	context.Context,
	*int64,
	*int64,
) ([]activities.TemplateWeekdayRosterRow, error) {
	r.weekdayRosterReads++
	return nil, nil
}

func ptrInt64(value int64) *int64 {
	return &value
}
