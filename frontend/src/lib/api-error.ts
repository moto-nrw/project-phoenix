import type { ErrorCode } from "~/lib/error-codes.generated";

/**
 * Types a wire code for comparison against registry codes, so a literal that
 * is not in error-registry.json fails the type check. The value itself is not
 * validated: an unknown code simply never equals a registered one.
 */
export function wireErrorCode(code: unknown): ErrorCode | undefined {
  return typeof code === "string" && code ? (code as ErrorCode) : undefined;
}

/** Structured HTTP failure shared by browser and domain clients. */
export class ApiError extends Error {
  status?: number;
  code?: ErrorCode;
  details?: Record<string, unknown>;
  errors?: { field: string; reason: string }[];
  instance?: string;
  requestId?: string;
  retryAfterSeconds?: number;

  constructor(
    message: string,
    status?: number,
    payload?: {
      code?: string;
      details?: Record<string, unknown>;
      errors?: { field: string; reason: string }[];
      instance?: string;
    },
  ) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code =
      wireErrorCode(payload?.code) ||
      (status === undefined ? undefined : errorClassCode(status));
    this.details = payload?.details;
    this.errors = payload?.errors;
    this.instance = payload?.instance;
    this.requestId = payload?.instance;
  }
}

/** Mirrors backend/api/common.ErrorClassCode until generated contracts include it. */
export function errorClassCode(status: number): ErrorCode {
  if (status === 401 || status === 403) return "general.permission";
  if (status === 409 || status === 410 || status === 422)
    return "general.business_rejection";
  if ([408, 429, 499, 502, 503, 504].includes(status))
    return "general.unavailable";
  return status >= 500 ? "general.server" : "general.input";
}

export function apiErrorFromBody(
  message: string,
  status: number,
  body: unknown,
): ApiError {
  const data =
    body && typeof body === "object" ? (body as Record<string, unknown>) : {};
  const details =
    data.details &&
    typeof data.details === "object" &&
    !Array.isArray(data.details)
      ? (data.details as Record<string, unknown>)
      : undefined;
  const errors = Array.isArray(data.errors)
    ? data.errors.filter(
        (entry): entry is { field: string; reason: string } =>
          !!entry &&
          typeof entry === "object" &&
          typeof entry.field === "string" &&
          typeof entry.reason === "string",
      )
    : undefined;
  return new ApiError(message, status, {
    code: typeof data.code === "string" && data.code ? data.code : undefined,
    details,
    errors,
    instance: typeof data.instance === "string" ? data.instance : undefined,
  });
}

/**
 * For clients that already read the body as text: keeps their message and
 * takes code, field errors and request ID from the envelope when the text is
 * JSON.
 */
export function apiErrorFromText(
  message: string,
  status: number,
  text: string,
): ApiError {
  let body: unknown;
  try {
    body = text ? JSON.parse(text) : undefined;
  } catch {
    body = undefined;
  }
  return apiErrorFromBody(message, status, body);
}

/**
 * For clients that hold the failed `Response`: keeps their message and takes
 * code, field errors and request ID from the envelope. A body that cannot be
 * read still yields the status class.
 */
export async function apiErrorFromResponse(
  response: Response,
  message: string,
): Promise<ApiError> {
  let text = "";
  try {
    text = await response.text();
  } catch {
    // Body already consumed or the stream broke: the status still classifies.
  }
  return apiErrorFromText(message, response.status, text);
}

/** Add wire fields without replacing the domain error's message or type. */
export function enrichApiError<T extends ApiError>(
  error: T,
  body: unknown,
  status = error.status,
): T {
  if (status === undefined) return error;
  const parsed = apiErrorFromBody(error.message, status, body);
  error.status = status;
  const wireCode =
    body && typeof body === "object" && "code" in body
      ? (body as { code?: unknown }).code
      : undefined;
  if (typeof wireCode === "string" && wireCode) {
    error.code = wireErrorCode(wireCode);
  } else if (!error.code) {
    error.code = parsed.code;
  }
  error.details = parsed.details ?? error.details;
  error.errors = parsed.errors ?? error.errors;
  error.instance = parsed.instance ?? error.instance;
  error.requestId = parsed.requestId ?? error.requestId;
  return error;
}
