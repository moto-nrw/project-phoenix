/** Structured HTTP failure shared by browser and domain clients. */
export class ApiError extends Error {
  status?: number;
  code?: string;
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
      payload?.code ||
      (status === undefined ? undefined : errorClassCode(status));
    this.details = payload?.details;
    this.errors = payload?.errors;
    this.instance = payload?.instance;
    this.requestId = payload?.instance;
  }
}

/** Mirrors backend/api/common.ErrorClassCode until generated contracts include it. */
export function errorClassCode(status: number): string {
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
    error.code = wireCode;
  } else if (!error.code) {
    error.code = parsed.code;
  }
  error.details = parsed.details ?? error.details;
  error.errors = parsed.errors ?? error.errors;
  error.instance = parsed.instance ?? error.instance;
  error.requestId = parsed.requestId ?? error.requestId;
  return error;
}
