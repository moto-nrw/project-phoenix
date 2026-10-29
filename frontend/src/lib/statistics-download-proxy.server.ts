import type { NextRequest } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";
import { problemResponse } from "~/lib/api-helpers.server";
import { captureBffException } from "~/lib/sentry-bff.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";

const logger = createLogger({ component: "StatisticsDownloadProxy" });

/**
 * Query of a Statistik download: the single-valued keys plus every
 * group_id, which the backend reads as a repeatable filter.
 */
function statisticsDownloadQuery(
  request: NextRequest,
  keys: readonly string[],
): string {
  const params = new URLSearchParams();
  for (const key of keys) {
    const value = request.nextUrl.searchParams.get(key);
    if (value) params.set(key, value);
  }
  for (const groupId of request.nextUrl.searchParams.getAll("group_id")) {
    if (groupId) params.append("group_id", groupId);
  }
  const query = params.toString();
  return query ? `?${query}` : "";
}

async function proxyDownload(
  request: NextRequest,
  backendPath: string,
  query: string,
  token: string,
): Promise<Response> {
  const backendResponse = await fetch(
    `${getServerApiUrl()}${backendPath}${query}`,
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

/**
 * Streams a Statistik export from the backend, retrying once with a
 * refreshed session when the backend answers 401.
 */
export async function proxyStatisticsDownload(
  request: NextRequest,
  backendPath: string,
  keys: readonly string[],
): Promise<Response> {
  try {
    const session = await auth();
    if (!session?.user?.token) {
      return problemResponse(401, "Unauthorized", request);
    }

    const query = statisticsDownloadQuery(request, keys);
    const response = await proxyDownload(
      request,
      backendPath,
      query,
      session.user.token,
    );
    if (response.status !== 401) return response;

    const refreshed = await uncachedAuth();
    if (
      !refreshed?.user?.token ||
      refreshed.user.token === session.user.token
    ) {
      return response;
    }
    return proxyDownload(request, backendPath, query, refreshed.user.token);
  } catch (error) {
    captureBffException(error, request);
    logger.error("statistics_export_proxy_failed", {
      path: backendPath,
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "Internal server error", request);
  }
}
