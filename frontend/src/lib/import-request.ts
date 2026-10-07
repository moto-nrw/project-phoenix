import {
  ApiError,
  apiErrorFromBody,
  apiErrorFromResponse,
  transportFetch,
} from "~/lib/api-error";

/**
 * Requests of the import pages (#2517): every failure is an `ApiError` with
 * code, field errors and request ID, so the page shows the catalog text.
 * A request that never reached the API becomes `general.unavailable`.
 */

/** Without a session token the login has expired: the error path sends the
 *  person to the login screen. */
function missingSessionError(): ApiError {
  return new ApiError("No authentication token available", 401);
}

function authorized(token: string | undefined): Record<string, string> {
  if (!token) throw missingSessionError();
  return { Authorization: `Bearer ${token}` };
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
  token: string | undefined,
  formData: FormData,
): Promise<ImportResponse> {
  const response = await transportFetch(url, {
    method: "POST",
    headers: authorized(token),
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
  token: string | undefined,
  filename: string,
): Promise<void> {
  const response = await transportFetch(url, { headers: authorized(token) });
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "Template download failed");
  }
  const blob = await response.blob();
  const objectUrl = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = objectUrl;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(objectUrl);
}
