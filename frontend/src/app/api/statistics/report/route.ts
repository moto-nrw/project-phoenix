// app/api/statistics/report/route.ts
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
  // section and group_id are repeatable on the backend: forward every value,
  // not just the first, or a two-section request silently loses a section.
  for (const key of ["section", "group_id"] as const) {
    for (const value of request.nextUrl.searchParams.getAll(key)) {
      if (value) queryParams.append(key, value);
    }
  }
  const endpoint = `/api/statistics/report?${queryParams.toString()}`;

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
