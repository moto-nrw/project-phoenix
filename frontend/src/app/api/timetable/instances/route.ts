// app/api/timetable/instances/route.ts
//
// GET  /api/timetable/instances?from=YYYY-MM-DD&to=YYYY-MM-DD
//   Proxies the weekly instance list (WP-F2 backend prerequisite
//   instances_list.go).
// POST /api/timetable/instances
//   Creates a planned (spontaneous or template-bound) instance.
//   Backend handler: instances_create.go. Body shape mirrors
//   CreateInstanceBody in lib/timetable-types.ts.
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";

// The Go backend returns { status, data, message }; strip that envelope
// here so route-wrapper does not double-wrap the payload.
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const search = request.nextUrl.searchParams.toString();
    const path = `/api/timetable/instances${search ? `?${search}` : ""}`;
    const response = await apiGet<{ data: unknown }>(path, token);
    return response.data;
  },
);

export const POST = createPostHandler(
  async (_request: NextRequest, body: unknown, token: string) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/timetable/instances",
      token,
      body ?? {},
    );
    return response.data;
  },
);
