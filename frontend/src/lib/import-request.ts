import type { Session } from "next-auth";
import {
  ApiError,
  apiErrorFromBody,
  apiErrorFromResponse,
  transportFetch,
} from "~/lib/api-error";
import { downloadBlob } from "~/lib/file-download";
import { hasSessionAccess } from "~/lib/session-access";

/**
 * Requests of the import pages (#2517): every failure is an `ApiError` with
 * code, field errors and request ID, so the page shows the catalog text.
 * A request that never reached the API becomes `general.unavailable`.
 */

/** Without a session that grants access the login has expired: the error
 *  path sends the person to the login screen. */
function missingSessionError(): ApiError {
  return new ApiError("No authentication token available", 401);
}

/** The import routes read the session cookie; the browser holds no token
 *  (#5005). */
function requireSession(session: Session | null | undefined): void {
  if (!hasSessionAccess(session)) throw missingSessionError();
}

/** The JSON envelope of an import response; `{}` when the body is not JSON. */
async function readEnvelope(
  response: Response,
): Promise<Record<string, unknown>> {
  try {
    const parsed: unknown = await response.json();
    return parsed && typeof parsed === "object"
      ? (parsed as Record<string, unknown>)
      : {};
  } catch {
    // Kein JSON (etwa eine Gateway-Seite): der Status ordnet den Fehler ein.
    return {};
  }
}

export interface ImportResponse {
  readonly ok: boolean;
  readonly status: number;
  readonly body: Record<string, unknown>;
}

/** POSTs the upload. A failed response is returned, not thrown, because a
 *  stopped batch carries the rows that were already saved. */
export async function postImportFile(
  url: string,
  session: Session | null | undefined,
  formData: FormData,
): Promise<ImportResponse> {
  requireSession(session);
  const response = await transportFetch(url, {
    method: "POST",
    credentials: "include",
    body: formData,
  });
  return {
    ok: response.ok,
    status: response.status,
    body: await readEnvelope(response),
  };
}

/** The ApiError of a failed import response. */
export function importResponseError(
  response: ImportResponse,
  message: string,
): ApiError {
  return apiErrorFromBody(message, response.status, response.body);
}

/** Downloads the import template as `filename`. */
export async function downloadImportTemplate(
  url: string,
  session: Session | null | undefined,
  filename: string,
): Promise<void> {
  requireSession(session);
  const response = await transportFetch(url, { credentials: "include" });
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "Template download failed");
  }
  downloadBlob(await response.blob(), filename);
}
