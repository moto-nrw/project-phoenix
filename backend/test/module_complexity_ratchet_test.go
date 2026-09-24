package test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/uudashr/gocognit"
)

// TestModuleComplexityRatchet extends .claude/rules/backend-conventions.md
// Rule 4 — gocognit cognitive complexity ≤ 15 — from api/ to the capability
// modules and application workflows.
//
// Why a second ratchet instead of a wider api/ one: migration #2580 moved
// 228,306 of 638,717 production LOC into modules/ and workflows/, and every
// existing quality gate stops at the api/ and models/ directory boundary.
// Moved code left the ratchet's reach without a single line changing. This
// test closes that gap by freezing today's state under the new roots.
//
// TestHandlerComplexityRatchet stays separate and stays at zero. Its empty
// allowlist is a hard-won result (75 offenders retired since 2026-07-12);
// merging the two registers would bury that zero inside a four-hundred-entry
// map, where nobody would notice it growing back.
//
// Allowlist semantics (per function, not per file), identical to the api/
// ratchet:
//
//   - A function NOT in the allowlist must score ≤ 15.
//   - An allowlisted function may never exceed its recorded score.
//   - When refactoring lowers a score (or drops it to ≤ 15), the test fails
//     until the entry is lowered/removed — the ratchet only turns one way.
//     Never raise a number, never add an entry.
//
// Keys are "relative/file.go:FuncName" with methods rendered exactly like the
// gocognit CLI ("(*service).ListOptions"), so CLI output maps 1:1 onto
// entries here. Scan roots are modules/ and workflows/, non-test .go files
// only; "legacy" subdirectories are counted deliberately — the retained
// legacy code is the bulk of the debt this ratchet is meant to drain.
const moduleComplexityThreshold = 15

