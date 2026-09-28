import * as Sentry from "@sentry/nextjs";
import { REQUEST_ID_HEADER } from "~/lib/request-id";

/** Report a failure produced by the BFF, not a backend HTTP response. */
export function captureBffException(error: unknown, request?: Request): void {
  const requestId = request?.headers?.get(REQUEST_ID_HEADER);
  // JSON.parse SyntaxError messages can embed slices of the input, including
  // credentials from request bodies. Keep the type but not the raw message.
  const safeError =
    error instanceof SyntaxError ? new SyntaxError("Invalid JSON") : error;
  Sentry.captureException(safeError, {
    ...(requestId ? { tags: { request_id: requestId } } : {}),
  });
}
