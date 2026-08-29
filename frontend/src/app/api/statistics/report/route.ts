// app/api/statistics/report/route.ts
import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createLogger } from "~/lib/logger";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";

const logger = createLogger({ component: "StatisticsReportRoute" });

/**
 * Proxy for GET /api/statistics/report (Statistik, #2606).
 *
 * The backend enforces config:read + users:read, validates the window and
 * writes the data-access audit row. We forward the whitelisted query
 * parameters and surface the response shape unchanged.
 */
async function GETHandler(request: NextRequest): Promise<NextResponse> {
  const session = await auth();
  if (!session?.user?.token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const queryParams = new URLSearchParams();
  for (const key of ["from", "to"] as const) {
    const value = request.nextUrl.searchParams.get(key);
    if (value) queryParams.set(key, value);
  }
  for (const groupId of request.nextUrl.searchParams.getAll("group_id")) {
    if (groupId) queryParams.append("group_id", groupId);
  }
  const endpoint = `/api/statistics/report?${queryParams.toString()}`;

  try {
    let envelope: { data: unknown };
    try {
      envelope = await apiGet<{ data: unknown }>(endpoint, session.user.token);
    } catch (apiError) {
      const message =
        apiError instanceof Error ? apiError.message : String(apiError);
      if (!message.includes("API error (401)")) throw apiError;

      const refreshed = await uncachedAuth();
      if (
        !refreshed?.user?.token ||
        refreshed.user.token === session.user.token
      ) {
        throw apiError;
      }
      envelope = await apiGet<{ data: unknown }>(
        endpoint,
        refreshed.user.token,
      );
    }
    return NextResponse.json(
      { status: "success", data: envelope.data },
      { headers: { "Cache-Control": "no-store" } },
    );
  } catch (apiError) {
    const message =
      apiError instanceof Error ? apiError.message : String(apiError);
    const statusMatch = message.match(/API error \((\d+)\)/);
    const status = statusMatch?.[1] ? Number.parseInt(statusMatch[1], 10) : 500;

    if (status === 403) {
      return NextResponse.json({ error: "forbidden" }, { status: 403 });
    }
    if (status === 400) {
      return NextResponse.json({ error: "invalid_request" }, { status: 400 });
    }

    logger.error("statistics_fetch_failed", { error: message });
    return NextResponse.json(
      { error: `Backend API error: ${message}` },
      { status },
    );
  }
}

export const GET = withTenantAuth(GETHandler);
