package api

import (
	"context"
	"log/slog"
	"testing"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type absenceMailEnabled struct{}

func (absenceMailEnabled) AbsenceApprovalEmailEnabled(context.Context) (bool, error) {
	return true, nil
}

type absenceMailContacts struct {
	requester, approver workforceCompose.AbsenceEmailContact
}

func (r absenceMailContacts) GetStaffContactInfo(_ context.Context, id int64) (*workforceCompose.AbsenceEmailContact, error) {
	if id == r.approver.StaffID {
		return &r.approver, nil
	}
	return &r.requester, nil
}
func (r absenceMailContacts) ListAbsenceApprovers(context.Context) ([]*workforceCompose.AbsenceEmailContact, error) {
	return []*workforceCompose.AbsenceEmailContact{&r.approver}, nil
}

type absenceMailSchool struct{}

func (absenceMailSchool) FindSchoolSubdomain(context.Context, int64) (string, bool, error) {
	return "school", true, nil
}

func TestAbsenceMailOutboxIsAtomicWithDecision(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"resubmit", "approve", "decline", "question"} {
		t.Run(action, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db, ctx, outbox := mailTransactionFixture(t)
			database := testpkg.NewDatabase(t, db)
			members := MustNewMembershipTestRepositories(database)
			rows, err := NewWorkSessionTestRepositoriesWithMembers(database, members)
			require.NoError(t, err)
			timetable, err := NewTimetableTestRepositoriesWithMembers(database, members)
			require.NoError(t, err)
			presence, err := NewTimetablePresenceRows(database, timetable, members)
			require.NoError(t, err)
			settings, err := NewSettingsTestModule(database, testpkg.TenantRuntime(t, db))
			require.NoError(t, err)
			people, err := NewRFIDTestPeople(database)
			require.NoError(t, err)
			requester, requesterAccount := testpkg.CreateTestStaffWithAccount(t, db, "Absence", "Requester")
			approver, approverAccount := testpkg.CreateTestStaffWithAccount(t, db, "Absence", "Approver")
			contacts := absenceMailContacts{
				requester: workforceCompose.AbsenceEmailContact{StaffID: requester.ID, FirstName: "Absence", LastName: "Requester", Email: requesterAccount.Email},
				approver:  workforceCompose.AbsenceEmailContact{StaffID: approver.ID, FirstName: "Absence", LastName: "Approver", Email: approverAccount.Email},
			}
			audits := NewTestAuditPersistence(database, nil)
			service, err := newTimeTrackingTestModule(database, rows, members.Staff, presence, settings.Settings, timeTrackingTestGraph{
				Audits: timeTrackingAudits{WorkSessionEdit: audits.WorkSessionEdit, TimeTrackingDeletion: audits.TimeTrackingDeletion, DataAccessLog: audits.DataAccessLog, TimeTrackingAuditLog: audits.TimeTrackingAuditLog},
				Events: timeTrackingEvents(deliveryCompose.NewRealtimeHub(slog.Default())), People: people, Holidays: noHolidays{}, ShiftPlanSync: noSickCascade{}, Logger: slog.Default(),
			}, &workforceCompose.AbsenceEmailDeps{Settings: absenceMailEnabled{}, Dispatcher: absenceEmailDispatcher{durable: outbox, from: email.NewEmail("moto", "no-reply@example.test")},
				StaffRepo: contacts, SchoolRepo: absenceMailSchool{}, FrontendURL: "http://localhost:3000", Logger: slog.Default(),
			})
			require.NoError(t, err)
			status := workforce.AbsenceStatusRequested
			if action == "resubmit" {
				status = workforce.AbsenceStatusQuestion
			}
			var absenceID int64
			day := calendar.NewDate(2099, 1, 5)
			require.NoError(t, db.NewRaw(`INSERT INTO active.staff_absences (tenant_id, staff_id, absence_type, date_start, date_end, status, created_by, requested_at) VALUES (?, ?, ?, ?, ?, ?, ?, NOW()) RETURNING id`, testpkg.Tenant(t), requester.ID, workforce.AbsenceTypeOther, day, day, status, requester.ID).Scan(ctx, &absenceID))
			address, kind := requesterAccount.Email, "absence_request_approved"
			switch action {
			case "resubmit":
				address, kind = approverAccount.Email, "absence_request_received"
			case "decline":
				kind = "absence_request_declined"
			case "question":
				kind = "absence_request_question"
			}
			assertTransactionalMails(t, db, ctx, false, address, []string{kind}, func(txCtx context.Context) error {
				var err error
				switch action {
				case "resubmit":
					_, err = service.StaffAbsences.ResubmitAbsence(txCtx, requester.ID, requesterAccount.ID, absenceID, "Updated request")
				case "approve":
					_, err = service.StaffAbsences.ApproveAbsence(txCtx, absenceID, approverAccount.ID, approver.ID, "Approved")
				case "decline":
					_, err = service.StaffAbsences.DenyAbsence(txCtx, absenceID, approverAccount.ID, approver.ID, "Declined")
				case "question":
					_, err = service.StaffAbsences.QuestionAbsence(txCtx, absenceID, approverAccount.ID, "Clarification")
				}
				return err
			})
		})
	}
}
