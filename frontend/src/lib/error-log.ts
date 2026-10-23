/**
 * The Loki entry of a failure (#2512): which code, which class, which school,
 * which request. Grafana counts `error_displayed` by `error_code` and
 * `tenant_id` (dashboard `phoenix-error-codes`); the client log route adds
 * `tenant_id`, `portal` and `user_id` from the verified session.
 *
 * The entry is built from identifiers only. These fields of a failure can
 * name a child or a person and are never copied into it:
 * - the displayed text and `ApiError.message` (the caller's sentence)
 * - `ApiError.detail` (backend diagnostics) and `ApiError.details` (the
 *   values the catalog text interpolates, such as a room or a child)
 * - `ApiError.errors[].reason`; only the field name is kept
 * - `Error.message` and the stack of any exception
 * - an SWR cache key, which can carry the search term of the Kindersuche
 */

import { ApiError, errorClassCode } from "~/lib/api-error";
import type { ErrorClass } from "~/lib/error-codes.generated";
import { expectedFailure } from "~/lib/expected-failure";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ErrorDisplay" });

export const ERROR_DISPLAYED = "error_displayed";

/**
 * The registry classes, plus `crash` (an exception, no API answer) and
 * `local` (a failure the screen found itself, without a request).
 */
type LoggedErrorClass = ErrorClass | "crash" | "local";

/** Where the person saw the failure. */
export type ErrorSurface = "toast" | "form" | "load" | "page";

export interface ErrorLogFields {
  error_class: LoggedErrorClass;
  error_code?: string;
  status?: number;
  request_id?: string;
  error_name?: string;
}

// Mirrors ERROR_CODE_CLASSES for the class codes, so callers outside the
// lazily loaded catalog can class a failure without its 90 kB table.
const CLASS_OF_CLASS_CODE: Record<string, ErrorClass> = {
  "general.input": "input",
  "general.request_too_large": "input",
  "general.permission": "permission",
  "general.business_rejection": "business_rejection",
  "general.unavailable": "unavailable",
  "general.server": "server",
};

const ERROR_CODE_PATTERN = /^[a-z][a-z0-9_]*(?:\.[a-z][a-z0-9_]*)+$/;
const ERROR_NAME_PATTERN = /^[A-Za-z]{1,40}$/;
const REQUEST_ID_PATTERN = /^[\w.:/-]{1,128}$/;

/** The class of an HTTP status, as `errorClassCode` assigns it. */
export function statusErrorClass(status: number): ErrorClass {
  return CLASS_OF_CLASS_CODE[errorClassCode(status)] ?? "server";
}

function errorName(error: unknown): string | undefined {
  const name = error instanceof Error ? error.name : undefined;
  return name && ERROR_NAME_PATTERN.test(name) ? name : undefined;
}

/**
 * The identifiers of a failure. The class of an API error follows its
 * status; the display path passes the class of the code instead.
 */
export function errorLogFields(error: unknown): ErrorLogFields {
  const name = errorName(error);
  if (!(error instanceof ApiError)) {
    const message = error instanceof Error ? error.message : "";
    // Only the browser's texts for a request without a response; a status
    // the message names (HTTP error! status: 409) is no transport failure.
    const expected = expectedFailure({ error: message }, "client");
    const transport = expected === "network" || expected === "aborted";
    return {
      error_class: transport ? "unavailable" : "crash",
      ...(name && { error_name: name }),
    };
  }
  const status = error.noResponse ? undefined : error.status;
  return {
    error_class: error.noResponse
      ? "unavailable"
      : statusErrorClass(status ?? 500),
    ...(error.code &&
      ERROR_CODE_PATTERN.test(error.code) && { error_code: error.code }),
    ...(status !== undefined && { status }),
    ...(error.requestId &&
      REQUEST_ID_PATTERN.test(error.requestId) && {
        request_id: error.requestId,
      }),
    ...(name && { error_name: name }),
  };
}

/**
 * A crash page or notice. The class is `crash` whatever was thrown; an API
 * error thrown while rendering keeps its code for the search.
 */
export function logDisplayedCrash(error: unknown): void {
  logDisplayedError("page", { ...errorLogFields(error), error_class: "crash" });
}

/** One entry per failure a person saw, whatever surface showed it. */
export function logDisplayedError(
  surface: ErrorSurface,
  fields: ErrorLogFields & { fields?: readonly string[] },
): void {
  logger.warn(ERROR_DISPLAYED, { surface, ...fields });
}
