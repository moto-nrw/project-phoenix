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
	// Default visit-data retention window (days) applied when a student's
	// privacy consent does not set its own data_retention_days. Issue #586
	// (Rule 12): the 30-day default + 1..31 bounds moved off the
	// PrivacyConsent model into this per-tenant setting.
	KeyPrivacyConsentRetentionDays = "gdpr.privacy_consent_retention_days"
)

// Attendance log / Raumverlauf (student attendance history) settings.
const (
	KeyAttendanceLogEnabled  = "gdpr.attendance_log_enabled"
	KeyAttendanceVisibleDays = "gdpr.attendance_visible_days"
	KeyRoomDetailVisibleDays = "gdpr.room_detail_visible_days"
	KeyAttendanceLogScope    = "gdpr.attendance_log_scope"
)

// AttendanceLogScope option values for KeyAttendanceLogScope.
const (
	AttendanceLogScopeGroupSupervisorsOnly = "group_supervisors_only"
	AttendanceLogScopeAllStaff             = "all_staff"
)

// Student data scope / who can read full student profile data.
const (
	KeyStudentDataScope = "gdpr.student_data_scope"
)

// StudentDataScope option values for KeyStudentDataScope.
const (
	StudentDataScopeGroupSupervisorsOnly = "group_supervisors_only"
	StudentDataScopeAllStaff             = "all_staff"
)

// Feedback settings.
const (
	KeyFeedbackEnabled           = "feedback.enabled"
	KeyFeedbackDataRetentionDays = "feedback.data_retention_days"
)

// Checkout button settings (devices tab).
const (
	KeyCheckoutRaumwechselEnabled = "checkout.raumwechsel_enabled"
	KeyCheckoutSchulhofEnabled    = "checkout.schulhof_enabled"
	KeyCheckoutWCEnabled          = "checkout.wc_enabled"
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
	KeyAdminSupervisionOverview        = "operations.admin_supervision_overview"
	KeyStatusFlagClearTime             = "operations.status_flag_clear_time"
	KeySickClearMode                   = "operations.sick_clear_mode"
	KeyExcusedClearMode                = "operations.excused_clear_mode"
	KeyPresenceMode                    = "operations.presence_mode"
	KeyAttendanceWebEnabled            = "attendance.web_enabled"
	KeyAttendanceNFCEnabled            = "attendance.nfc_enabled"
	KeyWebCheckinAccess                = "attendance.web_checkin_access"
	KeyStudentActivationIntervalMin    = "operations.student_activation_interval_minutes"
	KeyWebSpontaneousActivities        = "attendance.web_spontaneous_activities_enabled"
	KeyStudentPhotosEnabled            = "operations.student_photos_enabled"
	KeyGroupMode                       = "operations.group_mode"
	KeyCareConcept                     = "operations.care_concept"
	KeyParentSickNoteEnabled           = "operations.parent_sick_note_enabled"
	KeyParentExcusedRequiresApproval   = "operations.parent_excused_requires_approval"
	KeyParentNotesEnabled              = "operations.parent_notes_enabled"
	KeyParentMessageStaffNameVisible   = "operations.parent_message_staff_name_visible"
	KeyParentPickupChangeEnabled       = "operations.parent_pickup_change_enabled"
	KeyParentGuardianManagementEnabled = "operations.parent_guardian_management_enabled"
	KeyParentMasterDataEditEnabled     = "operations.parent_master_data_edit_enabled"
	KeyParentMasterDataRequestEnabled  = "operations.parent_master_data_request_enabled"
	KeyParentNewsEnabled               = "operations.parent_news_enabled"
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
)

// PresenceMode option values for KeyPresenceMode.
const (
	PresenceModeDetailed = "detailed"
	PresenceModeBinary   = "binary"
)

// WebCheckinAccess option values for KeyWebCheckinAccess.
const (
	WebCheckinAccessGroupSupervisors = "group_supervisors"
	WebCheckinAccessAllStaff         = "all_staff"
)

// GroupMode option values for KeyGroupMode.
const (
	GroupModeFixedGroups = "fixed_groups"
	GroupModeOpenCare    = "open_care"
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
	KeyEnrollmentDuplicateHandling                      = "enrollment.duplicate_handling"
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
