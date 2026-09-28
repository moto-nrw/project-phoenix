package compose

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// The timetable HTTP adapter validates requests with the public owner
// package instead of the retained models (#3424 slice S1). These tests pin
// every mirrored value and rule to the model it was taken from, so the two
// cannot drift while the models remain.

func TestHTTPVocabularyMatchesTheRetainedModels(t *testing.T) {
	t.Parallel()

	assert.Equal(t, activitiesModel.TargetGroupTypeJahrgang, timetable.TargetGroupTypeGrade)
	assert.Equal(t, activitiesModel.TargetGroupTypeKlasse, timetable.TargetGroupTypeSchoolClass)
	assert.Equal(t, activitiesModel.TargetGroupTypeGruppe, timetable.TargetGroupTypeEducationGroup)
	assert.Equal(t, activitiesModel.TargetGroupTypeAngebot, timetable.TargetGroupTypeOffering)
	assert.Equal(t, activitiesModel.TargetGroupTypeNone, timetable.TargetGroupTypeNone)

	assert.Equal(t, activitiesModel.GroupTypeActivity, timetable.GroupTypeActivity)
	assert.Equal(t, activitiesModel.GroupTypeCare, timetable.GroupTypeCare)
	assert.Equal(t, activitiesModel.GroupTypeExternal, timetable.GroupTypeExternal)

	assert.Equal(t, activitiesModel.ListKindEdgeHours, timetable.ListKindEdgeHours)
	assert.Equal(t, activitiesModel.ListKindLearningTime, timetable.ListKindLearningTime)
	assert.Equal(t, activitiesModel.ListKindActivity, timetable.ListKindActivity)
	assert.Equal(t, activitiesModel.ListKindMensa, timetable.ListKindMensa)

	assert.Equal(t, activitiesModel.WeekdayMonday, timetable.WeekdayMonday)
	assert.Equal(t, activitiesModel.WeekdayTuesday, timetable.WeekdayTuesday)
	assert.Equal(t, activitiesModel.WeekdayWednesday, timetable.WeekdayWednesday)
	assert.Equal(t, activitiesModel.WeekdayThursday, timetable.WeekdayThursday)
	assert.Equal(t, activitiesModel.WeekdayFriday, timetable.WeekdayFriday)
	assert.Equal(t, activitiesModel.WeekdaySaturday, timetable.WeekdaySaturday)
	assert.Equal(t, activitiesModel.WeekdaySunday, timetable.WeekdaySunday)

	assert.Equal(t, activitiesModel.TemplateWeekdayRosterKindEmpty, timetable.TemplateWeekdayRosterKindEmpty)
	assert.Equal(t, activitiesModel.TemplateWeekdayRosterKindStaff, timetable.TemplateWeekdayRosterKindStaff)
	assert.Equal(t, activitiesModel.TemplateWeekdayRosterKindStudent, timetable.TemplateWeekdayRosterKindStudent)
	assert.Equal(t, activitiesModel.TemplateWeekdayRosterKindProtectedStudent, timetable.TemplateWeekdayRosterKindProtectedStudent)

	assert.Equal(t, scheduleModel.InstanceStatusPlanned, timetable.InstanceStatusPlanned)
	assert.Equal(t, scheduleModel.InstanceStatusActive, timetable.InstanceStatusActive)
	assert.Equal(t, scheduleModel.InstanceStatusCompleted, timetable.InstanceStatusCompleted)
	assert.Equal(t, scheduleModel.InstanceStatusCancelled, timetable.InstanceStatusCancelled)
	assert.Equal(t, scheduleModel.ActivityInstanceIdempotencyKeyMaxLength, timetable.ActivityInstanceIdempotencyKeyMaxLength)
	assert.Equal(t, scheduleModel.MaxTimetableReadRangeDays, timetable.MaxTimetableReadRangeDays)
	assert.Equal(t, scheduleModel.ActivityExceptionCancelled, timetable.ActivityExceptionCancelled)
	assert.Equal(t, scheduleModel.ActivityExceptionModified, timetable.ActivityExceptionModified)

	assert.Equal(t, scheduleModel.AttendanceStatusExpected, timetable.SlotAttendanceExpected)
	assert.Equal(t, scheduleModel.AttendanceStatusPresent, timetable.SlotAttendancePresent)
	assert.Equal(t, scheduleModel.AttendanceStatusAbsent, timetable.SlotAttendanceAbsent)
	assert.Equal(t, scheduleModel.AttendanceSubstatusLate, timetable.SlotSubstatusLate)
	assert.Equal(t, scheduleModel.AttendanceSubstatusExcused, timetable.SlotSubstatusExcused)
	assert.Equal(t, scheduleModel.AttendanceSubstatusSick, timetable.SlotSubstatusSick)
	assert.Equal(t, scheduleModel.AttendanceSubstatusFieldTrip, timetable.SlotSubstatusFieldTrip)
	assert.Equal(t, scheduleModel.AttendanceSubstatusOther, timetable.SlotSubstatusOther)
	assert.Equal(t, scheduleModel.InstanceStudentNoteMaxLength, timetable.SlotAttendanceNoteMaxLength)
}

