// app/api/timetable/periods/bootstrap/route.ts
//
// POST /api/timetable/periods/bootstrap
//   Idempotent default-period setup. The backend creates a default
//   school-year period when the tenant has none and returns
//   { periods: [...], created: boolean }.
//
// The Go backend wraps responses in { status, data, message }. Strip that
// envelope here so route-wrapper does not double-wrap the payload.
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

export const POST = createPostHandler(
  async (_request: NextRequest, body: unknown, token: string) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/timetable/periods/bootstrap",
      token,
      body ?? {},
    );
    return response.data;
  },
);
