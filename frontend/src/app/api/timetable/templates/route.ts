// app/api/timetable/templates/route.ts
//
// POST /api/timetable/templates — proxies the smart template-creation
// endpoint (templates_create.go). The backend bundles timeframe +
// activities.groups + activities.schedules into one transaction and can
// optionally materialize the visible week.
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

export const POST = createPostHandler(
  async (_request: NextRequest, body: unknown, token: string) => {
    return await apiPost("/api/timetable/templates", token, body ?? {});
  },
);
