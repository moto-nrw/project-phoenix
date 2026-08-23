package students

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/services/listexport"
)

// birthdayFixtures covers the cases a birthday list has to get right: two
// children sharing a month, one in a different month, one born on a later
// calendar day but in an earlier year (so year-sorting would misplace them),
// and one with no birthday on record at all.
func birthdayFixtures() []StudentResponse {
	return []StudentResponse{
		{ID: 101, FirstName: "Mila", LastName: "Anders", Birthday: "2018-09-02"},
		{ID: 102, FirstName: "Finn", LastName: "Becker", Birthday: "2019-09-24"},
		{ID: 103, FirstName: "Emma", LastName: "Conrad", Birthday: "2017-12-30"},
		{ID: 104, FirstName: "Ida", LastName: "Dorn", Birthday: "2020-01-05"},
		{ID: 105, FirstName: "Jonas", LastName: "Ernst", Birthday: ""},
	}
}

func exportedIDs(students []StudentResponse) []int64 {
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		ids = append(ids, student.ID)
	}
	return ids
}

func TestParseExportMonths(t *testing.T) {
	t.Parallel()

	t.Run("empty list means every month", func(t *testing.T) {
		months, err := parseExportMonths(nil)
		require.NoError(t, err)
		assert.Nil(t, months)
	})

	t.Run("accepts zero-padded and bare month numbers", func(t *testing.T) {
		months, err := parseExportMonths([]string{"01", "9", " 12 "})
		require.NoError(t, err)
		assert.Equal(t, map[time.Month]bool{time.January: true, time.September: true, time.December: true}, months)
	})

	// A silently dropped month would render a list that looks complete while
	// covering the wrong period — the failure mode this validation exists for.
	for _, value := range []string{"0", "13", "abc", "", "-1", "1.5"} {
		t.Run("rejects "+value, func(t *testing.T) {
			_, err := parseExportMonths([]string{value})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid birthday month")
		})
	}
}

func TestDecodeStudentExportRequestRejectsInvalidMonth(t *testing.T) {
	t.Parallel()

	body := `{"preset":"birthday_list","filters":{"months":["13"]}}`
	req := httptest.NewRequest("POST", "/students/export", strings.NewReader(body))

	_, err := decodeStudentExportRequest(req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid birthday month")
}

func TestDecodeStudentExportRequestAcceptsValidMonths(t *testing.T) {
	t.Parallel()

	body := `{"preset":"birthday_list","filters":{"months":["09","10"]}}`
	req := httptest.NewRequest("POST", "/students/export", strings.NewReader(body))

	got, err := decodeStudentExportRequest(req)

	require.NoError(t, err)
	assert.Equal(t, listexport.PresetBirthdayList, got.Preset)
	assert.Equal(t, []string{"09", "10"}, got.Filters.Months)
}

// Zeitraum: a birthday recurs annually, so the month filter must match on the
// month alone and never on the birth year.
func TestApplyExportFiltersBirthdayMonths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		months  []string
		wantIDs []int64
	}{
		{
			name:    "single month keeps only that month across birth years",
			months:  []string{"09"},
			wantIDs: []int64{101, 102},
		},
		{
			name:    "multiple months keep every selected month",
			months:  []string{"09", "01"},
			wantIDs: []int64{101, 102, 104},
		},
		{
			name:    "month spanning the year boundary",
			months:  []string{"12", "01"},
			wantIDs: []int64{103, 104},
		},
		{
			name:    "no months selected keeps every child with a birthday",
			months:  nil,
			wantIDs: []int64{101, 102, 103, 104},
		},
		{
			name:    "month without birthdays yields an empty list",
			months:  []string{"07"},
			wantIDs: []int64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyExportFilters(birthdayFixtures(),
				studentExportFilters{Months: tt.months}, listexport.PresetBirthdayList, testExportDate)

			assert.Equal(t, tt.wantIDs, exportedIDs(got))
		})
	}
}

// A child with no stored birthday cannot belong on a birthday list: printing a
// blank date would read as data rather than as a gap in the records.
func TestApplyExportFiltersBirthdayPresetDropsChildrenWithoutBirthday(t *testing.T) {
	t.Parallel()

	got := applyExportFilters(birthdayFixtures(), studentExportFilters{}, listexport.PresetBirthdayList, testExportDate)

	assert.Equal(t, []int64{101, 102, 103, 104}, exportedIDs(got))
}

// The month filter is preset-independent, so a birthday-less child is dropped
// by the filter itself — they can never match a month.
func TestApplyExportFiltersMonthsDropChildrenWithoutBirthdayOnAnyPreset(t *testing.T) {
	t.Parallel()

	got := applyExportFilters(birthdayFixtures(),
		studentExportFilters{Months: []string{"09"}}, listexport.PresetOGSWeekly, testExportDate)

	assert.Equal(t, []int64{101, 102}, exportedIDs(got))
}

// Without a birthday filter or preset, the export keeps everyone — a child
// without a birthday still belongs on a weekly list.
func TestApplyExportFiltersKeepsBirthdaylessChildrenOnOtherPresets(t *testing.T) {
	t.Parallel()

	got := applyExportFilters(birthdayFixtures(), studentExportFilters{}, listexport.PresetOGSWeekly, testExportDate)

	assert.Equal(t, []int64{101, 102, 103, 104, 105}, exportedIDs(got))
}

func TestApplyExportFiltersBirthdayEmptyInput(t *testing.T) {
	t.Parallel()

	got := applyExportFilters([]StudentResponse{},
		studentExportFilters{Months: []string{"09"}}, listexport.PresetBirthdayList, testExportDate)

	assert.Empty(t, got)
}

