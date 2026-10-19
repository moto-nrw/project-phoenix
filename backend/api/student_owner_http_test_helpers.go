//go:build testhooks

package api

import (
	"context"
	"errors"
	"log/slog"
	"time"

	reviewbindings "github.com/moto-nrw/project-phoenix/roots/requestreviewbinding"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	careStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/careplan/http"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	groupLiveHTTP "github.com/moto-nrw/project-phoenix/modules/grouplive/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peopleStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/students"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	projectionStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/presenceprojection/http"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"

	reviewStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/requestreview/http"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	reviewsettings "github.com/moto-nrw/project-phoenix/modules/settings/review"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	presenceStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/studentpresence/http"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	studentdeletioncompose "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/compose"
	deleteStudentsHTTP "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/http"
)

// NewStudentHTTPTestResources uses the same owner bindings and tenant mounts as production.
// The caller supplies its existing fixture reader and actor, not a second row model.
func NewStudentHTTPTestResources(store transactionruntime.Database, unit tenant.UnitOfWork, svc StudentTestModule, live ActiveTestModule, liveSettings ActiveSettings, liveEducation ActiveEducation, liveCaller *owners.CallerRows, livePeople ActivePeople, livePresence ActivePresence, liveTimetable ActiveTimetable, peopleServices StudentTestPeople, audit auditModels.Command, groupLive grouplive.Query,
	broadcaster realtime.Broadcaster, caller peopleStudentsHTTP.CallerContext, studentRows carePlanCompose.StudentDirectory,
	feedback studentdeletioncompose.Feedback, auditActor peoplePorts.RequestAuditActor,
	authenticateDevice common.Middleware, deviceID peopleStudentsHTTP.DeviceIdentity, authoritative *bool, clock func() time.Time,
) (*peopleStudentsHTTP.Resource, *careStudentsHTTP.Resource, *projectionStudentsHTTP.Resource, *presenceStudentsHTTP.Resource, *deleteStudentsHTTP.Resource, *reviewStudentsHTTP.Resource, func(func() time.Time) chi.Router, error) {

	if clock == nil {
		clock = time.Now
	}
	students, repoFactory, err := NewStudentHTTPTestRows(store, audit)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	repoFactoryTimetable := NewEnrollmentTimetableRows(store)
	repoFactoryCare := NewEnrollmentCarePlanRows(store)
	repoFactoryRooms := MustNewTimetableRoomRows(store)
	schools, err := owners.NewOrganizationTenancy(store)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	messages := NewStudentMessageRows(store)
	parentEventEmitter := NewParentEventEmitter(store, unit, messages.ParentMessageThread, messages.ParentMessage, liveSettings.Settings, broadcaster, slog.Default())
	studentPhotos := peopleServices.StudentPhotos
	schoolGroups := studentSchoolGroups{GroupManagement: liveEducation.Education}
	presence, err := presenceCompose.New(presenceCompose.Dependencies{DB: store.Pool(), Observe: func(presenceCompose.Observation) {}})
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	requestReview, careReviews, err := newStudentHTTPTestReviews(store, svc, liveCaller, liveSettings, peopleServices, studentRows, clock)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	deletionOwners, err := owners.NewStudentDeletionOwners(store)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	recurrence, err := NewTimetableHTTPTestRecurrenceLock(store)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	studentDeletion, err := owners.NewStudentDeletion(studentdeletioncompose.Dependencies{
		DB: store.Pool(), Owners: deletionOwners, Directory: peopleServices.PeopleDirectory, CarePlan: students.CarePlan, Timetable: repoFactoryTimetable.Timetable,
		Feedback: feedback, IsVerifiedStaff: liveCaller.HasCurrentStaff, LockCareBookingWrites: recurrence.LockRecurrenceWrites,
		UnlinkPhoto: studentPhotos.ScheduleUnlinkAfterCommit, Broadcaster: broadcaster, Audit: audit, Now: clock,
	}, owners.NewStudentActivityEnrollments())
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	lifecycle := svc.CareLifecycle
	if authoritative != nil {
		bookings := *authoritative
		lifecycle, err = NewStudentHTTPTestCareLifecycle(store, StudentHTTPTestCareLifecycle{
			AuditActor: auditActor, BookingsAuthoritative: func(context.Context) (bool, error) { return bookings, nil }, Today: func() calendar.Date { return calendar.DateFromTime(clock()) },
		})
		if err != nil {
			return nil, nil, nil, nil, nil, nil, nil, err
		}
	}
	membership, err := NewStudentHTTPTestMembership(store)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	usages, ok := membership.(schoolmembership.ChildQuotaUsages)
	if !ok {
		err = errors.New("school membership does not expose child quota usage")
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	people, care, projection, status, deletion, review := NewStudentHTTPResources(
		peopleStudentsHTTP.ResourceConfig{
			AuthenticatedDevice: deviceID,
			ChildQuota:          childQuotaStudentsReader{usages: usages},
			DeviceAuthenticator: authenticateDevice,
			FamilyProtection:    peopleServices.PeopleDirectory,
			Logger:              slog.Default(),
			PeopleDirectory:     peopleServices.PeopleDirectory,
			Persons:             peopleCompose.NewStudentRoutePersons(livePeople.Users),
			StudentAuditService: peopleServices.PeopleDirectory,
			StudentPhotos:       peopleServices.PeopleDirectory,
			Uploads:             nativeUploads.UploadFiles{},
			UserContextService:  caller,
		},
		careStudentsHTTP.ResourceConfig{
			ArrivalScheduleService:   live.ArrivalSchedule,
			Broadcaster:              broadcaster,
			CareLifecycleService:     lifecycle,
			CareRequestReviews:       careReviews,
			CareRequestService:       svc.CareRequests,
			CompanionService:         NewStudentHTTPTestCompanions(students, repoFactory.Student, peopleServices.PeopleDirectory, peopleServices.StudentAudit),
			ExcusedRequestService:    svc.ExcusedRequests,
			Logger:                   slog.Default(),
			MasterDataReviewService:  svc.MasterDataReview,
			Now:                      clock,
			OfferingChangeService:    svc.OfferingChanges,
			ParentEventEmitter:       parentEventEmitter,
			ParentRequestBulkService: svc.ParentRequests,
			PartialAbsenceService:    svc.PartialAbsence,
			PeopleDirectory:          peopleServices.PeopleDirectory,
			Persons:                  peopleCompose.NewStudentRoutePersons(livePeople.Users),
			PickupAdjustmentService:  svc.PickupAdjustments,
			PickupScheduleService:    live.PickupSchedule,
			SettingsService:          liveSettings.Settings,
			StudentAuditService:      peopleServices.PeopleDirectory,
			StudentConsents:          NewStudentHTTPTestConsents(store),
			StudentPhotos:            peopleServices.PeopleDirectory,
			UserContextService:       caller,
			WeekdayPickupNotes:       students.CarePlan,
		},
		projectionStudentsHTTP.ResourceConfig{
			AbsenceOverview:        presenceservice.NewStatusDayOverviews(repoFactoryCare.StudentStatusDay, StudentTestStatusDayOverviewPeople(livePeople)),
			ActiveService:          livePresence.Active,
			ArrivalScheduleService: live.ArrivalSchedule,
			CareDayService:         live.CareDay,
			CareLifecycleService:   lifecycle,
			CompanionService:       NewStudentHTTPTestCompanions(students, repoFactory.Student, peopleServices.PeopleDirectory, peopleServices.StudentAudit),
			ExcusedRequestService:  svc.ExcusedRequests,
			InstanceService:        liveTimetable.Instance,
			ListExportService:      lists.NewRenderer(),
			Logger:                 slog.Default(),
			Now:                    clock,
			PeopleDirectory:        peopleServices.PeopleDirectory,
			Persons:                peopleCompose.NewStudentRoutePersons(livePeople.Users),
			PickupScheduleService:  live.PickupSchedule,
			SchoolGroups:           schoolGroups,
			SchoolService:          studentSchoolDirectory{schools: schools},
			SettingsService:        liveSettings.Settings,
			StudentConsents:        NewStudentHTTPTestConsents(store),
			StudentHistoryService: presenceservice.NewStudentHistory(presence, func(ctx context.Context, ids []int64) (map[int64]string, error) {
				rooms, err := repoFactoryRooms.Room.FindByIDs(ctx, ids)
				if err != nil {
					return nil, err
				}
				names := make(map[int64]string, len(rooms))
				for _, room := range rooms {
					names[room.ID] = room.Name
				}
				return names, nil
			}, StudentTestDataAccessAudit(audit), svc.HistorySlots(repoFactoryTimetable.InstanceStudent)),
			StudentStatusDayService: presenceservice.NewStatusDays(repoFactoryCare.StudentStatusDay, svc.ManualPartialAbsences(students.CarePlan), store.Pool(), students.CarePlan.LockExceptionDay),
			UserContextService:      caller,
		},
		presenceStudentsHTTP.ResourceConfig{
			ActiveService:           livePresence.Active,
			Broadcaster:             broadcaster,
			Logger:                  slog.Default(),
			Now:                     clock,
			ParentEventEmitter:      parentEventEmitter,
			PeopleDirectory:         peopleServices.PeopleDirectory,
			Persons:                 peopleCompose.NewStudentRoutePersons(livePeople.Users),
			PrivacyConsents:         presence,
			SettingsService:         liveSettings.Settings,
			StudentStatusDayService: presenceservice.NewStatusDays(repoFactoryCare.StudentStatusDay, svc.ManualPartialAbsences(students.CarePlan), store.Pool(), students.CarePlan.LockExceptionDay),
			UserContextService:      caller,
		},
		deleteStudentsHTTP.ResourceConfig{
			CareLifecycleService: lifecycle,
			ConstraintViolation:  transactionFailures.IsConstraintViolation,
			PeopleDirectory:      peopleServices.PeopleDirectory,
			StudentDeletion:      studentDeletion,
			UserContextService:   caller,
		},
		reviewStudentsHTTP.ResourceConfig{
			PeopleDirectory:     peopleServices.PeopleDirectory,
			RequestReview:       requestReview,
			RequestReviewAccess: owners.NewParentRequestReviewPolicy(liveCaller.Caller().ParentRequestReviews),
		},
	)

	router := func(transportClock func() time.Time) chi.Router {
		if transportClock != nil {
			care.Now = transportClock
			projection.Now = transportClock
			status.Now = transportClock
		}

		return people.Router(ProtectedTenantRoutesWith(care.Register, projection.Register, status.Register, deletion.Register, review.Register, groupLiveHTTP.NewResource(groupLive).Register))
	}
	return people, care, projection, status, deletion, review, router, nil

}
func newStudentHTTPTestReviews(store transactionruntime.Database, svc StudentTestModule, live *owners.CallerRows, liveSettings ActiveSettings, peopleServices StudentTestPeople, studentRows carePlanCompose.StudentDirectory, clock func() time.Time) (requestreview.Query, careplan.CareScheduleReviewQuery, error) {
	reviewGroups, err := owners.NewSchoolStructure(store)
	if err != nil {
		return nil, nil, err
	}
	reviewStudents, err := reviewbindings.NewStudentDirectory(peopleServices.PeopleDirectory, reviewGroups)
	if err != nil {
		return nil, nil, err
	}
	reviewAccess, err := reviewbindings.NewAccess(reviewStudentsHTTP.RequestReviewPrincipal, nil)
	if err != nil {
		return nil, nil, err
	}
	// The production root binds the same Identity & Access policy for the
	// projection as for the decisions (#3804).
	reviews := live.Caller().ParentRequestReviews
	reviewScope := func(ctx context.Context) (careplan.ReviewScope, error) {
		schoolWide, groupIDs, err := reviews.ReviewScope(ctx, sessiontoken.PermissionsFromCtx(ctx))
		return careplan.ReviewScope{SchoolWide: schoolWide, GroupIDs: groupIDs}, err
	}

	masterDataReviews, err := NewRequestReviewMasterData(store,
		studentRows,
		peopleServices.PeopleDirectory,
		reviewScope,
		func() careplan.Date {
			return careplan.Date(calendar.DateFromTime(clock()))
		},
		func(carePlanCompose.Observation) {})
	if err != nil {
		return nil, nil, err
	}
	careReviews, err := NewRequestReviewSchedules(store,
		studentRows,
		peopleServices.PeopleDirectory,
		reviewScope,
		func(ctx context.Context) (bool, error) {
			return liveSettings.Settings.ResolveBool(ctx, reviewsettings.BookingsAuthoritative)
		},
		func() careplan.Date {
			return careplan.Date(calendar.DateFromTime(clock()))
		},
		func(carePlanCompose.Observation) {},
		func(timetableCompose.Observation) {},
		NewStudentHTTPTestPickupReviewBlocks(store))
	if err != nil {
		return nil, nil, err
	}
	careQueue, err := reviewbindings.NewCareScheduleQueue(careReviews, func() careplan.Date {
		return careplan.Date(calendar.DateFromTime(clock()))
	})
	if err != nil {
		return nil, nil, err
	}
	offeringReviews, err := NewRequestReviewOfferings(store,
		peopleServices.PeopleDirectory,
		reviewScope,
		func() careplan.Date {
			return careplan.Date(calendar.DateFromTime(clock()))
		},
		func(carePlanCompose.Observation) {},
		func(timetableCompose.Observation) {})
	if err != nil {
		return nil, nil, err
	}
	offeringQueue, err := reviewbindings.NewOfferingQueue(offeringReviews, func() careplan.Date {
		return careplan.Date(calendar.DateFromTime(clock()))
	})
	if err != nil {
		return nil, nil, err
	}
	correctionQueries, err := NewUnobservedCorrectionQueries(store)
	if err != nil {
		return nil, nil, err
	}
	corrections, err := reviewbindings.NewCorrectionLog(correctionQueries, peopleServices.PeopleDirectory, func(ctx context.Context) bool {
		return reviewStudentsHTTP.RequestReviewCorrectionAccess(ctx, live.HasCurrentStaff)
	})
	if err != nil {
		return nil, nil, err
	}
	masterQueue, err := reviewbindings.NewMasterDataQueue(masterDataReviews)
	if err != nil {
		return nil, nil, err
	}
	excusedQueue, err := reviewbindings.NewExcusedQueue(svc.ExcusedRequests, func() careplan.Date {
		return careplan.Date(calendar.DateFromTime(clock()))
	})
	if err != nil {
		return nil, nil, err
	}
	requestReview, err := requestreview.NewChecked(requestreview.Dependencies{
		Queues: requestreview.Queues{DirectCorrections: corrections, MasterData: masterQueue, CareSchedule: careQueue, Offering: offeringQueue, Excused: excusedQueue},
		Access: reviewAccess, Students: reviewStudents, FamilyProtection: reviewbindings.NewFamilyProtection(peopleServices.PeopleDirectory),
		Today: func() requestreview.Date { return requestreview.Date(calendar.DateFromTime(clock())) },
	})
	if err != nil {
		return nil, nil, err
	}

	return requestReview, careReviews, nil
}
