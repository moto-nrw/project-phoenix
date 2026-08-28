package config

// Settings key constants. Use these instead of string literals to ensure
// compile-time safety when referencing settings keys across the codebase.

// Security settings.
const (
	KeyOGSDevicePIN = "security.ogs_device_pin"

	// MFA settings (issue #1308 — Phase 3).
	KeyMFAMode                 = "security.mfa_mode"
	KeyMFATrustedDeviceEnabled = "security.mfa_trusted_device_enabled"
	KeyMFATrustedDeviceDays    = "security.mfa_trusted_device_days"

	// Account brute-force lockout policy (issue #586 — Rule 12 extraction).
	// Shared by the PIN and MFA failure counters: after
	// KeyAccountLockoutThreshold failed attempts the account is locked for
	// KeyAccountLockoutDurationMinutes minutes.
	KeyAccountLockoutThreshold       = "security.account_lockout_threshold"
	KeyAccountLockoutDurationMinutes = "security.account_lockout_duration_minutes"
)

// MFAMode option values for KeyMFAMode.
const (
	MFAModeOff            = "off"
	MFAModeRequiredAdmins = "required_admins"
	MFAModeRequiredAll    = "required_all"
)

// GDPR / data cleanup settings.
const (
	KeyDataCleanupEnabled        = "gdpr.data_cleanup_enabled"
	KeyDataCleanupTime           = "gdpr.data_cleanup_time"
	KeyDataCleanupTimeoutMinutes = "gdpr.data_cleanup_timeout_minutes"
	// Retention window for active.work_sessions + work_session_breaks +
	// audit.work_session_edits + active.staff_absences. §16 Abs. 2 ArbZG
	// fixes the floor at 2 years; §147 AO / §257 HGB cap the legally
	// defensible ceiling at 8 years.
	KeyGDPRTimeTrackingRetentionDays = "gdpr.time_tracking_retention_days"
	// Retention window (days) for the per-child change history
	// (audit.student_field_edits, issue #1455). The log answers concrete
	// day-to-day Rückfragen ("wer hat das geändert"), not long-term archival,
	// so the window is short and admin-configurable.
	KeyGDPRStudentChangeLogRetentionDays = "gdpr.student_change_log_retention_days"
	// Default visit-data retention window (days) applied when a student's
	// privacy consent does not set its own data_retention_days. Issue #586
	// (Rule 12): the 30-day default + 1..31 bounds moved off the
	// PrivacyConsent model into this per-tenant setting.
	KeyPrivacyConsentRetentionDays = "gdpr.privacy_consent_retention_days"
	// Retention window (days) for PWA standalone-usage rows
	// (iot.pwa_standalone_usage, issue #2189). The metric only needs a
	// 30-day activity window, so stale rows carry no value and are swept.
	KeyGDPRPWAUsageRetentionDays = "gdpr.pwa_usage_retention_days"
	// Retention window (days) for the OGS-internal colleague chat
	// (users.staff_messages, issue #2598). Staff messages are employee personal
	// data, so the window exists from day one rather than being retrofitted.
	KeyGDPRStaffMessageRetentionDays = "gdpr.staff_message_retention_days"
)

// Attendance log / Raumverlauf (student attendance history) settings.
// The former gdpr.attendance_log_scope and gdpr.student_data_scope settings
// were removed in #2329: student access is tenant-wide for verified staff.
const (
	KeyAttendanceLogEnabled  = "gdpr.attendance_log_enabled"
	KeyAttendanceVisibleDays = "gdpr.attendance_visible_days"
	KeyRoomDetailVisibleDays = "gdpr.room_detail_visible_days"
)

// Feedback settings.
const (
	KeyFeedbackEnabled           = "feedback.enabled"
	KeyFeedbackDataRetentionDays = "feedback.data_retention_days"
)

// Checkout button settings (devices tab).
const (
	KeyCheckoutRaumwechselEnabled       = "checkout.raumwechsel_enabled"
	KeyCheckoutSchulhofEnabled          = "checkout.schulhof_enabled"
	KeyCheckoutWCEnabled                = "checkout.wc_enabled"
	KeyCheckoutDailyFromAllRoomsEnabled = "checkout.daily_checkout_from_all_rooms_enabled"
)

