import type { NextRequest } from "next/server";
import type { PortalKey } from "~/server/auth/portals";
import type { ServerSession } from "~/server/auth/session-types";
import {
  ApiResponseError,
  BackendUnavailableError,
  bffError,
  problemResponse,
} from "./api-response-error.server";
import { recordBackendProxyMetric } from "./backend-proxy-metrics";
import {
  backendIdentityHeaders,
  clientIdentityHeaders,
  incomingClientForwardHeaders,
} from "./client-headers.server";
import { sanitizeEndpoint } from "./log-sanitize";
import { createLogger } from "./logger";
import {
  createTokenExpiredResponse,
  createUnauthorizedResponse,
} from "./route-wrapper-utils.server";
import { getServerApiUrl } from "./server-api-url";

const logger = createLogger({ component: "BackendClient" });

/**
 * How long the backend may take until its response headers arrive. Below
 * Cloudflare's 100 s origin timeout, so the BFF answers 503 before the edge
 * does.
 * A body still streaming (a large export) is not cut off.
 */
const BACKEND_HEADERS_TIMEOUT_MS = 90_000;

export type BackendMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

export interface BackendCall {
  method?: BackendMethod;
  /** Serialized as JSON with `Content-Type: application/json`. */
  json?: unknown;
  /** A body sent as it is (form data, file bytes); see `contentType`. */
  body?: BodyInit;
  contentType?: string;
  idempotencyKey?: string | null;
  /** Further headers, such as the login's trusted-device cookie. */
  headers?: Record<string, string>;
  /** The browser request whose client identity travels along. */
  request?: Request;
  signal?: AbortSignal;
  /** A tighter bound than the default header timeout. */
  timeoutMs?: number;
}

/** Without a request at hand, the identity of the request being handled. */
async function identityHeaders(
  request: Request | undefined,
): Promise<Record<string, string>> {
  if (!request) return incomingClientForwardHeaders();
  return "nextUrl" in request
    ? backendIdentityHeaders(request as NextRequest)
    : clientIdentityHeaders(request.headers);
}

/**
 * Next decodes route params, so a raw `..` from the URL could re-target the
 * request at another backend path with the user's token.
 */
