import { NextResponse } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy for GET /api/statistics/report (Statistik, #2606).
 *
 * The backend enforces config:read + users:read, validates the window and
 * writes the data-access audit row. We forward the whitelisted query
 * parameters and surface the response shape unchanged.
 */
export const GET = createGetHandler(async (request, token) => {
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
  const envelope = await apiGet<{ data: unknown }>(
    `/api/statistics/report?${queryParams.toString()}`,
    token,
  );
  return NextResponse.json(
    { status: "success", data: envelope.data },
    { headers: { "Cache-Control": "no-store" } },
  );
});