// Checkin capacity-detail disclosure settings (issue #1879, devices tab).
// Gate whether the 409 capacity-exceeded responses include the `details`
// object PyrePortal renders as "{Name} ist voll (X/Y ...)". When off, the
// backend omits `details` and the kiosk shows its generic German fallback.
const (
	KeyCheckinActivityCapacityDetailsEnabled = "checkin.activity_capacity_details_enabled"
	KeyCheckinRoomCapacityDetailsEnabled     = "checkin.room_capacity_details_enabled"
)

// IoT device health-monitoring settings (issue #586 — Rule 12 extraction).
// A device counts as "online" when its last_seen timestamp is within
// KeyDeviceOnlineWindowMinutes of now. The 5-minute window moved off the
// iot.Device model into this per-tenant setting.
const (
	KeyDeviceOnlineWindowMinutes = "iot.device_online_window_minutes"
)

// Tracking indicator settings (student card activity indicators).
const (
	KeyTrackingIndicatorsEnabled = "tracking.indicators_enabled"
	KeyTrackingIndicator1        = "tracking.indicator_1"
	KeyTrackingIndicator2        = "tracking.indicator_2"
	KeyTrackingIndicator3        = "tracking.indicator_3"
)

// Time-tracking auto-checkout settings (#1798).
const (
	KeyTrackingAutoCheckoutEnabled      = "tracking.auto_checkout_enabled"
	KeyTrackingAutoCheckoutGraceMinutes = "tracking.auto_checkout_grace_minutes"
)

