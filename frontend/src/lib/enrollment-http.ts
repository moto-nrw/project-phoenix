import { unavailableApiError, type ApiError } from "~/lib/api-error";
import { readEnrollmentError } from "~/lib/enrollment-api-error";

type EnrollmentErrorLogger = Parameters<typeof readEnrollmentError>[2];

/** The query parameter that carries a late-invite token, on pages and API paths. */
export const LATE_INVITE_QUERY_PARAM = "late_invite";

/** Appends the late-invite token to a path or URL; no token, no change. */
export function withLateInviteQuery(
  url: string,
  token: string | null | undefined,
): string {
  const trimmed = token?.trim();
  if (!trimmed) return url;
  const separator = url.includes("?") ? "&" : "?";
  return `${url}${separator}${LATE_INVITE_QUERY_PARAM}=${encodeURIComponent(trimmed)}`;
}

/** A request that never reached the API counts as unavailable (#2515). */
export async function sendEnrollmentRequest(
  input: string,
  init?: RequestInit,
): Promise<Response> {
  try {
    return await fetch(input, init);
  } catch (cause) {
    throw unavailableApiError(cause);
  }
}

/** The backend wraps payloads as `{ status, data }`; bare payloads pass through. */
export async function readEnrollmentJSON<T>(response: Response): Promise<T> {
  const raw = (await response.json()) as { data?: T } | T;
  if (
    raw &&
    typeof raw === "object" &&
    "data" in raw &&
    raw.data !== undefined
  ) {
    return raw.data;
  }
  return raw as T;
}

/** Binds one API module's logger and log event to `readEnrollmentError`. */
export function enrollmentErrorReader(
  logger: EnrollmentErrorLogger,
  event: string,
): (response: Response, fallback: string) => Promise<ApiError> {
  return (response, fallback) =>
    readEnrollmentError(response, fallback, logger, event);
}