// Seeded 2026-09-18 at merge commit ecf0003369, the state of modules/ and
// workflows/ right after the #2580 move, measured with:
//
//	cd backend && ../scripts/run-go-toolchain.sh \
//	    go run github.com/uudashr/gocognit/cmd/gocognit -over 15 ./modules ./workflows
//
// (_test.go hits filtered out). 444 functions. The first seed was taken at
// 19feca2822 and re-measured here after the merge of origin/development
// (a47f77b2c3): PR #3408 (issue #3350) moved eleven functions over the limit
// into modules/, which the ratchet cannot absorb without a fresh measurement.
// The entries record the debt as it was on that day; every one of them may
// only shrink or disappear.
//
// Sorted by key, with a blank line between owner modules so gofmt aligns each
// block on its own and removing an entry reflows only that module's block.
var moduleComplexityAllowlist = map[string]int{
	"modules/appointments/recurrence.go:boundedRecurrenceDates": 77,
	"modules/appointments/recurrence.go:matchesRule":            17,

	"modules/careplan/carerequests/weekly.go:weeklyEntries":                                                      39,
	"modules/careplan/internal/adapters/postgres/offering_bookings.go:(*Store).RecordCareOfferingBookings":       16,
	"modules/careplan/internal/adapters/postgres/offering_bookings.go:(*Store).replaceCareOfferingBookings":      17,
	"modules/careplan/internal/adapters/postgres/status_days.go:(*statusDayStore).ArchiveStudentStatusFlags":     23,
	"modules/careplan/internal/adapters/postgres/status_days.go:(*statusDayStore).CountEffectiveStudentAbsences": 16,
	"modules/careplan/internal/adapters/postgres/withdrawal_completions.go:(*Store).ListWithdrawals":             20,
	"modules/careplan/internal/application/excused_requests.go:(*ExcusedRequests).Correct":                       18,
	"modules/careplan/internal/application/excused_requests.go:(*ExcusedRequests).Decide":                        34,
	"modules/careplan/internal/application/excused_requests.go:(*ExcusedRequests).ListHistory":                   20,
	"modules/careplan/internal/application/excused_requests.go:(*ExcusedRequests).ListPending":                   21,
	"modules/careplan/internal/application/offering_review_pending.go:(*OfferingReviews).pendingFacts":           28,
	"modules/careplan/internal/application/offering_review_pending.go:(*OfferingReviews).pendingReviews":         56,
	"modules/careplan/internal/application/offering_review_pending.go:materializeOfferingReview":                 24,
	"modules/careplan/internal/application/offering_reviews.go:(*OfferingReviews).ListHistory":                   21,
	"modules/careplan/internal/application/offering_reviews.go:(*OfferingReviews).ListPending":                   17,
	"modules/careplan/internal/application/review_weekly_plan.go:(reviewPlanFacts).pickupWeek":                   25,
	"modules/careplan/internal/application/schedule_review_pickup.go:(*ScheduleReviews).pickupDiff":              22,
	"modules/careplan/internal/application/schedule_reviews.go:(*ScheduleReviews).ListPending":                   30,
	"modules/careplan/internal/application/schedule_reviews.go:(*ScheduleReviews).plan":                          23,
	// Moved in by PR #3408 (#3350) from services/users and
	// database/repositories/users; the scores crossed the boundary unchanged.
	"modules/careplan/offeringrequests/diff.go:CorrectionDiff": 21,

	"modules/classday/internal/application/slotlists.go:(*service).ListOptions":         108,
	"modules/classday/internal/application/slotlists.go:(*service).buildList":           28,
	"modules/classday/internal/application/slotlists.go:(*service).collectSlotContexts": 20,
	"modules/classday/internal/application/slotlists.go:(*service).enrichEntries":       56,
	"modules/classday/internal/application/slotlists.go:filterRows":                     16,
	"modules/classday/internal/application/slotlists.go:slotHeadingDisambiguation":      29,

	"modules/communication/internal/parentmessages/events.go:(*EventEmitter).EmitChildEvent":                    43,
	"modules/communication/internal/parentmessages/service.go:(*Service).PostMessage":                           21,
	"modules/communication/internal/staffannouncements/care_cancellation.go:(*service).PublishCareCancellation": 16,
	"modules/communication/internal/staffannouncements/letter.go:(*service).ResendFailedEmails":                 37,
	"modules/communication/internal/staffannouncements/letter.go:(*service).queueLetterMailsAs":                 22,
	"modules/communication/internal/staffannouncements/poll.go:(*service).enqueueReminderEmails":                25,
	"modules/communication/internal/staffannouncements/poll.go:(*service).pushPollReminder":                     19,
	"modules/communication/internal/staffannouncements/poll.go:normalizePollOptions":                            18,
	"modules/communication/internal/staffannouncements/reminder.go:(*service).SendDueReminders":                 29,
	"modules/communication/internal/staffannouncements/reminder.go:(*service).UpdateReminder":                   16,
	"modules/communication/internal/staffannouncements/service.go:(*service).Publish":                           48,
	"modules/communication/internal/staffannouncements/service.go:(*service).enqueueAnnouncementEmailsAs":       37,
	"modules/communication/internal/staffannouncements/service.go:(*service).notifyAnnouncementGuardiansWith":   22,
	"modules/communication/internal/staffannouncements/service.go:normalizeDelivery":                            17,
	"modules/communication/internal/staffannouncements/service.go:normalizeInput":                               30,

	"modules/dataimport/compose/opening_references.go:NewOpeningReferences": 21,
	"modules/dataimport/fileformat/helpers.go:MapStudentRow":                27,
	"modules/dataimport/fileformat/template_writer.go:(Decoder).Template":   19,

	"modules/delivery/application/notifications/preferences.go:(*preferenceService).FilterOptedInByType":   20,
	"modules/delivery/application/notifications/preferences.go:(*preferenceService).GetForParent":          27,
	"modules/delivery/application/notifications/service.go:(*router).NotifyBatch":                          24,
	"modules/delivery/application/notifications/service.go:(*router).NotifyDurably":                        20,
	"modules/delivery/application/notifications/service.go:(*router).NotifySynchronously":                  26,
	"modules/delivery/application/notifications/service.go:validate":                                       36,
	"modules/delivery/application/notifications/webpush_channel.go:(*webPushChannel).DeliverDurably":       23,
	"modules/delivery/application/notifications/webpush_channel.go:(*webPushChannel).sendAllSynchronously": 16,

	"modules/devicefleet/internal/application/dashboard.go:(*Service).ResolveDashboard": 17,
	"modules/devicefleet/internal/application/display.go:(*Service).UpdateDisplay":      18,

	"modules/devicescan/internal/application/session_mirror.go:(*sessionMirror).MirrorSession": 17,
	"modules/devicescan/internal/application/systemspace.go:(*Service).systemActivity":         20,

	"modules/enrollment/delete_schema.go:(*Module).DeleteUnusedSchema":                                       27,
	"modules/enrollment/form_schema.go:(*FormField).validateQuestion":                                        34,
	"modules/enrollment/form_schema.go:(*FormLegalBlock).Validate":                                           26,
	"modules/enrollment/form_schema.go:(*FormSchema).Validate":                                               23,
	"modules/enrollment/form_schema.go:(*VisibilityCondition).Validate":                                      20,
	"modules/enrollment/internal/adapters/postgres/change_request_reads.go:(*Store).ChangeRequestsForReview": 17,
	"modules/enrollment/phase.go:(*Phase).Validate":                                                          41,
	"modules/enrollment/publish_schema.go:(*Module).PublishSchema":                                           21,
	"modules/enrollment/selection/automatic_shares.go:AutomaticShares":                                       18,
	"modules/enrollment/selection/materialize.go:MaterializeAdjustments":                                     44,
	"modules/enrollment/selection/materialize.go:autoLunchDaysForTarget":                                     16,
	"modules/enrollment/selection/materialize.go:materializeOfferingSelections":                              44,
	"modules/enrollment/selection/materialize.go:validateCareOfferingSelectionModeWithMissing":               29,
	"modules/enrollment/selection/types.go:(*AvailabilityRule).NormalizeAndValidate":                         21,

	"modules/facilities/compose/legacy/projections.go:historyNames": 26,

	"modules/filestorage/internal/application/attachments.go:(*Service).DeleteAttachment": 29,
	"modules/filestorage/internal/application/attachments.go:(*Service).UploadAttachment": 17,
	"modules/filestorage/internal/application/files.go:(*Service).DeleteFile":             33,
	"modules/filestorage/internal/application/files.go:(*Service).UploadFile":             17,
	"modules/filestorage/internal/application/folders.go:(*Service).DeleteFolder":         23,
	"modules/filestorage/internal/application/folders.go:(*Service).ListFolders":          26,
	"modules/filestorage/internal/application/folders.go:(*Service).UpdateFolder":         21,
	"modules/filestorage/internal/application/folders.go:(*Service).validateAudience":     20,

	"modules/identityaccess/internal/application/account_login.go:(*AccountAuthentication).loadPersonNamesFromMappedTenants":    17,
	"modules/identityaccess/internal/application/account_login.go:(*AccountAuthentication).persistSessionInTransaction":         17,
	"modules/identityaccess/internal/application/account_mfa_admin.go:(*AccountMFAFlows).OperatorSetGlobalMFAOverride":          21,
	"modules/identityaccess/internal/application/account_mfa_admin.go:(*AccountMFAFlows).setTenantOverride":                     22,
	"modules/identityaccess/internal/application/account_mfa_flow.go:(*AccountMFAFlows).resolvePolicy":                          16,
	"modules/identityaccess/internal/application/account_mfa_flow.go:(*AccountMFAFlows).verifyChallengeBound":                   20,
	"modules/identityaccess/internal/application/account_passkey_flow.go:(*AccountPasskeyFlows).verifyLogin":                    24,
	"modules/identityaccess/internal/application/account_provisioning.go:(*AccountProvisioning).LinkSchoolAccount":              29,
	"modules/identityaccess/internal/application/account_provisioning.go:(*AccountProvisioning).RegisterSchoolAccount":          24,
	"modules/identityaccess/internal/application/guardian_invitation.go:(*AccountLifecycle).AcceptGuardianInvitation":           26,
	"modules/identityaccess/internal/application/guardian_relative_access.go:(*AccountLifecycle).ApproveInvitation":             21,
	"modules/identityaccess/internal/application/guardian_relative_access.go:(*AccountLifecycle).ListPendingApprovalsDetailed":  35,
	"modules/identityaccess/internal/application/guardian_relative_access.go:(*AccountLifecycle).createStudentInvitation":       23,
	"modules/identityaccess/internal/application/guardian_relative_access.go:(*AccountLifecycle).revokeAccess":                  44,
	"modules/identityaccess/internal/application/operator_account_access.go:(*OperatorAccountAccess).GrantAccountTenantAccess":  37,
	"modules/identityaccess/internal/application/operator_account_access.go:(*OperatorAccountAccess).RevokeAccountTenantAccess": 38,
	"modules/identityaccess/internal/application/operator_account_access.go:(*OperatorAccountAccess).UpdateAccountTenantRole":   42,
	"modules/identityaccess/internal/application/operator_account_access.go:(*OperatorAccountAccess).loadTenantAccess":          20,
	"modules/identityaccess/internal/application/operator_authentication.go:(*OperatorAuthentication).RefreshToken":             48,
	"modules/identityaccess/internal/application/operator_authentication.go:(*OperatorAuthentication).resolveRefreshHandoff":    18,
	"modules/identityaccess/internal/application/operator_passkey_flow.go:(*OperatorPasskeyFlows).verifyLogin":                  22,
	"modules/identityaccess/internal/application/operator_provisioning.go:(*OperatorProvisioning).InitiateOperatorEmailChange":  25,
	"modules/identityaccess/internal/application/permission_administration.go:(*RoleAdministration).ReplaceRolePermissions":     31,
	"modules/identityaccess/internal/application/role_administration.go:(*RoleAdministration).AssignRoleToAccount":              51,
	"modules/identityaccess/internal/application/school_identity.go:(*AccountLifecycle).resolveIdentityPerson":                  18,
	"modules/identityaccess/internal/application/school_invitation.go:(*SchoolInvitation).AcceptInvitation":                     34,
	"modules/identityaccess/internal/application/school_invitation.go:(*SchoolInvitation).grantAccess":                          17,
	"modules/identityaccess/internal/application/school_login.go:(*AccountAuthentication).findSchoolPortalTenantForAccount":     21,
	"modules/identityaccess/internal/application/school_login.go:(*AccountAuthentication).loginSchoolWithMFAGate":               32,
	"modules/identityaccess/internal/application/school_login.go:(*AccountAuthentication).schoolMintGuard":                      20,
	"modules/identityaccess/internal/application/session_cleanup.go:(*AccountAuthentication).reconcileRevokedSessions":          25,
	"modules/identityaccess/internal/application/session_refresh.go:(*AccountAuthentication).refreshSessionInTransaction":       75,
	"modules/identityaccess/internal/application/session_refresh.go:(*AccountAuthentication).resolveRefreshHandoff":             20,
	"modules/identityaccess/internal/application/session_revocation.go:(*AccountAuthentication).ScheduleAccountWideRevoke":      21,
	"modules/identityaccess/internal/application/session_revocation.go:(*AccountAuthentication).finishScheduledAccountWideWipe": 39,
	"modules/identityaccess/internal/application/session_revocation.go:(*AccountAuthentication).wipeAccountWideIndependently":   16,
	"modules/identityaccess/internal/application/session_validation.go:(*AccountAuthentication).ValidateSessionTokens":          27,
	"modules/identityaccess/internal/application/staff_offboarding.go:(*AccountLifecycle).ExecuteStaffOffboarding":              32,
	"modules/identityaccess/internal/application/staff_offboarding.go:(*AccountLifecycle).staffOffboardingSnapshot":             18,
	"modules/identityaccess/internal/application/staff_preview.go:(*AccountLifecycle).ListStaffPreviewCandidates":               24,
	"modules/identityaccess/internal/application/staff_preview.go:(*AccountLifecycle).StartStaffPreview":                        36,
	"modules/identityaccess/internal/domain/operator_account_access.go:UnambiguousPersonIdentity":                               17,
	"modules/identityaccess/legacy/jwt/tokenauth.go:ParseStructToMap":                                                           17,

	"modules/mealplan/internal/application/service.go:(*Service).DailyParticipants":    20,
	"modules/mealplan/internal/application/service.go:(*Service).resolveParticipation": 20,

	"modules/organizationtenancy/internal/application/provisioning_devices.go:(*Provisioning).listDevices":     27,
	"modules/organizationtenancy/internal/application/provisioning_people.go:(*Provisioning).SoftDeletePerson": 19,
	"modules/organizationtenancy/internal/application/provisioning_schools.go:(*Provisioning).schoolChangeSet": 23,
	"modules/organizationtenancy/internal/application/school.go:(*Service).changeSchoolDeletion":               18,

	"modules/peopledirectory/departure/companion_note.go:NormalizeCompanionNote":                          20,
	"modules/peopledirectory/enrollment_departure.go:normalizeEnrollmentDeparture":                        41,
	"modules/peopledirectory/internal/application/student_photo.go:(*StudentPhotoService).CommitPhoto":    30,
	"modules/peopledirectory/internal/application/student_write.go:(*StudentService).UpdateStudent":       27,
	"modules/peopledirectory/internal/application/student_write.go:(*StudentService).decideStranding":     18,
	"modules/peopledirectory/internal/application/student_write.go:(*StudentService).reconcileCompanions": 19,
	"modules/peopledirectory/student_field_review.go:reviewStudentField":                                  16,

	"modules/planexport/betreuungsplan.go:(*betreuungsplanData).rows":      23,
	"modules/planexport/betreuungsplan.go:(*betreuungsplanData).staffLine": 17,
	"modules/planexport/betreuungsplan.go:(*service).blockColors":          21,
	"modules/planexport/dienstplan.go:(*dienstplanData).rowsByArea":        51,
	"modules/planexport/service.go:(*service).nonWorkingDays":              28,

	"modules/requestreview/compose/corrections.go:(correctionLog).History": 19,
	"modules/requestreview/query.go:(*ListQuery).validate":                 21,

	"modules/schoolcalendar/internal/adapters/postgres/store.go:(*Store).ListDateframes":                                18,
	"modules/schoolcalendar/internal/domain/ical.go:formatRRULE":                                                        22,
	"modules/schoolcalendar/portal/internal/application/feed.go:(*service).ParentCalendarFeedByToken":                   58,
	"modules/schoolcalendar/portal/internal/application/feed.go:(*service).projectStaffCalendarEvents":                  73,
	"modules/schoolcalendar/portal/internal/application/ics.go:(*service).ParentAppointmentICS":                         24,
	"modules/schoolcalendar/portal/internal/application/notifications.go:(*service).dispatchGuardianDevicesAfterCommit": 18,
	"modules/schoolcalendar/portal/internal/application/notifications.go:(*service).notifyGuardians":                    26,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).GetParentAppointmentOverview":             28,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).ListMyParentEvents":                       22,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).RecipientOptions":                         52,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).RespondToParentInvitation":                41,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).buildAppointmentOverview":                 16,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).cancelStaffAppointment":                   18,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).createStaffAppointment":                   22,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).expandAppointmentEvents":                  18,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).expandGuardianAppointmentEvents":          20,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).guardianRecipientStatusForStudents":       16,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).resolveTargets":                           86,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).staffShiftEventsWithCancelled":            19,
	"modules/schoolcalendar/portal/internal/application/service.go:(*service).updateStaffAppointment":                   29,

	"modules/schoolmembership/internal/adapters/postgres/store.go:(*Store).ListGuests":                         18,
	"modules/schoolmembership/internal/adapters/postgres/store.go:(*Store).ListTeachers":                       17,
	"modules/schoolmembership/internal/adapters/postgres/teaching_assignment.go:(*Store).ListGroupAssignments": 16,
	"modules/schoolmembership/internal/application/offboarding.go:(*Service).RetireStaff":                      22,
	"modules/schoolmembership/internal/application/offboarding.go:(*Service).retirementSnapshot":               24,

	"modules/schoolstructure/internal/adapters/postgres/transition_store.go:(*Store).ListTransitions": 16,
	"modules/schoolstructure/internal/application/transition.go:(*Service).UpdateTransition":          32,

	"modules/studentpresence/internal/adapters/postgres/live_groups.go:(*Store).QueryGroupSupervisions": 18,
	"modules/studentpresence/internal/adapters/postgres/live_groups.go:(*Store).QueryLiveGroups":        19,
	"modules/studentpresence/internal/application/live_groups.go:(*Service).QueryGroupSupervisions":     26,
	"modules/studentpresence/internal/application/live_groups.go:(*Service).QueryLiveGroups":            21,
	"modules/studentpresence/internal/application/school_status.go:(*Service).ListSchoolStatuses":       21,

	"modules/supervisiondashboard/supervisiondashboard.go:(*service).loadPresenceSections": 17,

	"modules/timetable/compose/legacy_composition_support.go:dateframeListingFromOptions":                                                            18,
	"modules/timetable/compose/legacy_repository_options.go:applyInstanceStudentCondition":                                                           16,
	"modules/timetable/compose/legacy_supervisor_operations.go:buildReplacementSupervisorIDs":                                                        22,
	"modules/timetable/internal/application/activity_instances.go:(*Service).lockOperationalInstanceStaff":                                           20,
	"modules/timetable/internal/application/pickup_extensions.go:(*Service).pickupExtensionBlocks":                                                   23,
	"modules/timetable/internal/application/recurrence_events.go:(*Service).GenerateRecurrenceEvents":                                                21,
	"modules/timetable/legacy/timetableplanning/attendance_correction.go:(*TimetableDataService).CorrectInstanceStudentAttendance":                   23,
	"modules/timetable/legacy/timetableplanning/attendance_sync_service.go:(*AttendanceSyncService).MirrorCheckOutForVisits":                         20,
	"modules/timetable/legacy/timetableplanning/auto_end_service.go:(*autoEndService).completeIfDue":                                                 20,
	"modules/timetable/legacy/timetableplanning/auto_start_service.go:(*autoStartService).RunForTenant":                                              31,
	"modules/timetable/legacy/timetableplanning/bulk_substitution.go:(*instanceService).executeBulkPlans":                                            29,
	"modules/timetable/legacy/timetableplanning/bulk_substitution.go:(*instanceService).planBulkDays":                                                18,
	"modules/timetable/legacy/timetableplanning/deviation_apply.go:(*instanceService).ApplyDeviations":                                               16,
	"modules/timetable/legacy/timetableplanning/deviation_apply.go:deviationInputInstanceIDs":                                                        16,
	"modules/timetable/legacy/timetableplanning/deviation_apply.go:planAbsences":                                                                     21,
	"modules/timetable/legacy/timetableplanning/deviation_apply.go:planPresences":                                                                    24,
	"modules/timetable/legacy/timetableplanning/deviation_apply.go:planSubstitutionRemovals":                                                         35,
	"modules/timetable/legacy/timetableplanning/deviation_apply.go:planSubstitutions":                                                                30,
	"modules/timetable/legacy/timetableplanning/deviation_apply.go:validateDeviationStaff":                                                           32,
	"modules/timetable/legacy/timetableplanning/deviation_service.go:(*instanceService).ApplySubstitute":                                             26,
	"modules/timetable/legacy/timetableplanning/edited_instance_detection.go:(*materializationService).DetectEditedInWindow":                         48,
	"modules/timetable/legacy/timetableplanning/edited_instance_detection.go:(*materializationService).expectedSlotsOn":                              16,
	"modules/timetable/legacy/timetableplanning/instance_move_staff.go:(*instanceService).executeStaffMove":                                          29,
	"modules/timetable/legacy/timetableplanning/instance_move_staff.go:(*instanceService).planStaffMove":                                             39,
	"modules/timetable/legacy/timetableplanning/instance_series_conversion.go:(*TimetableDataService).templateAssignmentsOn":                         26,
	"modules/timetable/legacy/timetableplanning/instance_series_conversion.go:(*instanceSeriesConversionService).ConvertInstanceToSeries":            26,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).Cancel":                                                       22,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).Complete":                                                     42,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).Reopen":                                                       20,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).Start":                                                        25,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).UpdatePlanned":                                                20,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).absorbUnsupervisedOpenGroups":                                 34,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).broadcastInstanceEvent":                                       29,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).broadcastRestoredVisits":                                      27,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).createInTenantTransaction":                                    20,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).reapplyDeviations":                                            51,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).replaceInstanceAssignments":                                   42,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).setUnderstaffedAck":                                           17,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).snapshotDeviations":                                           21,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).validateInstanceReferences":                                   49,
	"modules/timetable/legacy/timetableplanning/instance_service.go:(*instanceService).validateReopenSupervisorsUnchanged":                           22,
	"modules/timetable/legacy/timetableplanning/materialization_service.go:(*materializationService).materializeTemplate":                            48,
	"modules/timetable/legacy/timetableplanning/materialization_service.go:selectPeriod":                                                             19,
	"modules/timetable/legacy/timetableplanning/roster_reconciler.go:(*RosterReconciler).ReconcileSourcedTemplateRosters":                            43,
	"modules/timetable/legacy/timetableplanning/roster_reconciler.go:(*RosterReconciler).fillInstancesMaterializedDuringAlumnusWindow":               38,
	"modules/timetable/legacy/timetableplanning/template_create_service.go:(*TimetableDataService).createTemplateLocked":                             23,
	"modules/timetable/legacy/timetableplanning/template_create_service.go:normalizeDynamicTargets":                                                  30,
	"modules/timetable/legacy/timetableplanning/template_create_service.go:validateOfferingSourceInput":                                              24,
	"modules/timetable/legacy/timetableplanning/template_create_service.go:validateTemplateCreateInput":                                              16,
	"modules/timetable/legacy/timetableplanning/template_series.go:loadTemplateSeriesSegments":                                                       22,
	"modules/timetable/legacy/timetableplanning/template_series_roster.go:(*TimetableDataService).reconcilePredecessorEnrollmentRows":                34,
	"modules/timetable/legacy/timetableplanning/template_series_roster.go:(*TimetableDataService).reconcilePredecessorInstanceStaff":                 49,
	"modules/timetable/legacy/timetableplanning/template_series_roster.go:(*TimetableDataService).reconcilePredecessorSegmentRoster":                 19,
	"modules/timetable/legacy/timetableplanning/template_series_roster.go:(*TimetableDataService).reconcilePredecessorSupervisorRows":                44,
	"modules/timetable/legacy/timetableplanning/template_split_service.go:(*TemplateSplitService).cascadeEndToSeriesSegments":                        19,
	"modules/timetable/legacy/timetableplanning/template_split_service.go:(*TemplateSplitService).createSuccessorGroup":                              19,
	"modules/timetable/legacy/timetableplanning/template_split_service.go:(*TemplateSplitService).splitInTransaction":                                25,
	"modules/timetable/legacy/timetableplanning/template_update_service.go:(*TimetableDataService).resolvePulledForwardStart":                        19,
	"modules/timetable/legacy/timetableplanning/template_update_service.go:(*TimetableDataService).updateTemplateLocked":                             40,
	"modules/timetable/legacy/timetableplanning/timetable_bridge_service.go:(*TimetableBridgeService).notScheduledForEndedSessions":                  25,
	"modules/timetable/legacy/timetableplanning/timetable_operations_class_block.go:(*timetableOperationsService).EarliestPlannedBlockStartForClass": 23,
	"modules/timetable/legacy/timetableplanning/timetable_operations_service.go:(*timetableOperationsService).PatchAttendance":                       28,
	"modules/timetable/legacy/timetableplanning/timetable_operations_service.go:(*timetableOperationsService).PlannedNow":                            49,
	"modules/timetable/legacy/timetableplanning/timetable_operations_service.go:(*timetableOperationsService).buildRosterWithCareDay":                47,
	"modules/timetable/legacy/timetableplanning/timetable_operations_service.go:(*timetableOperationsService).checkInStudent":                        23,
	"modules/timetable/legacy/timetableplanning/timetable_operations_service.go:(*timetableOperationsService).enrichDayPlan":                         60,
	"modules/timetable/legacy/timetableplanning/timetable_operations_service.go:(*timetableOperationsService).requireRosterStudent":                  24,
	"modules/timetable/legacy/timetableplanning/timetable_operations_service.go:(*timetableOperationsService).rosterWarnings":                        25,
	"modules/timetable/legacy/timetableplanning/timetable_operations_session_blocks.go:(*timetableOperationsService).SessionBlocks":                  21,
	"modules/timetable/legacy/timetableplanning/timetable_read_gaps.go:(*TimetableDataService).ComputeGaps":                                          19,
	"modules/timetable/legacy/timetableplanning/timetable_read_student_week.go:(*TimetableDataService).PreloadStudentWeek":                           26,

	"modules/timetableprojection/projection.go:CourseGroupsForOfferings": 21,

	"modules/workforce/inbound/timetracking/staff_admin_document_cleanup.go:(*StaffAdminResource).CleanupOrphanedStaffDocumentFiles": 19,
	"modules/workforce/internal/adapters/postgres/shift_store.go:applyStaffShiftFilter":                                              25,
	"modules/workforce/internal/adapters/postgres/substitution_store.go:(*Store).ListGroupSubstitutions":                             19,
	"modules/workforce/internal/application/absence.go:(*Service).UpdateAbsenceType":                                                 23,
	"modules/workforce/internal/application/absence_allowance.go:(*Service).PreviewAllowanceRebooking":                               17,
	"modules/workforce/internal/application/absence_allowance.go:(*Service).SetAllowance":                                            22,
	"modules/workforce/internal/application/offboarding.go:(*Service).ExecuteStaffOffboarding":                                       39,
	"modules/workforce/internal/application/service.go:(*Service).ReplaceStaffSchedule":                                              16,
	"modules/workforce/internal/application/substitution.go:(*Service).UpdateGroupSubstitution":                                      16,
	"modules/workforce/internal/domain/absence_allowance.go:BuildAllowanceLedger":                                                    23,
	"modules/workforce/internal/domain/shift.go:ValidateStaffShiftSeries":                                                            20,
	"modules/workforce/internal/domain/staffrecord.go:(StaffMasterData).Validate":                                                    16,
	"modules/workforce/legacy/absence_repositories.go:applyStaffAbsenceCondition":                                                    28,
	"modules/workforce/legacy/absence_repositories.go:staffAbsenceFilterFromOptions":                                                 20,
	"modules/workforce/internal/planning/staff_assignment_service.go:(*staffAssignmentService).ListAssignmentsForStaff":              16,
	"modules/workforce/internal/planning/staff_schedule_overview.go:(*staffScheduleOverviewService).loadOverviewData":                17,
	"modules/workforce/internal/planning/staff_schedule_overview.go:(*staffScheduleOverviewService).resolveWeeklyTargets":            52,
	"modules/workforce/internal/planning/staff_shift_move.go:(*staffShiftService).MoveShift":                                         42,
	"modules/workforce/internal/planning/staff_shift_series_service.go:(*staffShiftSeriesService).SplitSeries":                       38,
	"modules/workforce/internal/planning/staff_shift_series_service.go:(*staffShiftSeriesService).materializeSeries":                 32,
	"modules/workforce/internal/planning/staff_shift_cancellation.go:(*staffShiftService).ApplyCancellation":                         49,
	"modules/workforce/internal/planning/staff_shift_service.go:(*staffShiftService).DeleteShift":                                    16,
	"modules/workforce/internal/planning/staff_shift_service.go:(*staffShiftService).updateShiftWithOptions":                         45,
	"modules/workforce/legacy/substitution_repository.go:(*groupSubstitutionRepository).List":                                        18,
	"modules/workforce/legacy/substitution_repository.go:applyGroupSubstitutionCondition":                                            25,
	"modules/workforce/legacy/substitution_repository.go:groupSubstitutionFilterFromOptions":                                         16,
	"modules/workforce/legacy/timetracking/labor_time_policy.go:EvaluateWorkSessionsLaborTime":                                       16,
	"modules/workforce/legacy/timetracking/labor_time_policy.go:netMinutesByDate":                                                    29,
	"modules/workforce/legacy/timetracking/staff_absence_rebooking.go:(*staffAbsenceService).RebookAbsences":                         21,
	"modules/workforce/legacy/timetracking/staff_absence_rebooking.go:(*staffAbsenceService).previewVacationRebooking":               20,
	"modules/workforce/legacy/timetracking/staff_absence_rebooking.go:(*staffAbsenceService).validateRebookedAbsence":                19,
	"modules/workforce/legacy/timetracking/staff_absence_service.go:(*staffAbsenceService).CreateAbsenceFor":                         18,
	"modules/workforce/legacy/timetracking/staff_absence_service.go:(*staffAbsenceService).PreviewCompTimeBalance":                   17,
	"modules/workforce/legacy/timetracking/staff_absence_service.go:(*staffAbsenceService).UpdateAbsence":                            31,
	"modules/workforce/legacy/timetracking/staff_absence_service.go:(*staffAbsenceService).ensureVacationFits":                       16,
	"modules/workforce/legacy/timetracking/staff_balance_adjustment_service.go:(*staffBalanceAdjustmentService).DeleteAdjustment":    19,
	"modules/workforce/legacy/timetracking/staff_balance_adjustment_service.go:(*staffBalanceAdjustmentService).ResetBalance":        23,
	"modules/workforce/legacy/timetracking/staff_overview_service.go:(*staffOverviewService).GetDashboardSummary":                    18,
	"modules/workforce/legacy/timetracking/staff_overview_service.go:(*staffOverviewService).GetTimeTrackingOverview":                31,
	"modules/workforce/legacy/timetracking/staff_overview_service.go:(*staffOverviewService).addTodayCounters":                       18,
	"modules/workforce/legacy/timetracking/staff_overview_service.go:(*staffOverviewService).buildPrefetch":                          26,
	"modules/workforce/legacy/timetracking/staff_time_export_datev.go:buildDatevLines":                                               22,
	"modules/workforce/legacy/timetracking/staff_time_export_rows.go:(*staffOverviewService).GetMonthExportRows":                     20,
	"modules/workforce/legacy/timetracking/work_session_service.go:(*workSessionService).AutoCheckoutDueSessions":                    53,
	"modules/workforce/legacy/timetracking/work_session_service.go:(*workSessionService).ExportSessions":                             16,
	"modules/workforce/legacy/timetracking/work_session_service.go:(*workSessionService).UpdateSession":                              18,
	"modules/workforce/legacy/timetracking/work_session_service.go:(*workSessionService).buildWeeklySummaries":                       20,
	"modules/workforce/legacy/timetracking/work_session_service.go:(*workSessionService).checkIn":                                    28,
	"modules/workforce/legacy/timetracking/work_session_service.go:(*workSessionService).detectPlannedDeviation":                     22,
	"modules/workforce/legacy/timetracking/work_time_month_service.go:(*workTimeMonthService).GetDailyProjection":                    17,
	"modules/workforce/legacy/timetracking/work_time_month_service.go:(*workTimeMonthService).getOpeningBalanceOnDate":               20,
	"modules/workforce/legacy/timetracking/work_time_month_service.go:walkCreditedAbsenceDays":                                       19,
	"modules/workforce/legacy/worksession_repositories.go:(staffBalanceAdjustmentRepository).List":                                   56,
	"modules/workforce/legacy/worksession_repositories.go:(workSessionBreakRepository).List":                                         27,
	"modules/workforce/legacy/worksession_repositories.go:vacationFilterFromOptions":                                                 31,
	"modules/workforce/legacy/worksession_repositories.go:workSessionFilterFromOptions":                                              41,

	"workflows/gradetransition/apply.go:(*Workflow).executeApply":                           20,
	"workflows/gradetransition/class_list_entries.go:(*Workflow).revertClassListEntries":    35,
	"workflows/gradetransition/class_teachers.go:(*Workflow).remapClassTeacherAssignments":  21,
	"workflows/gradetransition/class_teachers.go:(*Workflow).revertClassTeacherAssignments": 31,
	"workflows/gradetransition/compose/new.go:Assemble":                                     17,
	"workflows/gradetransition/preview.go:(*Workflow).Preview":                              17,
	"workflows/gradetransition/revert.go:(*Workflow).Revert":                                16,
	"workflows/gradetransition/revert.go:(*Workflow).restoreGraduateTags":                   19,
	"workflows/gradetransition/transition.go:(*Workflow).History":                           24,

	"workflows/openroommove/internal/application/command.go:(*command).move": 16,

	"workflows/parentportal/care/parent_care_schedule_service.go:(*Service).CreateCareScheduleRequest": 19,
	"workflows/parentportal/care/parent_care_schedule_service.go:(*Service).buildCareScheduleView":     20,
	"workflows/parentportal/care/parent_today_status_service.go:(*Service).GetChildTodayStatus":        20,
	"workflows/parentportal/care/parent_today_status_service.go:(*Service).resolveExpectedArrival":     17,
	"workflows/parentportal/messaging/parent_announcement_service.go:(*Service).RespondToAnnouncement": 35,
	"workflows/parentportal/messaging/parent_announcement_service.go:(*Service).announcementTenants":   16,
	"workflows/parentportal/messaging/parent_announcement_service.go:(*Service).stampAnnouncement":     28,
	"workflows/parentportal/messaging/parent_messaging_service.go:(*Service).PostChildMessage":         24,

	"workflows/reminderdelivery/internal/application/batch.go:(*service).ComputeBatch":                       20,
	"workflows/reminderdelivery/internal/application/batch.go:(*service).loadBatchConfig":                    20,
	"workflows/reminderdelivery/internal/application/batch.go:(*service).loadBatchInputs":                    95,
	"workflows/reminderdelivery/internal/application/delivery.go:(*preparation).dispatchReminderPushes":      57,
	"workflows/reminderdelivery/internal/application/delivery.go:(*preparation).prepareReminderPushDispatch": 58,
	"workflows/reminderdelivery/internal/application/service.go:(*service).Compute":                          29,
	"workflows/reminderdelivery/internal/application/service.go:buildActivityReminders":                      18,

	"workflows/sessionend/internal/application/command.go:(*command).EndSession": 19,

	"workflows/staffoffboarding/compose/new.go:New":                  33,
	"workflows/staffoffboarding/offboarding.go:(*Workflow).snapshot": 17,

	"workflows/studentdeletion/compose/new.go:Assemble":                         27,
	"workflows/studentdeletion/deletion.go:(*Workflow).checkCompanionStranding": 19,
	"workflows/studentdeletion/deletion.go:(*Workflow).loadCounts":              17,
	"workflows/studentdeletion/deletion.go:(*Workflow).lockCompanionGraph":      19,
}

func TestModuleComplexityRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	scores, err := moduleComplexityScan(backendRoot)
	if err != nil {
		t.Fatalf("complexity scan failed: %v", err)
	}

	violations := moduleComplexityViolations(scores, moduleComplexityAllowlist)
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Module-complexity ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// moduleComplexityViolations compares per-function scores over the threshold
// against the allowlist. Like its api/ counterpart it cannot reuse
// ratchetViolations: entries here are capped scores, not hit counts, and an
// allowlisted function dropping to or below the threshold disappears from the
// scan entirely (which is the "ratchet down" signal, not a stale-entry error
// in the per-file sense).
func moduleComplexityViolations(scores, allowlist map[string]int) []string {
	var violations []string
	for fn, got := range scores {
		allowed, ok := allowlist[fn]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s scores %d (limit %d), and is not in the allowlist.\n  Move branching/orchestration behind a capability method (Rule 4) — do not add new entries.",
				fn, got, moduleComplexityThreshold))
		case got > allowed:
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s scores %d, allowed %d.\n  Complexity grew. Simplify the function — never raise the allowlist.", fn, got, allowed))
		case got < allowed:
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s scores %d, allowlist says %d.\n  Nice — ratchet the entry down to %d so the improvement cannot regress.", fn, got, allowed, got))
		}
	}
	for fn, allowed := range allowlist {
		if _, ok := scores[fn]; !ok {
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s is allowlisted at %d but now scores ≤ %d (or was deleted/renamed).\n  Remove the entry.",
				fn, allowed, moduleComplexityThreshold))
		}
	}
	return violations
}

// moduleComplexityScan returns gocognit scores above the threshold for every
// function in non-test .go files under modules/ and workflows/, keyed
// "relpath:FuncName" with methods rendered like the gocognit CLI
// ("(*service).name"). It shares funcDisplayName with the api/ ratchet so
// both registers speak the same key language.
func moduleComplexityScan(backendRoot string) (map[string]int, error) {
	scores := make(map[string]int)
	for _, sub := range []string{"modules", "workflows"} {
		root := filepath.Join(backendRoot, sub)
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if strings.HasPrefix(d.Name(), ".") && path != root {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, relErr := filepath.Rel(backendRoot, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)

			fset := token.NewFileSet()
			file, parseErr := parser.ParseFile(fset, path, nil, 0)
			if parseErr != nil {
				return fmt.Errorf("parse %s: %w", rel, parseErr)
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				if score := gocognit.Complexity(fn); score > moduleComplexityThreshold {
					scores[rel+":"+funcDisplayName(fn)] = score
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return scores, nil
}