// Operations settings.
const (
	KeySessionEndEnabled               = "operations.session_end_enabled"
	KeySessionEndTime                  = "operations.session_end_time"
	KeySessionEndTimeoutMinutes        = "operations.session_end_timeout_minutes"
	KeyStudentDailyCheckoutTime        = "operations.student_daily_checkout_time"
	KeyPerStudentCheckoutEnabled       = "operations.per_student_checkout_enabled"
	KeyPerStudentCheckoutDeltaMinutes  = "operations.per_student_checkout_delta_minutes"
	KeySessionCleanupEnabled           = "operations.session_cleanup_enabled"
	KeySessionCleanupIntervalMinutes   = "operations.session_cleanup_interval_minutes"
	KeySessionAbandonedThresholdMin    = "operations.session_abandoned_threshold_minutes"
	KeySessionInactivityTimeoutMin     = "operations.session_inactivity_timeout_minutes"
	KeyOperationalOverviewScope        = "operations.operational_overview_scope"
	KeyStatusFlagClearTime             = "operations.status_flag_clear_time"
	KeySickClearMode                   = "operations.sick_clear_mode"
	KeyExcusedClearMode                = "operations.excused_clear_mode"
	KeyPresenceMode                    = "operations.presence_mode"
	KeyAttendanceWebEnabled            = "attendance.web_enabled"
	KeyAttendanceNFCEnabled            = "attendance.nfc_enabled"
	KeyStudentActivationIntervalMin    = "operations.student_activation_interval_minutes"
	KeyWebSpontaneousActivities        = "attendance.web_spontaneous_activities_enabled"
	KeyStudentPhotosEnabled            = "operations.student_photos_enabled"
	KeyGroupMode                       = "operations.group_mode"
	KeyBirthdayDisplayEnabled          = "operations.birthday_display_enabled"
	KeyBirthdayDisplayIncludeStaff     = "operations.birthday_display_include_staff"
	KeyEmergencyListHealthInfo         = "operations.emergency_list_health_info"
	KeyCareConcept                     = "operations.care_concept"
	KeyRequirePickupOfferingReview     = "operations.require_pickup_offering_review"
	KeyParentSickNoteEnabled           = "operations.parent_sick_note_enabled"
	KeyParentSickRequiresApproval      = "operations.parent_sick_requires_approval"
	KeyParentExcusedRequiresApproval   = "operations.parent_excused_requires_approval"
	KeyParentNotesEnabled              = "operations.parent_notes_enabled"
	KeyParentCareArrivalRequestEnabled = "operations.parent_care_arrival_request_enabled"
	KeyParentCarePickupRequestEnabled  = "operations.parent_care_pickup_request_enabled"
	KeyParentCareModeRequestEnabled    = "operations.parent_care_mode_request_enabled"
	KeyParentMessageStaffNameVisible   = "operations.parent_message_staff_name_visible"
	KeyParentPickupChangeEnabled       = "operations.parent_pickup_change_enabled"
	KeyParentGuardianManagementEnabled = "operations.parent_guardian_management_enabled"
	KeyParentMasterDataEditEnabled     = "operations.parent_master_data_edit_enabled"
	KeyParentMasterDataRequestEnabled  = "operations.parent_master_data_request_enabled"
	KeyParentNewsEnabled               = "operations.parent_news_enabled"
	// Whether colleagues at this school can write to each other inside moto
	// (OGS-internal 1:1 chat, issue #2598). Defaults OFF: a school switches an
	// internal staff channel on deliberately.
	KeyStaffMessagingEnabled           = "operations.staff_messaging_enabled"
	KeyTimeTrackingAccountStartDate    = "operations.time_tracking_account_start_date"
	KeyTimeTrackingEnforcePlannedStart = "operations.time_tracking_enforce_planned_start"
	// F9: stamping outside the tolerance window around the planned shift
	// window requires a reason; the tolerance is configurable per school.
	KeyTimeTrackingRequireDeviationReason    = "operations.time_tracking_require_deviation_reason"
	KeyTimeTrackingDeviationToleranceMinutes = "operations.time_tracking_deviation_tolerance_minutes"
	KeyMealPlanEnabled                       = "operations.meal_plan_enabled"
	// KeyFederalState is the school's Bundesland (ISO 3166-2, e.g. DE-NW).
	// Drives the public-holiday calendar in time tracking (#1418 3a).
	KeyFederalState = "operations.federal_state"
	// KeyNotificationsAbsenceApprovalEmail toggles the absence-request email
	// notifications (request received / approved / declined / Rückfrage, #1419).
	KeyNotificationsAbsenceApprovalEmail = "notifications.absence_approval_email"
	// KeyNotificationsDispatchEnabled is the feature flag for the notification
	// abstraction (#1624). When off (the default), Notify(ctx, event) is a
	// no-op and no channel delivers anything.
	KeyNotificationsDispatchEnabled = "notifications.dispatch_enabled"
	// KeyNotificationsActiveWindowStart / End bound the wall-clock window in
	// which notifications may be delivered at all. Enforced once in the
	// notification router, so it applies to every producer.
	KeyNotificationsActiveWindowStart = "notifications.active_window_start"
	KeyNotificationsActiveWindowEnd   = "notifications.active_window_end"
	// KeyNotificationsAbsenceReportedEnabled lets a school switch off the
	// sick/excused notification entirely. This is a data-minimisation decision
	// about health information, not a matter of personal taste, which is why it
	// exists on top of the per-person opt-in.
	KeyNotificationsAbsenceReportedEnabled = "notifications.absence_reported_enabled"
	// KeyNotificationsOnDutyOnly restricts personal notifications to staff who
	// are currently checked in. An empty presence map fails closed; schools
	// without time tracking must disable this setting.
	KeyNotificationsOnDutyOnly = "notifications.on_duty_only"
	// KeyNotificationsCareCancelledEnabled is the school-wide gate for the
	// automatic parent notice when a care block is cancelled (#2601). It is
	// independent of the parent-news feature flag on purpose: a school can keep
	// its news feed off and still owe families the cancellation notice.
	KeyNotificationsCareCancelledEnabled = "notifications.care_cancelled_enabled"
	// KeyNotificationsCareCancelledDefaultOn pre-selects "Eltern informieren"
	// in the cancel dialog. The person cancelling can always flip it.
	KeyNotificationsCareCancelledDefaultOn = "notifications.care_cancelled_default_on"
	// KeyNotificationsCareCancelledEmail additionally e-mails the notice through
	// the shared outbox, next to the in-app feed entry and the push.
	KeyNotificationsCareCancelledEmail = "notifications.care_cancelled_email"
)

// PresenceMode option values for KeyPresenceMode.
const (
	PresenceModeDetailed = "detailed"
	PresenceModeBinary   = "binary"
)

// GroupMode option values for KeyGroupMode.
const (
	GroupModeFixedGroups = "fixed_groups"
	GroupModeOpenCare    = "open_care"
)

