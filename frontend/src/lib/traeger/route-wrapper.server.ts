import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { traegerAuth } from "~/server/auth/traeger";
import { withTraegerAuth } from "~/server/auth/traeger-route";
import { incomingAnalyticsSessionHeaders } from "../analytics-session-header.server";
import { handleApiError } from "../api-helpers.server";
import { forwardBackendResponse } from "../backend-proxy-response.server";
import {
  backendResponseError,
  bffError,
  isApiResponseError,
} from "../api-response-error.server";
import { makeProxyFactories } from "../route-proxy-factory.server";
import {
  buildQueryString,
  createTokenExpiredResponse,
  createUnauthorizedResponse,
  extractParams,
  parseRequestBody,
  wrapInApiResponse,
} from "../route-wrapper-utils.server";

/**
 * Träger-portal route wrapper ("moto träger", #4649). Mirrors
 * lib/school/route-wrapper.server.ts, but reads the Träger NextAuth session
 * (traeger.session-token) so a Träger route can never forward a tenant,
 * parent, or school token. Reads, the holiday period writes (#4659) and the
 * Träger-Büro account routes (#4650) go through the same proxy factories;
 * the access log's CSV export (#4655) and the logo (#4653) stream through
 * proxyFile, the logo upload goes through proxyUpload.
 */

async function traegerServerFetch<T>(
  endpoint: string,
  token: string,
  options: { method: string; body?: unknown },
): Promise<T> {
  const { getServerApiUrl } = await import("~/lib/server-api-url");
  const response = await fetch(`${getServerApiUrl()}${endpoint}`, {
    method: options.method,
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
      ...(await incomingAnalyticsSessionHeaders()),
    },
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });

  if (!response.ok) {
    throw await backendResponseError(response);
  }
  if (response.status === 204) {
    return undefined as T;
  }

  const json: unknown = await response.json();
  // Unwrap the backend envelope { status, data, message } of common.Respond().
  if (
    typeof json === "object" &&
    json !== null &&
    "data" in json &&
    "status" in json
  ) {
    return (json as { data: T }).data;
  }
  return json as T;
}

/** A refreshed Träger session token, or null when there is none. */
async function refreshedToken(originalToken: string): Promise<string | null> {
  const { uncachedTraegerAuth } = await import("~/server/auth/traeger");
  const updated = await uncachedTraegerAuth();
  if (!updated?.user?.token || updated.user.token === originalToken) {
    return null;
  }
  return updated.user.token;
}

type NoBodyHandler<T> = (
  request: NextRequest,
  token: string,
  params: Record<string, unknown>,
) => Promise<T>;

type WithBodyHandler<T, B> = (
  request: NextRequest,
  body: B,
  token: string,
  params: Record<string, unknown>,
) => Promise<T>;

/**
 * Runs the request and, after a backend 401, once more with a refreshed
 * session. Only a missing refresh or a second 401 counts as an expired
 * session; any other answer of the retry (a business rejection, a null
 * DELETE result) is the answer of the request.
 */
async function executeWithRetry<T>(
  request: NextRequest,
  token: string,
  execute: (token: string) => Promise<T>,
): Promise<NextResponse> {
  try {
    return NextResponse.json(wrapInApiResponse(await execute(token)));
  } catch (error) {
    if (!isApiResponseError(error, 401)) throw error;
  }
  let fresh: string | null = null;
  try {
    fresh = await refreshedToken(token);
  } catch {
    // The refresh itself failed: the session counts as expired.
  }
  if (fresh === null) return createTokenExpiredResponse(request);
  try {
    return NextResponse.json(wrapInApiResponse(await execute(fresh)));
  } catch (retryError) {
    if (isApiResponseError(retryError, 401)) {
      return createTokenExpiredResponse(request);
    }
    throw retryError;
  }
}

function createNoBodyHandler<T>(handler: NoBodyHandler<T>) {
  return withTraegerAuth(async (request, context): Promise<NextResponse> => {
    try {
      const session = await traegerAuth();
      const token = session?.user?.token;
      if (!token) return createUnauthorizedResponse(request);
      const params = await extractParams(request, context);
      return await executeWithRetry(request, token, (current) =>
        handler(request, current, params),
      );
    } catch (error) {
      return handleApiError(error, request);
    }
  });
}

function createWithBodyHandler<T, B>(handler: WithBodyHandler<T, B>) {
  return withTraegerAuth(async (request, context): Promise<NextResponse> => {
    try {
      const session = await traegerAuth();
      const token = session?.user?.token;
      if (!token) return createUnauthorizedResponse(request);
      const params = await extractParams(request, context);
      const body = await parseRequestBody<B>(request);
      return await executeWithRetry(request, token, (current) =>
        handler(request, body, current, params),
      );
    } catch (error) {
      return handleApiError(error, request);
    }
  });
}

/**
 * Träger proxy factories: envelope-unwrapping pass-throughs on the Träger
 * session. The fetchers already unwrap `data`.
 */
export const { proxyGet, proxyPost, proxyPut, proxyDelete } =
  makeProxyFactories({
    get: createNoBodyHandler,
    post: createWithBodyHandler,
    put: createWithBodyHandler,
    del: createNoBodyHandler,
    apiGet: (endpoint, token) =>
      traegerServerFetch(endpoint, token, { method: "GET" }),
    apiPost: (endpoint, token, body) =>
      traegerServerFetch(endpoint, token, { method: "POST", body }),
    apiPut: (endpoint, token, body) =>
      traegerServerFetch(endpoint, token, { method: "PUT", body }),
    apiDelete: (endpoint, token) =>
      traegerServerFetch(endpoint, token, { method: "DELETE" }),
    fetcherUnwrapsData: true,
  });

