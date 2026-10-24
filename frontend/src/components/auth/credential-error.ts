import { ApiError } from "~/lib/api-error";
import type { ErrorCode } from "~/lib/error-codes.generated";

/** The 401 refusals of a password login, in every portal. */
export const LOGIN_REFUSALS = [
  "identity.invalid_credentials",
  "identity.session_account_inactive",
] as const satisfies readonly ErrorCode[];

/** The 401 refusals of a passkey login. */
export const PASSKEY_LOGIN_REFUSALS = [
  "identity.passkey_login_failed",
  "identity.session_account_inactive",
  "identity.mfa_code_invalid",
] as const satisfies readonly ErrorCode[];

/** The 401 refusal of an invitation accept: the invited account must log in. */
export const INVITATION_REFUSALS = [
  "identity.invitation_account_login_required",
] as const satisfies readonly ErrorCode[];

/**
 * The 401 refusals of an MFA step, in every portal: a wrong or expired code
 * (verify, resend, enrollment confirm), and the credential check of the
 * token exchange after a correct code. Any other 401 there is an expired
 * challenge or enrollment session and leads to the login screen.
 */
export const MFA_STEP_REFUSALS = [
  "identity.mfa_code_invalid",
  "identity.invalid_credentials",
  "identity.session_account_inactive",
] as const satisfies readonly ErrorCode[];

/**
 * The shared error path sends every 401 to the login screen, because a 401
 * normally means the session ran out. A login step answers 401 for refused
 * credentials instead (wrong password, wrong code): there is no session to
 * renew, and a jump to the login page would hide the reason (#2517).
 *
 * Only a 401 whose code is in `codes` counts as such a refusal. It comes back
 * as a copy without the HTTP status, so the form shows the catalog text for
 * the code; code, details, field errors and request ID stay. Every other
 * error, including a 401 without one of these codes, is returned unchanged:
 * no code is invented here (ADR 0006).
 */
export function credentialError(
  error: unknown,
  codes: readonly ErrorCode[],
): unknown {
  if (!(error instanceof ApiError) || error.status !== 401) return error;
  if (!(error.code && codes.includes(error.code))) return error;
  const copy = new ApiError(error.message, undefined, {
    code: error.code,
    details: error.details,
    errors: error.errors,
    instance: error.instance,
  });
  copy.requestId = error.requestId;
  copy.retryAfterSeconds = error.retryAfterSeconds;
  return copy;
}
