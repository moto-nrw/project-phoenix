import type { NextRequest } from "next/server";
import { decodeJwtPayload } from "~/server/auth/jwt-payload";
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
  incomingBackendIdentityHeaders,
} from "./client-headers.server";
import { sanitizeEndpoint } from "./log-sanitize";
import { createLogger } from "./logger";
import { captureBffException } from "./sentry-bff.server";
import {
  createTokenExpiredResponse,
  createUnauthorizedResponse,
} from "./route-wrapper-utils.server";
import { getServerApiUrl } from "./server-api-url";

const logger = createLogger({ component: "BackendClient" });

/**
 * How long the backend may take until its response headers arrive, per kind
 * of call. A body still streaming (a large export) is not cut off.
 */
const BACKEND_TIMEOUTS_MS = {
  /** Below Cloudflare's 100 s origin timeout, so the BFF answers 503 first. */
  default: 90_000,
  /** A password, MFA or passkey step and the BFF logout. */
  login: 10_000,
  logout: 10_000,
  /** Session upkeep inside Auth.js callbacks, which hold up the request:
   * refresh, family and token checks, staff preview, a failed install's
   * revocation. */
  session: 5_000,
  /** An image or document a public page embeds. */
  publicAsset: 5_000,
} as const;

type BackendTimeout = keyof typeof BACKEND_TIMEOUTS_MS;

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
  /** A tighter budget than `default`; see BACKEND_TIMEOUTS_MS. */
  timeout?: BackendTimeout;
}

/** Without a request at hand, the identity of the request being handled. */
async function identityHeaders(
  request: Request | undefined,
): Promise<Record<string, string>> {
  if (!request) return incomingBackendIdentityHeaders();
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

/** The tenant, organization and scope of a call's token, for log and metric. */
function bearerClaims(bearer: string | null | undefined) {
  const claims = bearer ? decodeJwtPayload(bearer) : null;
  const number = (value: unknown) =>
    typeof value === "number" ? value : undefined;
  return {
    tenantId: number(claims?.tenant_id),
    orgId: number(claims?.org_id),
    scope: typeof claims?.scope === "string" ? claims.scope : undefined,
  };
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
    BACKEND_TIMEOUTS_MS[call.timeout ?? "default"],
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
    const claims = bearerClaims(bearer);
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

/**
 * The backend's JSON for a token the caller holds, envelope included; an
 * error answer throws ApiResponseError. The one JSON path of every portal:
 * the portal handlers' `api` and the tenant's `apiGet`/`apiPost`/… use it,
 * and a 401 is repeated by the route handler around it, per endpoint.
 */
export async function backendJson<T>(
  method: BackendMethod,
  endpoint: string,
  token: string,
  body?: unknown,
  options: Pick<BackendCall, "idempotencyKey" | "request" | "signal"> = {},
): Promise<T> {
  const response = await sendBackend(endpoint, token, {
    method,
    json: body ?? undefined,
    ...options,
  });
  return readBackendJson<T>(response);
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
  /** The request's session (cached for the request). */
  auth: () => Promise<ServerSession | null>;
  /** The session renewed now; called after a backend 401. */
  uncachedAuth: () => Promise<ServerSession | null>;
  /** Whether the caller runs inside the portal's response wrapper. */
  inRequestContext: () => boolean;
}

/**
 * Outside the portal's response wrapper a session read can rotate the
 * refresh token without its cookie leaving (#1938), and a renewal cannot
 * force a refresh. A helper that reads the session refuses to run there.
 */
export function assertPortalRequestContext(portal: PortalSession): void {
  if (portal.inRequestContext()) return;
  const error = new Error(
    "portal session helper called outside its portal's auth wrapper",
  );
  logger.error("portal_session_outside_auth_wrapper", { error: error.message });
  throw error;
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
 * What renewing the session after a backend 401 decided (#5194, #5399),
 * for the session's access token or the token `tokenOf` picks:
 * - `renewed`: a new token; the request is repeated with it.
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
  portal: Pick<PortalSession, "uncachedAuth">,
  failed: string,
  tokenOf: (session: ServerSession) => string | undefined = (session) =>
    session.user?.token,
): Promise<Renewal> {
  let session: ServerSession | null;
  try {
    session = await portal.uncachedAuth();
  } catch (error) {
    // The session stays; a renewal that throws every time would otherwise
    // be an endless, invisible series of 503s.
    logger.warn("session_renewal_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    captureBffException(error);
    return { outcome: "undecided" };
  }
  const token = session ? tokenOf(session) : undefined;
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
    assertPortalRequestContext(portal);
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
