import { captureBffException } from "~/lib/sentry-bff.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import type { NextRequest } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { incomingAnalyticsSessionHeaders } from "~/lib/analytics-session-header.server";
import { createLogger } from "~/lib/logger";
import { problemResponse } from "~/lib/api-helpers.server";

// Abholerlaubnis (#4780), OGS: der Nachweis einer unterschriebenen
// Elternanfrage als PDF. Das Backend prüft users:update und den
// Prüfbereich für das Kind; 403/404 laufen unverändert durch.

const logger = createLogger({ component: "CareRequestProofRoute" });

interface RouteContext {
  params: Promise<{ requestId: string }>;
}

async function proxyProof(
  request: NextRequest,
  requestId: string,
  token: string,
): Promise<Response> {
  const backendResponse = await fetch(
    `${getServerApiUrl()}/api/students/care-schedule-change-requests/${encodeURIComponent(requestId)}/proof`,
    {
      headers: {
        Authorization: `Bearer ${token}`,
        ...(await incomingAnalyticsSessionHeaders()),
      },
      cache: "no-store",
    },
  );
  if (!backendResponse.ok) {
    const response = forwardBackendResponse(backendResponse, request);
    response.headers.set("Cache-Control", "no-store");
    return response;
  }
  if (!backendResponse.body) {
    return problemResponse(502, "No response body from backend", request);
  }
  const headers = new Headers({ "Cache-Control": "no-store" });
  for (const name of [
    "Content-Type",
    "Content-Disposition",
    "Content-Length",
  ]) {
    const value = backendResponse.headers.get(name);
    if (value) headers.set(name, value);
  }
  return new Response(backendResponse.body, {
    status: backendResponse.status,
    headers,
  });
}

async function GETHandler(request: NextRequest, context: RouteContext) {
  try {
    const session = await auth();
    if (!session?.user?.token) {
      return problemResponse(401, "Unauthorized", request);
    }
    const { requestId } = await context.params;
    if (!/^\d+$/.test(requestId)) {
      return problemResponse(400, "invalid request id", request);
    }
    const response = await proxyProof(request, requestId, session.user.token);
    if (response.status !== 401) return response;
    const refreshed = await uncachedAuth();
    if (
      !refreshed?.user?.token ||
      refreshed.user.token === session.user.token
    ) {
      return response;
    }
    return proxyProof(request, requestId, refreshed.user.token);
  } catch (error) {
    captureBffException(error, request);
    logger.error("care_request_proof_proxy_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "Internal server error", request);
  }
}

export const GET = withTenantAuth(GETHandler);
