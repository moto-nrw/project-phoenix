package api

import (
	"errors"

	careStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/careplan/http"
	peopleStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/students"
	projectionStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/presenceprojection/http"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
	reviewStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/requestreview/http"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/studentpresence/http"
	deleteStudentsHTTP "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/http"
)

// NewStudentHTTPResources binds owner transports at the existing students mount.
func NewStudentHTTPResources(
	peopleCfg peopleStudentsHTTP.ResourceConfig,
	careCfg careStudentsHTTP.ResourceConfig,
	projectionCfg projectionStudentsHTTP.ResourceConfig,
	presenceCfg presenceStudentsHTTP.ResourceConfig,
	deleteCfg deleteStudentsHTTP.ResourceConfig,
	reviewCfg reviewStudentsHTTP.ResourceConfig,
) (*peopleStudentsHTTP.Resource, *careStudentsHTTP.Resource, *projectionStudentsHTTP.Resource, *presenceStudentsHTTP.Resource, *deleteStudentsHTTP.Resource, *reviewStudentsHTTP.Resource) {

	projectionCfg.CompareSchoolClasses = peopleStudentsHTTP.CompareSchoolClasses
	projectionCfg.CompareGermanNames = peopleStudentsHTTP.CompareGermanNames
	projection := projectionStudentsHTTP.NewResource(projectionCfg)
	presence := presenceStudentsHTTP.NewResource(presenceCfg)
	careCfg.RespondCreatedStudent = projection.RespondCreatedStudent
	careCfg.RespondUpdatedStudent = projection.RespondUpdatedStudent
	careCfg.PersistStudentStatusHistory = presence.PersistStudentStatusHistory
	careCfg.NotifyAbsenceReported = presence.NotifyAbsenceReported
	careCfg.IsPartialAbsenceConflict = func(err error) bool { return errors.Is(err, studentpresence.ErrStudentStatusDayPartialAbsenceConflict) }
	careCfg.WriteCareDecision = reviewStudentsHTTP.WriteCareDecision
	careCfg.WriteCareDetail = reviewStudentsHTTP.WriteCareDetail
	careCfg.WriteMasterDataDecision = reviewStudentsHTTP.WriteMasterDataDecision
	careCfg.WriteExcusedDecision = reviewStudentsHTTP.WriteExcusedDecision
	careCfg.OfferingDiffLabel = requestreview.GermanOfferingDiffLabel
	care := careStudentsHTTP.NewResource(careCfg)
	reviewCfg.QueueError = careStudentsHTTP.ParentRequestQueueErrorRenderer
	return peopleStudentsHTTP.NewResource(peopleCfg), care, projection, presence, deleteStudentsHTTP.NewResource(deleteCfg), reviewStudentsHTTP.NewResource(reviewCfg)

}