// The mirrored school grade range must be the one the retained target
// validation enforces: its edges pass, one step outside fails.
func TestHTTPGradeRangeMatchesTheRetainedTargetValidation(t *testing.T) {
	t.Parallel()

	for _, level := range []int{timetable.MinSchoolGradeLevel, timetable.MaxSchoolGradeLevel} {
		grade := int16(level)
		target := &activitiesModel.GroupTarget{TargetGroupType: activitiesModel.TargetGroupTypeJahrgang, TargetGradeLevel: &grade}
		require.NoError(t, target.Validate(), "grade %d", level)
	}
	for _, level := range []int{timetable.MinSchoolGradeLevel - 1, timetable.MaxSchoolGradeLevel + 1} {
		grade := int16(level)
		target := &activitiesModel.GroupTarget{TargetGroupType: activitiesModel.TargetGroupTypeJahrgang, TargetGradeLevel: &grade}
		require.Error(t, target.Validate(), "grade %d", level)
	}
}

func TestHTTPRequestRulesMatchTheRetainedModels(t *testing.T) {
	t.Parallel()

	t.Run("weekday", func(t *testing.T) {
		for weekday := -1; weekday <= 9; weekday++ {
			assert.Equal(t, activitiesModel.IsValidWeekday(weekday), timetable.IsValidWeekday(weekday), "weekday %d", weekday)
		}
	})

	t.Run("list kind", func(t *testing.T) {
		for _, kind := range []string{"", "edge_hours", "learning_time", "activity", "mensa", "Mensa", " mensa", "lunch"} {
			assert.Equal(t, activitiesModel.IsValidListKind(kind), timetable.IsValidListKind(kind), "kind %q", kind)
		}
	})

	t.Run("target group type", func(t *testing.T) {
		for _, targetType := range []string{"", "jahrgang", "klasse", "gruppe", "angebot", "none", "Klasse", "class"} {
			assert.Equal(t, activitiesModel.IsValidTargetGroupType(targetType), timetable.IsValidTargetGroupType(targetType), "type %q", targetType)
		}
	})

	t.Run("participant limit", func(t *testing.T) {
		for _, limit := range []int{-3, 0, 1, 24} {
			assert.Equal(t, activitiesModel.ParticipantLimitPtr(limit), timetable.ParticipantLimitPtr(limit), "limit %d", limit)
		}
	})

	t.Run("source school classes", func(t *testing.T) {
		for _, classes := range [][]string{nil, {}, {"1a"}, {" 1a ", "1b"}, {"1a", " "}, {"1a", "1A"}, {"1a", " 1a"}, {"Klasse 3", "klasse 4"}} {
			wantClasses, wantErr := activitiesModel.NormalizeSourceSchoolClasses(classes)
			gotClasses, gotErr := timetable.NormalizeSourceSchoolClasses(classes)
			assert.Equal(t, wantClasses, gotClasses, "classes %q", classes)
			assertSameError(t, wantErr, gotErr, fmt.Sprintf("classes %q", classes))
		}
	})
}

