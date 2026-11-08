import type { ErrorCode } from "~/lib/error-codes.generated";

/** The step a password login still needs. The backend answers 200 with this
 * status, not an error; the login page drives it through lib/mfa-api. */
type MfaLoginStep = "mfa_required" | "mfa_enrollment_required";

/** A login the backend throttled (429). Its answer carries only the class code
 * general.unavailable, whose text asks to try again at once. */
export const LOGIN_RATE_LIMITED = "rate_limited";

/** The `code` of the CredentialsSignin a refused portal login throws. */
export type LoginRefusalCode =
  ErrorCode | MfaLoginStep | typeof LOGIN_RATE_LIMITED;
