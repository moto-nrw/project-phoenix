// app/api/statistics/export/route.ts
import type { NextRequest } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StatisticsExportRoute" });

function exportQuery(request: NextRequest): string {
  const params = new URLSearchParams();
  for (const key of ["from", "to", "format", "section"] as const) {
    const value = request.nextUrl.searchParams.get(key);
    if (value) params.set(key, value);
  }
  for (const groupId of request.nextUrl.searchParams.getAll("group_id")) {
    if (groupId) params.append("group_id", groupId);
  }
  const query = params.toString();
  return query ? `?${query}` : "";
}

async function proxyExport(query: string, token: string): Promise<Response> {
  const backendResponse = await fetch(
    `${getServerApiUrl()}/api/statistics/export${query}`,
    {
      headers: { Authorization: `Bearer ${token}` },
      cache: "no-store",
    },
  );

  if (!backendResponse.ok) {
    const body = await backendResponse.text().catch(() => "");
    return new Response(body || "Export failed", {
      status: backendResponse.status,
      headers: { "Cache-Control": "no-store" },
    });
  }
  if (!backendResponse.body) {
    return new Response("No response body from backend", { status: 502 });
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

async function GETHandler(request: NextRequest) {
  try {
    const session = await auth();
    if (!session?.user?.token) {
      return new Response("Unauthorized", { status: 401 });
    }

    const query = exportQuery(request);
    const response = await proxyExport(query, session.user.token);
    if (response.status !== 401) return response;

    const refreshed = await uncachedAuth();
    if (
      !refreshed?.user?.token ||
      refreshed.user.token === session.user.token
    ) {
      return response;
    }
    return proxyExport(query, refreshed.user.token);
  } catch (error) {
    logger.error("statistics_export_proxy_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return new Response("Internal server error", { status: 500 });
  }
}

export const GET = withTenantAuth(GETHandler);
