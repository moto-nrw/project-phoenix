import type { ErrorCode } from "~/lib/error-codes.generated";

/**
 * Types a wire code for comparison against registry codes, so a literal that
 * is not in error-registry/ fails the type check. The value itself is not
 * validated: an unknown code simply never equals a registered one.
 */
export function wireErrorCode(code: unknown): ErrorCode | undefined {
  return typeof code === "string" && code ? (code as ErrorCode) : undefined;
}

/** Structured HTTP failure shared by browser and domain clients. */
export class ApiError extends Error {
  status?: number;
  code?: ErrorCode;
  /** Backend diagnostic detail. It is for logs, never for user-facing copy. */
  detail?: string;
  details?: Record<string, unknown>;
  errors?: { field: string; reason: string }[];
  instance?: string;
  requestId?: string;
  retryAfterSeconds?: number;
  /** No HTTP response arrived; the status is a stand-in (unavailableApiError). */
  noResponse?: boolean;

  constructor(
    message: string,
    status?: number,
    payload?: {
      code?: string;
      detail?: string;
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
    this.detail = payload?.detail;
    this.details = payload?.details;
    this.errors = payload?.errors;
    this.instance = payload?.instance;
    this.requestId = payload?.instance;
  }
}

/** Mirrors backend/api/common.ErrorClassCode until generated contracts include it. */
export function errorClassCode(status: number): ErrorCode {
  if (status === 401 || status === 403) return "general.permission";
  if (status === 413 || status === 431) return "general.request_too_large";
  if (status === 409 || status === 410 || status === 422)
    return "general.business_rejection";
  if ([408, 429, 499, 502, 503, 504].includes(status))
    return "general.unavailable";
  return status >= 500 ? "general.server" : "general.input";
}

/** A request failed before the API could return an HTTP response. */
export function unavailableApiError(cause?: unknown): ApiError {
  const message =
    cause instanceof Error && cause.message
      ? cause.message
      : "Network request failed";
  const error = new ApiError(message, 503, {
    code: "general.unavailable",
  });
  if (cause instanceof Error && cause.name) error.name = cause.name;
  // Keeps the original failure reachable, e.g. a request a logout discarded.
  if (cause !== undefined) error.cause = cause;
  error.noResponse = true;
  return error;
}

/**
 * `fetch` for domain clients: a request that never reached the API (offline
 * or DNS failure) becomes `general.unavailable` instead of a raw `TypeError`
 * like "Failed to fetch". Callers use AbortSignal to supersede stale work;
 * those cancellations must retain their normal AbortError semantics.
 */
export async function transportFetch(
  input: RequestInfo | URL,
  init?: RequestInit,
): Promise<Response> {
  try {
    return await fetch(input, init);
  } catch (error) {
    if (
      error !== null &&
      typeof error === "object" &&
      "name" in error &&
      error.name === "AbortError"
    ) {
      throw error;
    }
    throw unavailableApiError(error);
  }
}

/**
 * The RFC 9457 error body every API answer carries (ADR 0006). `status` is the
 * HTTP status number, `detail` is developer diagnostics and never shown; the
 * user reads the catalog text for `code`.
 */
export interface ProblemBody {
  type?: string;
  title?: string;
  status: number;
  detail: string;
  instance?: string;
  code: ErrorCode;
  errors?: { field: string; reason: string }[];
  details?: Record<string, unknown>;
  /** Companion-plan 409 of the student PUT (CompanionPlanConflictError). */
  conflicts?: unknown[];
}

/**
 * Builds the envelope for an error that starts in the frontend's own server
 * layer, so the browser reads it like a backend answer. Without a code the
 * error carries the code of its class.
 */
export function problemBody(
  status: number,
  detail: string,
  extra: Omit<Partial<ProblemBody>, "status" | "detail"> = {},
): ProblemBody {
  return {
    type: "about:blank",
    title: problemTitle(status),
    ...extra,
    status,
    detail,
    code: wireErrorCode(extra.code) ?? errorClassCode(status),
  };
}

export function problemTitle(status: number): string {
  const titles: Record<number, string> = {
    400: "Bad Request",
    401: "Unauthorized",
    403: "Forbidden",
    404: "Not Found",
    405: "Method Not Allowed",
    409: "Conflict",
    413: "Content Too Large",
    422: "Unprocessable Content",
    429: "Too Many Requests",
    500: "Internal Server Error",
    502: "Bad Gateway",
    503: "Service Unavailable",
    504: "Gateway Timeout",
  };
  return titles[status] ?? (status >= 500 ? "Server Error" : "Client Error");
}

/** The diagnostic `detail` of an error body, for logs and error messages. */
export function problemDetail(body: unknown): string | undefined {
  if (!body || typeof body !== "object") return undefined;
  const detail = (body as { detail?: unknown }).detail;
  return typeof detail === "string" && detail.trim() ? detail : undefined;
}

const sessionTenantMismatchListeners = new Set<() => void>();

/**
 * Who learns that the BFF refused a request because the session belongs to
 * another school than this subdomain (#5130). The session cache drops its
 * session and wakes the SessionProvider, so TenantGuard switches back;
 * TenantGuard hides the page until then. Error answers that become an
 * ApiError report here, and the fetch observer reports every fetch answer,
 * also of clients that never read the body. Returns the unsubscribe function.
 */
export function onSessionTenantMismatch(listener: () => void): () => void {
  sessionTenantMismatchListeners.add(listener);
  return () => {
    sessionTenantMismatchListeners.delete(listener);
  };
}

export function reportSessionTenantMismatch(): void {
  for (const listener of sessionTenantMismatchListeners) listener();
}

/**
 * Turns an error body into an ApiError. The caller supplies the user-safe
 * message; the envelope's diagnostic `detail` remains separate for logs.
 */
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
  const error = new ApiError(message, status, {
    code: typeof data.code === "string" && data.code ? data.code : undefined,
    detail: problemDetail(data),
    details,
    errors,
    instance: typeof data.instance === "string" ? data.instance : undefined,
  });
  if (error.code === "identity.session_tenant_mismatch") {
    reportSessionTenantMismatch();
  }
  return error;
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
  error.detail = parsed.detail ?? error.detail;
  error.errors = parsed.errors ?? error.errors;
  error.instance = parsed.instance ?? error.instance;
  error.requestId = parsed.requestId ?? error.requestId;
  return error;
}
