import { ApiError } from "~/lib/api-error";
import type { ErrorCode } from "~/lib/error-codes.generated";

/**
 * The shared error path sends every 401 to the login screen, because a 401
 * normally means the session ran out. A login step answers 401 for refused
 * credentials instead (wrong password, wrong code): there is no session to
 * renew, and a jump to the login page would hide the reason (#2517).
 *
 * Returns a copy without the HTTP status, so the form shows the catalog text
 * for the code. Code, details, field errors and request ID stay. Without
 * `codes` every 401 counts as a refusal (forms before a session exists);
 * with `codes` only those do, so an expired session in an authenticated
 * form still leads to the login screen.
 */
export function credentialError(
  error: unknown,
  codes?: readonly ErrorCode[],
): unknown {
  if (!(error instanceof ApiError) || error.status !== 401) return error;
  if (codes && !(error.code && codes.includes(error.code))) return error;
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
