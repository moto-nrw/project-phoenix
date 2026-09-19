package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/offeringrequests"
	"github.com/stretchr/testify/assert"
)

// These owner-contract assertions replace the retired generic conflict helper's
// tests. The projection now receives keys directly from each native owner.
func TestNativeConflictKeyContracts(t *testing.T) {
	t.Parallel()
	sick := excusedrequests.Request{Dates: []excusedrequests.Date{"2026-09-01"}, AbsenceStatus: "sick"}
	excused := excusedrequests.Request{Dates: sick.Dates, AbsenceStatus: "excused"}
	assert.Equal(t, []string{"absence:2026-09-01"}, sick.ConflictKeys())
	assert.Equal(t, sick.ConflictKeys(), excused.ConflictKeys())
	otherDay := excusedrequests.Request{Dates: []excusedrequests.Date{"2026-09-02"}}
	assert.NotEqual(t, sick.ConflictKeys(), otherDay.ConflictKeys())
	dates := excusedrequests.Request{Dates: []excusedrequests.Date{"2026-09-02", "2026-09-01", "2026-09-01", ""}}
	assert.Equal(t, []string{"absence:2026-09-01", "absence:2026-09-02"}, dates.ConflictKeys())

	weekly := careplan.CareScheduleReviewItem{Request: &carerequests.Request{}, Diff: []carerequests.DiffEntry{
		{Weekday: 1, CareKind: "booking"}, {Weekday: 3, CareKind: "booking"},
	}}
	assert.Equal(t, []string{"care:1:booking", "care:3:booking"}, careplan.CareReviewConflictKeys(weekly))
	booking := careplan.CareScheduleReviewItem{Request: weekly.Request, Diff: []carerequests.DiffEntry{{Weekday: 2, CareKind: "booking"}}}
	pickup := careplan.CareScheduleReviewItem{Request: weekly.Request, Diff: []carerequests.DiffEntry{{Weekday: 2, CareKind: "pickup"}}}
	assert.NotEqual(t, careplan.CareReviewConflictKeys(booking), careplan.CareReviewConflictKeys(pickup))

	name := masterdatarequests.Request{Target: "person", FieldKey: "last_name"}
	class := masterdatarequests.Request{Target: "student", FieldKey: "school_class"}
	assert.Equal(t, []string{"md:person:last_name"}, name.ConflictKeys())
	assert.NotEqual(t, name.ConflictKeys(), class.ConflictKeys())
	offering := careplan.OfferingReviewItem{Request: &offeringrequests.Request{EffectiveFrom: "2026-09-01"}, Diff: []careplan.OfferingReviewDiffEntry{{OfferingID: 42}}}
	assert.Equal(t, []string{"offer:42"}, careplan.OfferingReviewConflictKeys(offering))
	later := offering
	later.Request = &offeringrequests.Request{EffectiveFrom: "2026-10-01"}
	assert.Equal(t, careplan.OfferingReviewConflictKeys(offering), careplan.OfferingReviewConflictKeys(later))

	assert.Empty(t, (masterdatarequests.Request{Target: "person"}).ConflictKeys())
	invalid := careplan.CareScheduleReviewItem{Request: weekly.Request, Diff: []carerequests.DiffEntry{{Weekday: 0}, {Weekday: 9}}}
	assert.Empty(t, careplan.CareReviewConflictKeys(invalid))
	assert.Empty(t, (excusedrequests.Request{}).ConflictKeys())
	assert.Empty(t, careplan.OfferingReviewConflictKeys(careplan.OfferingReviewItem{}))
}
