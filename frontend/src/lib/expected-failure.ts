/**
 * Failures the app expects and handles, which are not defects (#3694): a
 * client request that never reached the server, an expired session (401), and a
 * business-rule conflict (409, e.g. a stale review, a past date, a room still
 * in use). The logger records them as warn, so they stay in the logs and as
 * Sentry breadcrumbs without opening a Sentry issue. 403, 5xx, exceptions and
 * unreadable responses stay errors.
 */

export type ExpectedFailure = "network" | "unauthorized" | "conflict";

// The browser's TypeError text for a request without a response (Chrome,
// Safari, Firefox). Callers often prefix it: "TypeError: Load failed",
// "Error fetching students: Failed to fetch".
const NETWORK_ERROR =
  /(?:^|: )(?:Failed to fetch|Load failed|NetworkError when attempting to fetch resource\.?)$/;

// Some API helpers put the status only into the message.
const API_ERROR_STATUS = /API error[:\s(]+(\d{3})/;
const HTTP_ERROR_STATUS = /^(?:Error: )?HTTP error! status: (\d{3})$/;

const EXPECTED_STATUS = new Map<number, ExpectedFailure>([
  [401, "unauthorized"],
  [409, "conflict"],
]);

/** Classifies an error log entry's context by source, `error` and `status`. */
export function expectedFailure(
  context: Record<string, unknown> | undefined,
  source: "server" | "client",
): ExpectedFailure | null {
  if (!context) return null;
  const message = typeof context.error === "string" ? context.error : "";
  const status =
    typeof context.status === "number"
      ? context.status
      : Number(
          API_ERROR_STATUS.exec(message)?.[1] ??
            HTTP_ERROR_STATUS.exec(message)?.[1],
        );
  if (!Number.isNaN(status)) return EXPECTED_STATUS.get(status) ?? null;
  return source === "client" && NETWORK_ERROR.test(message) ? "network" : null;
}

/**
 * The HTTP status an API error carries (`status`, or `httpStatus` on
 * TimetableOperationsApiError), for the `status` field of a log entry.
 */
export function errorStatus(err: unknown): number | undefined {
  if (!err || typeof err !== "object") return undefined;
  const { status, httpStatus } = err as {
    status?: unknown;
    httpStatus?: unknown;
  };
  if (typeof status === "number") return status;
  return typeof httpStatus === "number" ? httpStatus : undefined;
}