// OperationalOverviewScope option values for KeyOperationalOverviewScope
// (#2380). The setting is the ONLY rule deciding who may see and operate every
// running module of the school; the organisational group mode
// (KeyGroupMode) deliberately no longer grants operational access.
const (
	// OverviewScopeOwn keeps every caller on the modules they supervise
	// themselves. Administrators included — this is the default.
	OverviewScopeOwn = "own"
	// OverviewScopeAdmins opens every running module to administrators.
	OverviewScopeAdmins = "admins"
	// OverviewScopeAllStaff opens every running module to administrators and
	// to every verified staff member of the tenant. Role permissions still
	// decide WHICH actions those callers may perform.
	OverviewScopeAllStaff = "all_staff"
)

// CareConcept option values for KeyCareConcept.
const (
	CareConceptFixedSchedule = "fixed_schedule"
	CareConceptOpenRooms     = "open_rooms"
)

// StatusFlagClearMode option values for KeySickClearMode and KeyExcusedClearMode.
const (
	ClearModeManual      = "manual"
	ClearModeNextCheckin = "next_checkin"
	ClearModeEndOfDay    = "end_of_day"
)

// Invitations settings (auth flows).
const (
	KeyGuardianInvitationTokenExpiryHours = "invitations.guardian_token_expiry_hours"
)

// Guardian / related-accounts settings. Control whether parents may invite
// further guardians to their own child and whether they may revoke another
// account's access. Staff capabilities are always-on (permission-gated).
const (
	KeyGuardianParentInviteMode = "guardians.parent_invite_mode"
	KeyGuardianParentCanRemove  = "guardians.parent_can_remove"
)

// ParentInviteMode option values for KeyGuardianParentInviteMode.
const (
	ParentInviteModeDisabled      = "disabled"       // parents cannot invite
	ParentInviteModeDirect        = "direct"         // invite sent immediately
	ParentInviteModeStaffApproval = "staff_approval" // invite queued for staff approval
)

// Parent-enrollment settings. Tenant-wide behavioural toggles only -
// per-phase overrides (open window, form schema, care offering selection,
// overflow mode, status-reason visibility) live on enrollment.phases
// columns.
const (
	KeyEnrollmentEnabled                                = "enrollment.enabled"
	KeyEnrollmentCollectGradeLevel                      = "enrollment.collect_grade_level"
	KeyEnrollmentCollectSchoolClass                     = "enrollment.collect_school_class"
	KeyEnrollmentCareOfferingsEnabled                   = "enrollment.care_offerings_enabled"
	KeyEnrollmentDefaultActivationMode                  = "enrollment.default_activation_mode"
	KeyEnrollmentNotificationEmails                     = "enrollment.notification_emails"
	KeyEnrollmentChangeRequestEmailNotificationsEnabled = "enrollment.change_request_email_notifications_enabled"
	KeyEnrollmentAutoInviteGuardianOnApprove            = "enrollment.auto_invite_guardian_on_approval"
	KeyEnrollmentOfferingChangesEnabled                 = "enrollment.offering_changes_enabled"
	KeyEnrollmentOfferingChangesLeadDays                = "enrollment.offering_changes_lead_days"
	KeyEnrollmentDuplicateHandling                      = "enrollment.duplicate_handling"
	KeyEnrollmentBookingsAuthoritative                  = "enrollment.bookings_authoritative"
	KeyEnrollmentAllowSubmissionEdit                    = "enrollment.allow_submission_edit"
	KeyEnrollmentRequireCaptcha                         = "enrollment.require_captcha"
	KeyEnrollmentRejectedRetentionDays                  = "enrollment.rejected_retention_days"
	KeyEnrollmentWaitlistEnabled                        = "enrollment.waitlist_enabled"
	KeyEnrollmentNotifyPerDecision                      = "enrollment.notify_per_decision"
	KeyEnrollmentOutboxMaxAttempts                      = "enrollment.outbox_max_attempts"
	KeyEnrollmentOutboxWorkerIntervalSeconds            = "enrollment.outbox_worker_interval_seconds"
	KeyEnrollmentStatusTokenTTLDays                     = "enrollment.status_token_ttl_days"
	KeyEnrollmentCaptchaSiteKey                         = "enrollment.captcha_site_key"
	KeyEnrollmentCaptchaSecretKey                       = "enrollment.captcha_secret_key"
	KeyEnrollmentGradeLevelMax                          = "enrollment.grade_level_max"
	// Per-tenant info texts (Markdown) shown behind each consent
	// checkbox on the public enrollment form. A block is shown only when
	// its matching enable setting is true and its text is non-empty.
	KeyEnrollmentLegalAGBText          = "enrollment.legal_agb_text"
	KeyEnrollmentLegalAGBDocumentURL   = "enrollment.legal_agb_document_url"
	KeyEnrollmentLegalAGBDisplayMode   = "enrollment.legal_agb_display_mode"
	KeyEnrollmentLegalDSGVOText        = "enrollment.legal_dsgvo_text"
	KeyEnrollmentLegalEmailContactText = "enrollment.legal_email_contact_text"
	KeyEnrollmentLegalPhotoText        = "enrollment.legal_photo_text"
	// Whether the AGB/Teilnahmebedingungen block is shown and required on
	// the public form. Default off: NRW imposes no general duty to use
	// AGB, so a Träger without standard terms must not be forced to show a
	// mandatory "AGB akzeptieren" checkbox. Schools that incorporate terms
	// switch this on and fill in enrollment.legal_agb_text.
	KeyEnrollmentLegalTermsEnabled        = "enrollment.legal_terms_enabled"
	KeyEnrollmentLegalDSGVOEnabled        = "enrollment.legal_dsgvo_enabled"
	KeyEnrollmentLegalEmailContactEnabled = "enrollment.legal_email_contact_enabled"
	KeyEnrollmentLegalPhotoEnabled        = "enrollment.legal_photo_enabled"
)

