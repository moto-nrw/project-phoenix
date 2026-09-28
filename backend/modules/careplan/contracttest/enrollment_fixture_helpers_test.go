package contracttest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Small enrollment fixtures the offering suites share (moved from
// services/enrollment, #3565).

func uniquePhaseName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, testpkg.UniqueSuffix())
}

func uniqueOfferingName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, testpkg.UniqueSuffix())
}

// uniqueSchemaName builds a per-test name so parallel tests don't collide on
// a (tenant_id, name) uniqueness check.
func uniqueSchemaName(prefix string) string {
	return strings.ReplaceAll(prefix, " ", "-") + "-" + time.Now().Format("150405.000000")
}

func makeOffering(phaseID int64, name string) *enrollmentModels.CareOffering {
	return &enrollmentModels.CareOffering{
		PhaseID:        phaseID,
		Name:           name,
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		IsActive:       true,
	}
}

func makeOwnerEligibilityPhase(name string) *enrollmentTest.Phase {
	return &enrollmentTest.Phase{
		Name: name, Kind: enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: enrollmentTest.Date("2026-09-01"), ServiceEndDate: enrollmentTest.Date("2027-07-31"),
		IsActive: true, CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
	}
}

// insertOwnerPhaseForTest persists a phase fixture through the owner and
// returns its generated metadata on the value.
func insertOwnerPhaseForTest(ctx context.Context, owner interface {
	InsertPhase(context.Context, *enrollmentTest.Phase) error
}, phase *enrollmentTest.Phase) error {
	return owner.InsertPhase(ctx, phase)
}

// ownerPhaseForTest copies a phase for an isolated fixture update.
func ownerPhaseForTest(phase *enrollmentTest.Phase) *enrollmentTest.Phase {
	if phase == nil {
		return nil
	}
	value := *phase
	return &value
}

// readOwnerChildForTest loads a request child through the owner query.
func readOwnerChildForTest(ctx context.Context, owner interface {
	ChildByID(context.Context, int64) (*enrollmentTest.RequestChild, error)
}, id int64) (*enrollmentTest.RequestChild, error) {
	return owner.ChildByID(ctx, id)
}

// updateOwnerChildForTest writes a request child's data through the owner.
func updateOwnerChildForTest(ctx context.Context, owner interface {
	UpdateChildData(context.Context, *enrollmentTest.RequestChild) error
}, child *enrollmentTest.RequestChild) error {
	return owner.UpdateChildData(ctx, child)
}

// readOwnerRequestForTest loads a request through the owner query.
func readOwnerRequestForTest(ctx context.Context, owner interface {
	RequestByID(context.Context, int64, bool) (*enrollmentTest.Request, error)
}, id int64) (*enrollmentTest.Request, error) {
	return owner.RequestByID(ctx, id, false)
}

// rawJSON encodes a fixture value for the owner's raw JSON fields.
func rawJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return raw
}

// pickupResyncer records offering pickup projection refreshes.
type pickupResyncer struct {
	OfferingIDs []int64
	Err         error
}

func (r *pickupResyncer) ReconcileOfferingPickupForOffering(_ context.Context, offeringID int64) error {
	r.OfferingIDs = append(r.OfferingIDs, offeringID)
	return r.Err
}

// recordingSourcedTemplateResyncer captures the offering IDs handed to the
// sourced-template resync after a service-window change. A non-nil err is
// returned after recording, simulating a resync that reports the new window
// as incompatible with a sourced template.
type recordingSourcedTemplateResyncer struct {
	offeringIDs         []int64
	detachedOfferingIDs []int64
	err                 error
}

func (r *recordingSourcedTemplateResyncer) ResyncTemplatesSourcedFromOffering(_ context.Context, offeringID int64, _ timezone.Date) error {
	r.offeringIDs = append(r.offeringIDs, offeringID)
	return r.err
}

func (r *recordingSourcedTemplateResyncer) DetachTemplatesSourcedFromOffering(_ context.Context, offeringID int64, _ timezone.Date) error {
	r.detachedOfferingIDs = append(r.detachedOfferingIDs, offeringID)
	return nil
}

// phaseOfferings reads a phase's Care Plan offerings, the way the root binds
// the phase administration.
type phaseOfferings struct{ carePlan careplan.Capability }

func (o phaseOfferings) OfferingIDsForPhase(ctx context.Context, phaseID int64) ([]int64, error) {
	offerings, err := o.carePlan.ListCareOfferings(ctx, careplan.CareOfferingFilter{PhaseIDs: []int64{phaseID}, Order: careplan.OfferingOrderCatalog})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(offerings))
	for _, offering := range offerings {
		ids = append(ids, offering.ID)
	}
	return ids, nil
}

func (o phaseOfferings) CountOfferingsForPhase(ctx context.Context, phaseID int64) (int, error) {
	return o.carePlan.CountCareOfferingsByPhase(ctx, phaseID)
}
