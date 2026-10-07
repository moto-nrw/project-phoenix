import type { ErrorCode } from "~/lib/error-codes.generated";

/**
 * Every registered code a response to the operator portal can carry (#2519),
 * traced from the operator BFF routes to the handlers of the /operator router.
 * The completeness test requires each code beyond the general.* fallbacks to
 * have its own German text, so an operator never reads only the class
 * sentence. The operator portal is German only.
 *
 * A new operator route or a new code on one belongs here.
 */
export const OPERATOR_PORTAL_ERROR_CODES = [
  // Fallback of every route (`backend/api/common/problem.go`, ErrorClassCode):
  // malformed requests, invalid IDs, refused tokens, rate limits, server errors.
  "general.input",
  "general.permission",
  "general.business_rejection",
  "general.unavailable",
  "general.server",
  // Login, refresh, active-operator check
  // (`modules/identityaccess/inbound/operator/errors.go`, `middleware.go`).
  "identity.invalid_credentials",
  "identity.account_inactive",
  // Second factor and passkeys (`mfa.go`, `passkeys.go`).
  "identity.mfa_code_invalid",
  "identity.mfa_blocked",
  "identity.mfa_not_enrolled",
  "identity.mfa_already_enrolled",
  "identity.passkey_login_failed",
  "identity.passkey_not_found",
  // Profile, password and e-mail change (`errors.go`, `operator_profile.go`).
  "identity.current_password_wrong",
  "identity.password_too_weak",
  "identity.email_already_exists",
  "identity.email_change_rate_limited",
  "identity.email_change_same_email",
  "identity.email_change_link_invalid",
  // Operator invitations (`invitations.go`).
  "identity.invitation_not_found",
  "identity.invitation_rate_limited",
  "identity.operator_invitation_invalid",
  // School access of accounts and school-account MFA (`errors.go`,
  // `mfa_admin.go`).
  "identity.account_not_found",
  "identity.tenant_access_not_found",
  "identity.account_already_has_tenant_access",
  "identity.account_not_in_school",
  "identity.lehrkraft_role_immutable",
  // Provisioning of organisations, schools, devices, persons and accounts
  // (`modules/organizationtenancy/inbound/operator/provisioning_error_renderer.go`).
  "identity.username_taken",
  "identity.password_mismatch",
  "provisioning.organization_slug_taken",
  "provisioning.school_subdomain_taken",
  "provisioning.school_slug_taken",
  "provisioning.device_api_key_taken",
  "provisioning.device_id_taken",
  "provisioning.organization_not_found",
  "provisioning.organization_already_deleted",
  "provisioning.organization_not_deleted",
  "provisioning.organization_has_schools",
  "provisioning.organization_deleted",
  "provisioning.school_not_found",
  "provisioning.school_inactive",
  "provisioning.school_already_deleted",
  "provisioning.school_not_deleted",
  "provisioning.device_not_found",
  "provisioning.device_in_use",
  "provisioning.device_protected",
  "provisioning.device_transfer_protected",
  "provisioning.device_online",
  "provisioning.device_active_session",
  "provisioning.device_other_organization",
  "provisioning.device_same_school",
  "provisioning.person_not_found",
  "provisioning.person_has_supervisions",
  "provisioning.caregiver_capability_blocked",
  // School settings (`modules/settings/inbound/operator/settings.go`).
  "settings.not_found",
  "settings.admin_only",
  "settings.managed_by_upload",
  "settings.invalid_value",
  "settings.presence_mode_switch_blocked",
  "settings.booking_authority_blocked",
  // Announcements, unregistered RFID scans, billing.
  "communication.announcement_not_found",
  "devices.tag_scan_not_found",
  "devices.tag_scan_already_resolved",
  "billing.invalid_key_day",
] as const satisfies readonly ErrorCode[];
