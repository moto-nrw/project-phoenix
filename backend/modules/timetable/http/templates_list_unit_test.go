package timetablehttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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
			row: templateRow{TemplateListRow: timetable.TemplateListRow{
				RequiredStaff:           testpkg.Int64Ptr(7),
				CapacityEnrollmentCount: 12,
				CapacityOccurrenceFound: true,
			}},
			want: 7,
		},
		{
			name: "no override derives from the Betreuungsschlüssel",
			row: templateRow{TemplateListRow: timetable.TemplateListRow{
				CapacityEnrollmentCount: 12,
				CapacityOccurrenceFound: true,
			}},
			want: 2,
		},
		{
			name: "override is suppressed when no occurrence exists in the period",
			row: templateRow{TemplateListRow: timetable.TemplateListRow{
				RequiredStaff:           testpkg.Int64Ptr(7),
				CapacityOccurrenceFound: false,
			}},
			want: 0,
		},
		{
			name: "no override and no occurrence stays zero",
			row: templateRow{TemplateListRow: timetable.TemplateListRow{
				CapacityOccurrenceFound: false,
			}},
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

	rows := []timetable.TemplateWeekdayRosterRow{
		{TemplateID: 7, Weekday: timetable.WeekdayMonday, Kind: timetable.TemplateWeekdayRosterKindEmpty},
		{TemplateID: 7, Weekday: timetable.WeekdayMonday, Kind: timetable.TemplateWeekdayRosterKindStaff, PersonID: 11, IsPrimary: true},
		{TemplateID: 7, Weekday: timetable.WeekdayMonday, Kind: timetable.TemplateWeekdayRosterKindStudent, PersonID: 21},
		{TemplateID: 7, Weekday: timetable.WeekdayMonday, Kind: timetable.TemplateWeekdayRosterKindProtectedStudent, PersonID: 22},
		{TemplateID: 7, Weekday: timetable.WeekdayTuesday, Kind: timetable.TemplateWeekdayRosterKindEmpty},
	}

	byTemplate := buildTemplateWeekdayAssignments(rows)
	templateID := rows[0].TemplateID
	require.Contains(t, byTemplate, templateID)
	assert.Equal(t, []templateWeekdayAssignmentResponse{
		{
			Weekday:        timetable.WeekdayMonday,
			StaffIDs:       []int64{11},
			StudentIDs:     []int64{21},
			PrimaryStaffID: ptrInt64(11),
		},
		{
			Weekday:    timetable.WeekdayTuesday,
			StaffIDs:   []int64{},
			StudentIDs: []int64{},
		},
	}, byTemplate[templateID])

	protectedByTemplate := buildTemplateProtectedStudentAssignments(rows)
	assert.Equal(t, []templateProtectedStudentAssignmentResponse{{
		Weekday:    timetable.WeekdayMonday,
		StudentIDs: []int64{22},
	}}, protectedByTemplate[templateID])
}

func TestListTemplates_WithoutPeriodSkipsWeekdayRosterRead(t *testing.T) {
	t.Parallel()

	repo := &catalogTemplateListing{}
	resource := NewResource(Dependencies{
		TimetableData: repo.data(),
	})
	request := httptest.NewRequest(http.MethodGet, "/templates", nil)
	request = request.WithContext(tenant.WithTenantID(request.Context(), 42))
	response := httptest.NewRecorder()

	resource.listTemplates(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, 0, repo.weekdayRosterReads)
}

// catalogTemplateListing serves the owner's Vorlagen list: an empty period
// list, and a counted weekday roster read.
type catalogTemplateListing struct {
	weekdayRosterReads int
}

func (r *catalogTemplateListing) data() timetable.TimetableDataCapability {
	return &fakeTimetableData{
		ListTemplateEntriesForPeriodFn: func(context.Context, *int64, int) ([]timetable.TemplateListEntry, error) {
			return []timetable.TemplateListEntry{}, nil
		},
		ListTemplateWeekdayRosterFn: func(context.Context, *int64, *int64) ([]timetable.TemplateWeekdayRosterRow, error) {
			r.weekdayRosterReads++
			return nil, nil
		},
	}
}

func ptrInt64(value int64) *int64 {
	return &value
}