func TestHTTPDynamicTargetRuleMatchesTheRetainedModel(t *testing.T) {
	t.Parallel()

	for name, target := range map[string]timetable.GroupTargetInput{
		"grade":                   {TargetGroupType: "jahrgang", TargetGradeLevel: parityGrade(3)},
		"grade missing":           {TargetGroupType: "jahrgang"},
		"grade too low":           {TargetGroupType: "jahrgang", TargetGradeLevel: parityGrade(0)},
		"grade too high":          {TargetGroupType: "jahrgang", TargetGradeLevel: parityGrade(14)},
		"grade with class":        {TargetGroupType: "jahrgang", TargetGradeLevel: parityGrade(3), TargetSchoolClass: parityText("3a")},
		"grade with group":        {TargetGroupType: "jahrgang", TargetGradeLevel: parityGrade(3), EducationGroupID: parityID(4)},
		"class":                   {TargetGroupType: "klasse", TargetSchoolClass: parityText(" 3a ")},
		"class missing":           {TargetGroupType: "klasse"},
		"class blank":             {TargetGroupType: "klasse", TargetSchoolClass: parityText("  ")},
		"class with grade":        {TargetGroupType: "klasse", TargetSchoolClass: parityText("3a"), TargetGradeLevel: parityGrade(3)},
		"class with group":        {TargetGroupType: "klasse", TargetSchoolClass: parityText("3a"), EducationGroupID: parityID(4)},
		"education group":         {TargetGroupType: "gruppe", EducationGroupID: parityID(4)},
		"education group missing": {TargetGroupType: "gruppe"},
		"education group zero":    {TargetGroupType: "gruppe", EducationGroupID: parityID(0)},
		"education group grade":   {TargetGroupType: "gruppe", EducationGroupID: parityID(4), TargetGradeLevel: parityGrade(3)},
		"education group class":   {TargetGroupType: "gruppe", EducationGroupID: parityID(4), TargetSchoolClass: parityText("3a")},
		"offering":                {TargetGroupType: "angebot"},
		"none":                    {TargetGroupType: "none"},
		"empty":                   {},
	} {
		t.Run(name, func(t *testing.T) {
			model := &activitiesModel.GroupTarget{
				TargetGroupType: target.TargetGroupType, TargetGradeLevel: cloneGrade(target.TargetGradeLevel),
				TargetSchoolClass: cloneText(target.TargetSchoolClass), EducationGroupID: cloneID(target.EducationGroupID),
			}
			owner := timetable.GroupTargetInput{
				TargetGroupType: target.TargetGroupType, TargetGradeLevel: cloneGrade(target.TargetGradeLevel),
				TargetSchoolClass: cloneText(target.TargetSchoolClass), EducationGroupID: cloneID(target.EducationGroupID),
			}
			assertSameError(t, model.Validate(), owner.ValidateDynamicTarget(), name)
			assert.Equal(t, model.TargetSchoolClass, owner.TargetSchoolClass)
		})
	}
}

