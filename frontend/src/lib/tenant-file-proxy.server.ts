import type { NextRequest } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { sendWithTokenRefresh } from "~/lib/token-refresh-retry.server";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { problemResponse } from "./api-response-error.server";
import { forwardBackendResponse } from "./backend-proxy-response.server";
import { clientIdentityHeaders } from "./client-headers.server";
import { createLogger } from "./logger";
import type { ProxyEndpoint } from "./route-proxy-factory.server";
import {
  allowedQueryString,
  extractParams,
  type RouteContext,
} from "./route-wrapper-utils.server";
import { captureBffException } from "./sentry-bff.server";
import { getServerApiUrl } from "./server-api-url";

const logger = createLogger({ component: "TenantFileProxy" });

/**
 * This proxy is all the browser talks to, so a backend header missing here
 * does not exist in production. nosniff and the CSP keep an uploaded file
 * from being re-interpreted as something executable.
 */
const FILE_HEADERS = [
  "Content-Type",
  "Content-Disposition",
  "Content-Length",
  "Content-Security-Policy",
  "X-Content-Type-Options",
] as const;

interface FileProxyOptions {
  /** POST forwards the JSON request body, read once for the retry. */
  readonly method?: "GET" | "POST";
  /** Query parameters passed on to the backend; none by default. */
  readonly forwardQuery?: readonly string[];
}

function fileResponse(backend: Response, request: NextRequest): Response {
  if (!backend.ok) return forwardBackendResponse(backend, request);
  if (!backend.body) {
    return problemResponse(502, "No response body from backend", request);
  }
  const headers = new Headers();
  for (const name of FILE_HEADERS) {
    const value = backend.headers.get(name);
    if (value) headers.set(name, value);
  }
  if (!headers.has("X-Content-Type-Options")) {
    headers.set("X-Content-Type-Options", "nosniff");
  }
  return new Response(backend.body, { status: backend.status, headers });
}

/**
 * Tenant file download: streams the backend's bytes with their file headers,
 * forwards the client identity, and repeats the request once with a refreshed
 * session after a 401. A backend error keeps its status and problem body.
 * Wrap the result in `withPrivateDownloadNoStore` at the route.
 */
export function proxyFile(
  endpoint: ProxyEndpoint,
  options: FileProxyOptions = {},
) {
  const method = options.method ?? "GET";
  return withTenantAuth(async (request: NextRequest, context: RouteContext) => {
    try {
      const session = await auth();
      const token = session?.user?.token;
      if (!token) return problemResponse(401, "Unauthorized", request);

      const path =
        typeof endpoint === "string"
          ? endpoint
          : endpoint(await extractParams(request, context));
      const query = options.forwardQuery
        ? allowedQueryString(request, options.forwardQuery)
        : "";
      const target = `${getServerApiUrl()}${path}${query}`;
      const body = method === "POST" ? await request.text() : undefined;
      const send = (bearer: string) =>
        fetch(target, {
          method,
          headers: {
            Authorization: `Bearer ${bearer}`,
            ...(body === undefined
              ? {}
              : { "Content-Type": "application/json" }),
            ...clientIdentityHeaders(request.headers),
          },
          body,
          cache: "no-store",
        });

      return fileResponse(
        await sendWithTokenRefresh(token, send, () => uncachedAuth()),
        request,
      );
    } catch (error) {
      captureBffException(error, request);
      logger.error("tenant_file_proxy_failed", {
        path: request.nextUrl.pathname,
        error: error instanceof Error ? error.message : String(error),
      });
      return problemResponse(500, "Internal server error", request);
    }
  });
}
