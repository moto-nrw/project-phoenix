// app/api/timetable/templates/route.ts
//
// GET  /api/timetable/templates?period_id=...
//   Proxies the read-only template list for the planner "Vorlagen" view.
// POST /api/timetable/templates — proxies the smart template-creation
// endpoint (templates_create.go). The backend bundles timeframe +
// activities.groups + activities.schedules into one transaction and can
// optionally materialize the visible week.
//
// The Go backend returns { status, data, message }. We unwrap the Go
// envelope here so route-wrapper's wrapInApiResponse doesn't double-wrap
// the payload — clients (timetable-api.ts unwrap) expect a single
// envelope and read envelope.data as the real result.
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";

export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const search = request.nextUrl.searchParams.toString();
    const path = `/api/timetable/templates${search ? `?${search}` : ""}`;
    const response = await apiGet<{ data: unknown }>(path, token);
    return response.data;
  },
);

export const POST = createPostHandler(
  async (_request: NextRequest, body: unknown, token: string) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/timetable/templates",
      token,
      body ?? {},
    );
    return response.data;
  },
);
