import "server-only";
import type { NextRequest } from "next/server";
import { problemResponse } from "~/lib/api-response-error.server";
import { createLogger } from "~/lib/logger";
import { SESSION_ACCOUNT_HEADER } from "~/lib/session-account";
import type { SentryPortal } from "~/lib/sentry-context";
import type { ServerSession } from "./shared";
import { MISMATCH_EXEMPT_PATHS } from "./tenant-host-check";

const logger = createLogger({ component: "SessionAccountCheck" });

/**
 * Routes that must work while the cookie holds another account than the
 * tab: the tenant routes that end a school mismatch (switch, token refresh,
 * logout, end of a staff preview, log lines, invitation acceptance) and the
 * logout of every other portal that has its own.
 */
const ACCOUNT_CHECK_EXEMPT_PATHS: ReadonlySet<string> = new Set([
  ...MISMATCH_EXEMPT_PATHS,
  "/api/school/auth/logout",
  "/api/caterer/auth/logout",
  "/api/traeger/auth/logout",
]);

/**
 * Refuses a request whose tab names another account than the session cookie
 * (#5183). All tabs of a portal share one cookie; when another account signs
 * in in tab B, tab A keeps sending requests for its own account until it
 * hears of the change. Without this check the BFF answered them as the new
 * account: its data appeared under the old account's page, and an action in
 * tab A ran as the new account.
 *
 * Returns null when the request may pass: no session (the route answers
 * 401 itself), an exempt route, the same account, or no header. Requests
 * without the header (navigation, server actions, the event stream, image
 * and download links) cannot carry one and pass as before.
 */
export function sessionAccountMismatchResponse(
  request: NextRequest,
  session: ServerSession | null,
  portal: SentryPortal,
): Response | null {
  const sessionAccountId = session?.user?.id;
  if (!sessionAccountId) return null;
  const expectedAccountId = request.headers.get(SESSION_ACCOUNT_HEADER);
  if (expectedAccountId === null) {
    logger.debug("session_account_header_missing", {
      portal,
      path: request.nextUrl.pathname,
    });
    return null;
  }
  if (expectedAccountId === sessionAccountId) return null;
  if (ACCOUNT_CHECK_EXEMPT_PATHS.has(request.nextUrl.pathname)) return null;

  logger.warn("session_account_mismatch", {
    portal,
    session_account_id: sessionAccountId,
  });
  return problemResponse(
    409,
    "The session belongs to another account than this page",
    { code: "identity.session_account_mismatch" },
    request,
  );
}