function assertNoDotSegments(path: string): void {
  const pathname = path.split(/[?#]/, 1)[0] ?? "";
  for (const segment of pathname.split("/")) {
    let decoded: string;
    try {
      decoded = decodeURIComponent(segment);
    } catch {
      throw bffError(400, "invalid backend path segment");
    }
    // An encoded `..%2F..` stays one segment here but may not at the backend.
    if (decoded.split(/[\\/]/).some((part) => part === "." || part === "..")) {
      throw bffError(400, "invalid backend path segment");
    }
  }
}

function tokenScope(bearer: string | null | undefined): {
  tenantId?: number;
  orgId?: number;
  scope?: string;
} {
  const payload = bearer?.split(".")[1];
  if (!payload) return {};
  try {
    const claims = JSON.parse(Buffer.from(payload, "base64url").toString());
    return {
      tenantId:
        typeof claims.tenant_id === "number" ? claims.tenant_id : undefined,
      orgId: typeof claims.org_id === "number" ? claims.org_id : undefined,
      scope: typeof claims.scope === "string" ? claims.scope : undefined,
    };
  } catch {
    return {};
  }
}

function contentLength(response: Response): number | null {
  const value = response.headers?.get?.("content-length");
  if (!value) return null;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : null;
}

/**
 * The one request path to the backend for every portal: base URL, bearer,
 * client identity, `cache: "no-store"`, Idempotency-Key, a header timeout,
 * and one log line and metric per call.
 */
export async function sendBackend(
  path: string,
  bearer: string | null | undefined,
  call: BackendCall = {},
): Promise<Response> {
  assertNoDotSegments(path);
  const method = call.method ?? "GET";
  const timeout = new AbortController();
  const timer = setTimeout(
    () => timeout.abort(new Error("backend did not answer in time")),
    call.timeoutMs ?? BACKEND_HEADERS_TIMEOUT_MS,
  );
  const body = call.json === undefined ? call.body : JSON.stringify(call.json);
  const contentType =
    call.json === undefined ? call.contentType : "application/json";
  const headers: Record<string, string> = {
    ...(await identityHeaders(call.request)),
    ...call.headers,
    // The session's bearer wins over any header a route adds.
    ...(bearer && { Authorization: `Bearer ${bearer}` }),
    ...(contentType && { "Content-Type": contentType }),
    ...(call.idempotencyKey && { "Idempotency-Key": call.idempotencyKey }),
  };
  const startedAt = Date.now();
  let status = 0;
  let responseBytes: number | null = null;
  let outcome: "success" | "backend_error" | "network_error" = "network_error";
  try {
    const response = await fetch(`${getServerApiUrl()}${path}`, {
      method,
      headers,
      body,
      cache: "no-store",
      signal: call.signal
        ? AbortSignal.any([call.signal, timeout.signal])
        : timeout.signal,
    });
    status = response.status;
    responseBytes = contentLength(response);
    outcome = response.ok ? "success" : "backend_error";
    return response;
  } catch (error) {
    // The caller's own abort (a closed browser request) stays what it is.
    if (call.signal?.aborted) throw error;
    throw new BackendUnavailableError(error);
  } finally {
    clearTimeout(timer);
    const durationMs = Date.now() - startedAt;
    const backendEndpoint = sanitizeEndpoint(path);
    const claims = tokenScope(bearer);
    logger.info("frontend backend proxy request completed", {
      method,
      backend_endpoint: backendEndpoint,
      status,
      duration_ms: durationMs,
      response_bytes: responseBytes,
      tenant_id: claims.tenantId,
      org_id: claims.orgId,
      scope: claims.scope,
      outcome,
    });
    try {
      recordBackendProxyMetric({
        method,
        backendEndpoint,
        status,
        durationMs,
        outcome,
        scope: claims.scope,
      });
    } catch {
      // Metrics must never alter request behavior.
    }
  }
}

/** The JSON of a successful backend answer; an error answer throws. */
export async function readBackendJson<T>(response: Response): Promise<T> {
  if (!response.ok) {
    throw new ApiResponseError(response.status, await response.text(), {
      retryAfter: response.headers?.get?.("Retry-After") ?? null,
      contentType: response.headers?.get?.("Content-Type") ?? null,
    });
  }
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

/** The `data` of the backend's `{ status, data, message }` envelope. */
export function unwrapEnvelope<T>(json: unknown): T {
  if (typeof json === "object" && json !== null && "data" in json) {
    return (json as { data: T }).data;
  }
  return json as T;
}

/** One portal's session, as its Auth.js instance reads it. */
export interface PortalSession {
  key: PortalKey;
  /** The request's session (cached for the request). */
  auth: () => Promise<ServerSession | null>;
  /** The session renewed now; called after a backend 401. */
  uncachedAuth: () => Promise<ServerSession | null>;
}

export interface SessionCall extends Omit<BackendCall, "request"> {
  /**
   * Repeat a backend 401 once with the renewed session (default). Off only
   * for an endpoint that answers a refused credential with 401: the retry
   * would send that credential a second time. Name the reason.
   */
  retry?: true | { off: string };
}

/**
 * What renewing the session after a backend 401 decided (#5194, #5399):
 * - `renewed`: a new access token; the request is repeated with it.
 * - `refused`: the backend refused the refresh (revoked or foreign family,
 *   consumed or invalid refresh token): the session has ended, 401.
 * - `undecided`: the refresh could not be decided (endpoint unreachable,
 *   timeout, 5xx), so the session stays as it is and its own refresh
 *   decides later, 503. A transient rotation leaves the old token in place.
 */
export type Renewal =
  | { outcome: "renewed"; token: string }
  | { outcome: "refused" }
  | { outcome: "undecided" };

export async function renewSession(
  portal: PortalSession,
  failed: string,
): Promise<Renewal> {
  let session: ServerSession | null;
  try {
    session = await portal.uncachedAuth();
  } catch {
    return { outcome: "undecided" };
  }
  const token = session?.user?.token;
  if (!session || session.error || !token) return { outcome: "refused" };
  if (token === failed) return { outcome: "undecided" };
  return { outcome: "renewed", token };
}

/** The answer when the renewal was refused (401) or undecided (503). */
export function renewalFailureResponse(
  outcome: "refused" | "undecided",
  request: Request,
): Response {
  return outcome === "refused"
    ? createTokenExpiredResponse(request)
    : problemResponse(
        503,
        "session renewal could not be decided",
        { code: "general.unavailable" },
        request,
      );
}

/**
 * The backend call bound to the request's session of one portal. Without a
 * session it answers 401 "Unauthorized", after a refused renewal 401 "Token
 * expired", after an undecided one 503 (see Renewal): the BFF's own bodies,
 * the same for every portal and route. A
 * second 401 with the renewed token is the backend's real refusal and keeps
 * its body and code.
 */
export function createBackendFetch(portal: PortalSession) {
  return async function backendFetch(
    request: NextRequest,
    path: string,
    call: SessionCall = {},
  ): Promise<Response> {
    const token = (await portal.auth())?.user?.token;
    if (!token) return createUnauthorizedResponse(request);
    const send = (bearer: string) =>
      sendBackend(path, bearer, { ...call, request });
    const response = await send(token);
    const retries = call.retry === undefined || call.retry === true;
    if (response.status !== 401 || !retries) return response;
    const renewal = await renewSession(portal, token);
    if (renewal.outcome !== "renewed") {
      return renewalFailureResponse(renewal.outcome, request);
    }
    return send(renewal.token);
  };
}