// Enrollment select-option values.
const (
	EnrollmentActivationModeImmediate = "immediate"
	EnrollmentActivationModeScheduled = "scheduled"

	EnrollmentDuplicateHandlingBlock  = "block"
	EnrollmentDuplicateHandlingWarn   = "warn"
	EnrollmentDuplicateHandlingIgnore = "ignore"

	EnrollmentNotifyPerDecisionDigest    = "digest"
	EnrollmentNotifyPerDecisionImmediate = "immediate"

	EnrollmentLegalAGBDisplayModeText = "text"
	EnrollmentLegalAGBDisplayModePDF  = "pdf"
)

// Timetable settings (WP-B7). Per-tenant configuration for the activity
// template → instance materialization pipeline and the staff-facing
// auto-start behaviour. All definitions live in defaults/timetable.go.
const (
	KeyTimetableEnabled                   = "timetable.enabled"
	KeyTimetableMaterializationEnabled    = "timetable.materialization_enabled"
	KeyTimetableMaterializationWeekday    = "timetable.materialization_weekday"
	KeyTimetableMaterializationWeeksAhead = "timetable.materialization_weeks_ahead"
	KeyTimetableAutoStartPlanned          = "timetable.auto_start_planned"
	KeyTimetableAutoEndEnabled            = "timetable.auto_end_enabled"
	KeyTimetableAutoEndGraceMinutes       = "timetable.auto_end_grace_minutes"
	KeyTimetableStartLeadMinutes          = "timetable.start_lead_minutes"
	KeyTimetableEnforcePlannedEnd         = "timetable.enforce_planned_end"
	KeyTimetableOverdueThresholdMinutes   = "timetable.overdue_threshold_minutes"
	KeyTimetableShowExpectedChildrenCount = "timetable.show_expected_children_count"
	// KeyTimetableChildrenPerStaffRatio is the Betreuungsschlüssel: the max
	// number of children one staff member supervises unassisted. Used to
	// derive a block's required staff count (ceil(children/ratio)) instead
	// of a manually-maintained headcount field.
	KeyTimetableChildrenPerStaffRatio = "timetable.children_per_staff_ratio"
	KeyGDPRTimetableRetentionDays     = "gdpr.timetable_retention_days"
	// Display range for the admin weekly calendar (Apple-style grid).
	// Both are HH:MM strings; the UI renders the visible window between them
	// and scrolls if events fall outside.
	KeyTimetableDayStartTime = "timetable.day_start_time"
	KeyTimetableDayEndTime   = "timetable.day_end_time"

	// Pickup buckets for timetable-derived slot lists. Schools use different
	// checkout cutoffs, so list labels and cohort membership are tenant config.
	KeySlotListShortDayCutoff = "timetable.slot_list_short_day_cutoff"
	KeySlotListLongDayCutoff  = "timetable.slot_list_long_day_cutoff"
)

