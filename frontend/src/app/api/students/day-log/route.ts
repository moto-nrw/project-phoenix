// app/api/students/day-log/route.ts
import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import {
  apiGet,
  ApiResponseError,
  handleApiError,
} from "~/lib/api-helpers.server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";

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
    let envelope: { data: unknown };
    try {
      envelope = await apiGet<{ data: unknown }>(endpoint, session.user.token);
    } catch (apiError) {
      if (!(apiError instanceof ApiResponseError) || apiError.status !== 401)
        throw apiError;

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
    return handleApiError(apiError);
  }
}

export const GET = withTenantAuth(GETHandler);