// Sortierung: the list must read as a calendar. Sorting by the raw date would
// order by birth year and scatter the months.
func TestSortExportResponsesByBirthday(t *testing.T) {
	t.Parallel()

	students := birthdayFixtures()

	sortExportResponses(students, "birthday")

	assert.Equal(t, []int64{104, 101, 102, 103, 105}, exportedIDs(students),
		"expected calendar order Jan 05, Sep 02, Sep 24, Dec 30, then the child without a birthday")
}

func TestSortExportResponsesByBirthdayBreaksTiesByName(t *testing.T) {
	t.Parallel()

	students := []StudentResponse{
		{ID: 201, FirstName: "Zoe", LastName: "Zimmer", Birthday: "2018-05-04"},
		{ID: 202, FirstName: "Anna", LastName: "Ärmel", Birthday: "2019-05-04"},
		{ID: 203, FirstName: "Bea", LastName: "Berg", Birthday: "2017-05-04"},
	}

	sortExportResponses(students, "birthday")

	assert.Equal(t, []int64{202, 203, 201}, exportedIDs(students),
		"children sharing a day fall back to German name collation")
}

func TestSortExportResponsesByBirthdayPlacesUnknownLast(t *testing.T) {
	t.Parallel()

	students := []StudentResponse{
		{ID: 301, FirstName: "Ada", LastName: "Aal", Birthday: ""},
		{ID: 302, FirstName: "Bo", LastName: "Bär", Birthday: "2018-12-31"},
	}

	sortExportResponses(students, "birthday")

	assert.Equal(t, []int64{302, 301}, exportedIDs(students))
}

// The preset defines the ordering, so a request that only asks for the birthday
// list still gets a calendar — no caller has to know to pass sort="birthday".
func TestExportSortModeDerivedFromBirthdayPreset(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "birthday", exportSortMode(studentExportRequest{
		Preset: listexport.PresetBirthdayList,
	}))
}

func TestExportSortModeKeepsExplicitSort(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "pickup", exportSortMode(studentExportRequest{
		Preset:  listexport.PresetBirthdayList,
		Filters: studentExportFilters{Sort: "pickup"},
	}))
}

func TestExportSortModeLeavesOtherPresetsAlone(t *testing.T) {
	t.Parallel()

	assert.Empty(t, exportSortMode(studentExportRequest{
		Preset: listexport.PresetOGSWeekly,
	}))
}

func TestBirthdayExportCell(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "02.09.2018", birthdayExportCell("2018-09-02"))
	assert.Equal(t, "", birthdayExportCell(""), "no birthday renders empty, never a fabricated date")
	assert.Equal(t, "", birthdayExportCell("not-a-date"))
}

func TestAgeExportCell(t *testing.T) {
	t.Parallel()

	onDate := testExportDate // 2026-07-15

	tests := []struct {
		name     string
		birthday string
		want     string
	}{
		{name: "birthday already passed this year", birthday: "2018-07-14", want: "8"},
		{name: "birthday is today", birthday: "2018-07-15", want: "8"},
		{name: "birthday still ahead this year", birthday: "2018-07-16", want: "7"},
		{name: "earlier month", birthday: "2018-01-05", want: "8"},
		{name: "later month", birthday: "2018-12-30", want: "7"},
		{name: "no birthday", birthday: "", want: ""},
		{name: "unparseable birthday", birthday: "2018-13-01", want: ""},
		{name: "birthday in the future", birthday: "2027-01-01", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ageExportCell(tt.birthday, onDate))
		})
	}
}

func TestBuildExportRowRendersBirthdayAndAge(t *testing.T) {
	t.Parallel()

	row := buildExportRow(
		StudentResponse{ID: 101, FirstName: "Mila", LastName: "Anders", Birthday: "2018-09-02"},
		weeklySchedule{}, map[int64]string{}, testExportDate, true)

	assert.Equal(t, "02.09.2018", row.Values[listexport.ColumnBirthday])
	assert.Equal(t, "7", row.Values[listexport.ColumnAge])
}

func TestBuildExportRowLeavesBirthdayCellsEmptyWithoutBirthday(t *testing.T) {
	t.Parallel()

	row := buildExportRow(
		StudentResponse{ID: 105, FirstName: "Jonas", LastName: "Ernst"},
		weeklySchedule{}, map[int64]string{}, testExportDate, true)

	assert.Empty(t, row.Values[listexport.ColumnBirthday])
	assert.Empty(t, row.Values[listexport.ColumnAge])
}

func TestExportTitleBirthdayList(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Geburtstagsliste",
		exportTitle(studentExportRequest{Preset: listexport.PresetBirthdayList}))
	assert.Equal(t, "Wer hat wann Geburtstag",
		exportTitle(studentExportRequest{Preset: listexport.PresetBirthdayList, Title: "Wer hat wann Geburtstag"}))
}

func TestBirthdayMonthFilterLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		months []string
		want   string
	}{
		{name: "no filter", months: nil, want: ""},
		{name: "single month is singular", months: []string{"09"}, want: "Geburtsmonat: September"},
		{
			name:   "several months are listed chronologically regardless of input order",
			months: []string{"12", "01", "09"},
			want:   "Geburtsmonate: Januar, September, Dezember",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, birthdayMonthFilterLabel(tt.months))
		})
	}
}

func TestExportFilterLabelsIncludesBirthdayMonths(t *testing.T) {
	t.Parallel()

	labels := exportFilterLabelsForDate(studentExportFilters{Months: []string{"09"}, Bus: "yes"}, testExportDate, true)

	assert.Contains(t, labels, "Geburtsmonat: September")
	assert.Contains(t, labels, "Buskind", "the extracted yes/no label must keep its wording")
}