// Info-point display settings (issue #1325). The whole feature is opt-in:
// a school must explicitly enable it before the admin UI and public
// dashboard endpoint become reachable. Definitions live in defaults/display.go.
const (
	KeyDisplayEnabled = "display.enabled"
)

// Reminder settings (issue #1457). Visual-only (no sound) reminders surfaced
// on the staff "Erinnerungen" page. Every type defaults OFF — a school opts in
// per event type. Lead-time minutes control how early an upcoming-pickup /
// activity-start reminder appears. Definitions live in defaults/reminders.go.
const (
	KeyRemindersPickupUpcomingEnabled     = "reminders.pickup_upcoming_enabled"
	KeyRemindersPickupUpcomingLeadMinutes = "reminders.pickup_upcoming_lead_minutes"
	KeyRemindersPickupOverdueEnabled      = "reminders.pickup_overdue_enabled"
	KeyRemindersActivityStartEnabled      = "reminders.activity_start_enabled"
	KeyRemindersActivityStartLeadMinutes  = "reminders.activity_start_lead_minutes"
	KeyRemindersActivityOverdueEnabled    = "reminders.activity_overdue_enabled"
)

// Appointment reminder settings (issue #1671). Unlike the staff reminders
// above, these address guardians: a school-configurable lead time before a
// parent-facing appointment, delivered as the appointment_reminder e-mail and
// (for guardians who asked for it) a push. Definitions live in
// defaults/appointment_reminders.go.
const (
	KeyCalendarAppointmentReminderEnabled   = "calendar.appointment_reminder_enabled"
	KeyCalendarAppointmentReminderLeadHours = "calendar.appointment_reminder_lead_hours"
)

// Payroll foundation settings (#1417 Tranche 2b). Lohnartnummern are
// mandant-specific DATEV numbers — there are NO defaults on purpose: a
// plausible-looking preset would silently bill wrong. Empty means "not
// configured yet"; the later DATEV writers refuse to produce a file until
// the configuration is complete. Definitions live in defaults/payroll.go.
const (
	KeyPayrollLohnartRegelarbeit       = "payroll.lohnart_regelarbeit"
	KeyPayrollLohnartPlusStunden       = "payroll.lohnart_plus_stunden"
	KeyPayrollLohnartAuszahlung        = "payroll.lohnart_auszahlung"
	KeyPayrollLohnartFreizeitausgleich = "payroll.lohnart_freizeitausgleich"
	KeyPayrollLohnartKrank             = "payroll.lohnart_krank"
	KeyPayrollLohnartUrlaub            = "payroll.lohnart_urlaub"
	KeyPayrollLohnartFortbildung       = "payroll.lohnart_fortbildung"

	// Unit per category, only where day values exist (sick/vacation/training).
	// The remaining categories are minute sums with no day representation, so
	// they are always exported in hours and carry no unit setting.
	KeyPayrollEinheitKrank       = "payroll.einheit_krank"
	KeyPayrollEinheitUrlaub      = "payroll.einheit_urlaub"
	KeyPayrollEinheitFortbildung = "payroll.einheit_fortbildung"

	// LODAS file-header identifiers; Lohn und Gehalt does not need them.
	KeyPayrollDatevBeraternummer   = "payroll.datev_beraternummer"
	KeyPayrollDatevMandantennummer = "payroll.datev_mandantennummer"
)

// Payroll unit select values.
const (
	PayrollUnitHours = "stunden"
	PayrollUnitDays  = "tage"
)

// School file storage (#2596).
const (
	// KeyFilesStaffUploadEnabled lets non-admins upload into every folder they
	// can see (and delete their own uploads). Folders and visibility stay with
	// files:manage regardless.
	KeyFilesStaffUploadEnabled = "files.staff_upload_enabled"
	// KeyFilesMaxStorageMB caps the total size of all stored files of a
	// school. Uploads that would exceed it are refused.
	KeyFilesMaxStorageMB = "files.max_storage_mb"
)
