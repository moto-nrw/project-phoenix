import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { captureBffException } from "~/lib/sentry-bff.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import type { NextRequest } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";
import { problemResponse } from "~/lib/api-helpers.server";

const logger = createLogger({
  component: "StudentAttendanceHistoryExportRoute",
});

interface RouteContext {
  params: Promise<{ id: string }>;
}

function exportQuery(request: NextRequest): string {
  const params = new URLSearchParams();
  for (const key of ["format", "from", "to"] as const) {
    const value = request.nextUrl.searchParams.get(key);
    if (value) params.set(key, value);
  }
  const query = params.toString();
  return query ? `?${query}` : "";
}

async function proxyExport(
  request: NextRequest,
  studentId: string,
  query: string,
  token: string,
): Promise<Response> {
  const backendResponse = await fetch(
    `${getServerApiUrl()}/api/students/${encodeURIComponent(studentId)}/attendance-history/export${query}`,
    {
      headers: { Authorization: `Bearer ${token}` },
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

    const { id } = await context.params;
    if (!id) {
      return problemResponse(400, "Student ID is required", request);
    }

    const query = exportQuery(request);
    const response = await proxyExport(request, id, query, session.user.token);
    if (response.status !== 401) return response;

    const refreshed = await uncachedAuth();
    if (
      !refreshed?.user?.token ||
      refreshed.user.token === session.user.token
    ) {
      return response;
    }
    return proxyExport(request, id, query, refreshed.user.token);
  } catch (error) {
    captureBffException(error, request);
    logger.error("attendance_history_export_proxy_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "Internal server error", request);
  }
}

export const GET = withPrivateDownloadNoStore(withTenantAuth(GETHandler));
