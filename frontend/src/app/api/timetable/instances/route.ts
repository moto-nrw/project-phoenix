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

export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const search = request.nextUrl.searchParams.toString();
    const path = `/api/timetable/instances${search ? `?${search}` : ""}`;
    return await apiGet(path, token);
  },
);

export const POST = createPostHandler(
  async (_request: NextRequest, body: unknown, token: string) => {
    return await apiPost("/api/timetable/instances", token, body ?? {});
  },
);
