// app/api/statistics/report/route.ts
import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import {
  apiGet,
  handleApiError,
  problemResponse,
} from "~/lib/api-helpers.server";
import { executeWithTokenRefresh } from "~/lib/token-refresh-retry.server";
import { auth, uncachedAuth, withTenantAuth } from "~/server/auth";

/**
 * Proxy for GET /api/statistics/report (Statistik, #2606).
 *
 * The backend enforces config:read + users:read, validates the window and
 * writes the data-access audit row. We forward the whitelisted query
 * parameters and surface the response shape unchanged.
 */
async function GETHandler(request: NextRequest) {
  const session = await auth();
  if (!session?.user?.token) {
    return problemResponse(401, "Unauthorized", request);
  }

  const queryParams = new URLSearchParams();
  for (const key of ["from", "to"] as const) {
    const value = request.nextUrl.searchParams.get(key);
    if (value) queryParams.set(key, value);
  }
  // section, group_id and school_class are repeatable on the backend: forward every value,
  // not just the first, or a two-section request silently loses a section.
  for (const key of ["section", "group_id", "school_class"] as const) {
    for (const value of request.nextUrl.searchParams.getAll(key)) {
      if (value) queryParams.append(key, value);
    }
  }
  const endpoint = `/api/statistics/report?${queryParams.toString()}`;

  try {
    return await executeWithTokenRefresh(
      request,
      session.user.token,
      (token) => apiGet<{ data: unknown }>(endpoint, token),
      (envelope) =>
        NextResponse.json(
          { status: "success", data: envelope.data },
          { headers: { "Cache-Control": "no-store" } },
        ),
      () => uncachedAuth(),
    );
  } catch (apiError) {
    return handleApiError(apiError, request);
  }
}

export const GET = withTenantAuth(GETHandler);