func TestHTTPTemplateTargetGroupRuleMatchesTheRetainedModel(t *testing.T) {
	t.Parallel()

	for name, target := range map[string]timetable.TemplateTargetGroup{
		"empty":                        {},
		"none":                         {TargetGroupType: "none"},
		"unknown type":                 {TargetGroupType: "class"},
		"grade":                        {TargetGroupType: "jahrgang", TargetGradeLevel: parityGrade(13)},
		"grade missing":                {TargetGroupType: "jahrgang"},
		"grade too high":               {TargetGroupType: "jahrgang", TargetGradeLevel: parityGrade(14)},
		"grade with class":             {TargetGroupType: "jahrgang", TargetGradeLevel: parityGrade(2), TargetSchoolClass: parityText("2a")},
		"class":                        {TargetGroupType: "klasse", TargetSchoolClass: parityText(" 2a ")},
		"class blank":                  {TargetGroupType: "klasse", TargetSchoolClass: parityText(" ")},
		"class with grade":             {TargetGroupType: "klasse", TargetSchoolClass: parityText("2a"), TargetGradeLevel: parityGrade(2)},
		"education group":              {TargetGroupType: "gruppe", EducationGroupID: parityID(7)},
		"education group missing":      {TargetGroupType: "gruppe"},
		"education group with class":   {TargetGroupType: "gruppe", EducationGroupID: parityID(7), TargetSchoolClass: parityText("2a")},
		"valueless with grade":         {TargetGroupType: "none", TargetGradeLevel: parityGrade(2)},
		"offering":                     {TargetGroupType: "angebot"},
		"offering source":              {TargetGroupType: "angebot", SourceCareOfferingIDs: []int64{3, 5}},
		"offering source grades":       {TargetGroupType: "angebot", SourceCareOfferingIDs: []int64{3}, SourceGradeLevels: []int{1, 2}},
		"offering source empty grades": {TargetGroupType: "angebot", SourceCareOfferingIDs: []int64{3}, SourceGradeLevels: []int{}},
		"offering source classes":      {TargetGroupType: "angebot", SourceCareOfferingIDs: []int64{3}, SourceSchoolClasses: []string{" 1a ", "1b"}},
		"offering source blank class":  {TargetGroupType: "angebot", SourceCareOfferingIDs: []int64{3}, SourceSchoolClasses: []string{"1a", ""}},
		"offering source dup class":    {TargetGroupType: "angebot", SourceCareOfferingIDs: []int64{3}, SourceSchoolClasses: []string{"1a", "1A"}},
		"offering source both filters": {TargetGroupType: "angebot", SourceCareOfferingIDs: []int64{3}, SourceGradeLevels: []int{1}, SourceSchoolClasses: []string{"1a"}},
		"offering source zero id":      {TargetGroupType: "angebot", SourceCareOfferingIDs: []int64{0}},
		"offering source dup id":       {TargetGroupType: "angebot", SourceCareOfferingIDs: []int64{3, 3}},
		"offering source grade range":  {TargetGroupType: "angebot", SourceCareOfferingIDs: []int64{3}, SourceGradeLevels: []int{14}},
		"offering source dup grade":    {TargetGroupType: "angebot", SourceCareOfferingIDs: []int64{3}, SourceGradeLevels: []int{2, 2}},
		"source on other type":         {TargetGroupType: "klasse", TargetSchoolClass: parityText("1a"), SourceCareOfferingIDs: []int64{3}},
		"source on empty type":         {SourceCareOfferingIDs: []int64{3}},
		"grades without source":        {TargetGroupType: "angebot", SourceGradeLevels: []int{1}},
		"classes without source":       {TargetGroupType: "angebot", SourceSchoolClasses: []string{"1a"}},
		"empty source slices":          {TargetGroupType: "angebot", SourceCareOfferingIDs: []int64{}, SourceGradeLevels: []int{}, SourceSchoolClasses: []string{}},
	} {
		t.Run(name, func(t *testing.T) {
			model := &activitiesModel.Group{
				TargetGroupType: target.TargetGroupType, TargetGradeLevel: cloneGrade(target.TargetGradeLevel),
				TargetSchoolClass: cloneText(target.TargetSchoolClass), EducationGroupID: cloneID(target.EducationGroupID),
				SourceCareOfferingIDs: cloneSlice(target.SourceCareOfferingIDs), SourceGradeLevels: cloneSlice(target.SourceGradeLevels),
				SourceSchoolClasses: cloneSlice(target.SourceSchoolClasses),
			}
			owner := timetable.TemplateTargetGroup{
				TargetGroupType: target.TargetGroupType, TargetGradeLevel: cloneGrade(target.TargetGradeLevel),
				TargetSchoolClass: cloneText(target.TargetSchoolClass), EducationGroupID: cloneID(target.EducationGroupID),
				SourceCareOfferingIDs: cloneSlice(target.SourceCareOfferingIDs), SourceGradeLevels: cloneSlice(target.SourceGradeLevels),
				SourceSchoolClasses: cloneSlice(target.SourceSchoolClasses),
			}
			assertSameError(t, model.ValidateTargetGroup(), owner.Validate(), name)
			// The handlers read the canonicalized fields back, also after a
			// failed check, so the partial canonicalization must agree too.
			assert.Equal(t, model.TargetGroupType, owner.TargetGroupType)
			assert.Equal(t, model.TargetSchoolClass, owner.TargetSchoolClass)
			assert.Equal(t, model.SourceCareOfferingIDs, owner.SourceCareOfferingIDs)
			assert.Equal(t, model.SourceGradeLevels, owner.SourceGradeLevels)
			assert.Equal(t, model.SourceSchoolClasses, owner.SourceSchoolClasses)
		})
	}
}

func assertSameError(t *testing.T, want, got error, label string) {
	t.Helper()
	if want == nil {
		assert.NoError(t, got, label)
		return
	}
	if assert.Error(t, got, label) {
		assert.Equal(t, want.Error(), got.Error(), label)
	}
}

func parityGrade(value int16) *int16 { return &value }

func parityText(value string) *string { return &value }

func parityID(value int64) *int64 { return &value }

func cloneGrade(value *int16) *int16 {
	if value == nil {
		return nil
	}
	return parityGrade(*value)
}

func cloneText(value *string) *string {
	if value == nil {
		return nil
	}
	return parityText(*value)
}

func cloneID(value *int64) *int64 {
	if value == nil {
		return nil
	}
	return parityID(*value)
}

// cloneSlice keeps nil and empty apart, which the canonicalization tells
// apart.
func cloneSlice[T any](values []T) []T {
	if values == nil {
		return nil
	}
	return append(make([]T, 0, len(values)), values...)
}
