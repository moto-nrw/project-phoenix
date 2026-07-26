// app/api/students/day-log/route.ts
import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createLogger } from "~/lib/logger";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";

const logger = createLogger({ component: "StudentsDayLogRoute" });

/**
 * Proxy for GET /api/students/day-log (Tagesauswertung, #1456).
 *
 * The backend enforces the feature flag (`gdpr.attendance_log_enabled`),
 * the group scope (admins/all_staff see every group, supervisors only their
 * own), the retrospective visibility cap, and writes the audit row. We just
 * forward the request and surface the response shape unchanged.
 */
async function GETHandler(request: NextRequest): Promise<NextResponse> {
  const session = await auth();

  if (!session?.user?.token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const queryParams = new URLSearchParams();
  for (const key of ["date", "group_id"] as const) {
    const value = request.nextUrl.searchParams.get(key);
    if (value) queryParams.set(key, value);
  }
  const queryString = queryParams.toString();
  const endpoint = `/api/students/day-log${queryString ? `?${queryString}` : ""}`;

  try {
    const envelope = await apiGet<{ data: unknown }>(
      endpoint,
      session.user.token,
    );
    return NextResponse.json(
      { status: "success", data: envelope.data },
      { headers: { "Cache-Control": "no-store" } },
    );
  } catch (apiError) {
    const message =
      apiError instanceof Error ? apiError.message : String(apiError);

    // apiGet throws errors in the format "API error (STATUS): body".
    const statusMatch = message.match(/API error \((\d+)\)/);
    const status = statusMatch?.[1] ? Number.parseInt(statusMatch[1], 10) : 500;

    if (status === 403) {
      let code = "feature_disabled";
      if (message.includes("not_group_supervisor"))
        code = "not_group_supervisor";
      if (message.includes("no_permitted_groups")) code = "no_permitted_groups";
      return NextResponse.json({ error: code }, { status: 403 });
    }
    if (status === 400) {
      return NextResponse.json({ error: "invalid_request" }, { status: 400 });
    }

    logger.error("day_log_fetch_failed", { error: message });
    return NextResponse.json(
      { error: `Backend API error: ${message}` },
      { status },
    );
  }
}

export const GET = withTenantAuth(GETHandler);