async function traegerServerFile(
  endpoint: string,
  token: string,
): Promise<Response> {
  const { getServerApiUrl } = await import("~/lib/server-api-url");
  return fetch(`${getServerApiUrl()}${endpoint}`, {
    method: "GET",
    headers: {
      Authorization: `Bearer ${token}`,
      ...(await incomingAnalyticsSessionHeaders()),
    },
    cache: "no-store",
  });
}

/** Hands the backend's file through, with its type and file name. */
function fileResponse(backend: Response, request: NextRequest): Response {
  if (!backend.ok || !backend.body) {
    const response = forwardBackendResponse(backend, request);
    response.headers.set("Cache-Control", "no-store");
    return response;
  }
  const headers = new Headers({ "Cache-Control": "no-store" });
  for (const name of [
    "Content-Type",
    "Content-Disposition",
    "Content-Length",
  ]) {
    const value = backend.headers.get(name);
    if (value) headers.set(name, value);
  }
  return new Response(backend.body, { status: backend.status, headers });
}

/**
 * GET proxy for a Träger file export (#4655): forwards the query and streams
 * the file back. A backend error keeps its problem body, so the client shows
 * the catalog text for its code. The endpoint may depend on the route's path
 * parameters, like the export of a holiday period's registrations (#4663).
 */
export function proxyFile(
  endpoint: string | ((params: Record<string, unknown>) => string),
) {
  return withTraegerAuth(async (request: NextRequest, context) => {
    try {
      const session = await traegerAuth();
      const token = session?.user?.token;
      if (!token) return createUnauthorizedResponse(request);

      const path =
        typeof endpoint === "string"
          ? endpoint
          : endpoint(await extractParams(request, context));
      const target = `${path}${buildQueryString(request)}`;
      const first = await traegerServerFile(target, token);
      if (first.status !== 401) return fileResponse(first, request);

      const { uncachedTraegerAuth } = await import("~/server/auth/traeger");
      const updated = await uncachedTraegerAuth();
      if (!updated?.user?.token || updated.user.token === token) {
        return createTokenExpiredResponse(request);
      }
      const retried = await traegerServerFile(target, updated.user.token);
      if (retried.status === 401) return createTokenExpiredResponse(request);
      return fileResponse(retried, request);
    } catch (error) {
      return handleApiError(error, request);
    }
  });
}

/** Room for the multipart boundaries around the largest admitted file. */
const UPLOAD_OVERHEAD_BYTES = 64 * 1024;

async function traegerServerUpload(
  endpoint: string,
  token: string,
  contentType: string,
  body: ArrayBuffer,
): Promise<Response> {
  const { getServerApiUrl } = await import("~/lib/server-api-url");
  return fetch(`${getServerApiUrl()}${endpoint}`, {
    method: "PUT",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": contentType,
      ...(await incomingAnalyticsSessionHeaders()),
    },
    body,
    cache: "no-store",
  });
}

/**
 * Reads the body up to limit bytes and stops reading as soon as it grows
 * beyond, also without a Content-Length. Null means too large.
 */
async function readBounded(
  request: NextRequest,
  limit: number,
): Promise<ArrayBuffer | null> {
  if (!request.body) return new ArrayBuffer(0);
  const reader = request.body.getReader();
  const chunks: Uint8Array[] = [];
  let size = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    size += value.byteLength;
    if (size > limit) {
      await reader.cancel();
      return null;
    }
    chunks.push(value);
  }
  const body = new Uint8Array(size);
  let offset = 0;
  for (const chunk of chunks) {
    body.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return body.buffer;
}

/**
 * PUT proxy for a multipart upload on the Träger session, such as the logo
 * (#4653). The body is read once within maxFileBytes plus multipart room, so
 * it can be sent again after a refresh; the backend checks type, size and
 * content. A backend refusal keeps its problem body.
 */
export function proxyUpload(endpoint: string, maxFileBytes: number) {
  return withTraegerAuth(async (request: NextRequest) => {
    try {
      const session = await traegerAuth();
      const token = session?.user?.token;
      if (!token) return createUnauthorizedResponse(request);

      const contentType = request.headers.get("content-type") ?? "";
      if (!contentType.startsWith("multipart/form-data")) {
        throw bffError(400, "Expected a multipart upload");
      }
      const limit = maxFileBytes + UPLOAD_OVERHEAD_BYTES;
      const declared = Number(request.headers.get("content-length") ?? "0");
      if (declared > limit) {
        throw bffError(400, "Upload too large", {
          code: "traeger.logo_too_large",
        });
      }
      const body = await readBounded(request, limit);
      if (body === null) {
        throw bffError(400, "Upload too large", {
          code: "traeger.logo_too_large",
        });
      }

      const first = await traegerServerUpload(
        endpoint,
        token,
        contentType,
        body,
      );
      if (first.status !== 401) return forwardBackendResponse(first, request);

      const { uncachedTraegerAuth } = await import("~/server/auth/traeger");
      const updated = await uncachedTraegerAuth();
      if (!updated?.user?.token || updated.user.token === token) {
        return createTokenExpiredResponse(request);
      }
      const retried = await traegerServerUpload(
        endpoint,
        updated.user.token,
        contentType,
        body,
      );
      if (retried.status === 401) return createTokenExpiredResponse(request);
      return forwardBackendResponse(retried, request);
    } catch (error) {
      return handleApiError(error, request);
    }
  });
}
