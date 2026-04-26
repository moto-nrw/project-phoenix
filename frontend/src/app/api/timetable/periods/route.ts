// app/api/timetable/periods/route.ts
//
// GET  /api/timetable/periods       — list all calendar periods for the tenant
// POST /api/timetable/periods       — create a new calendar period
//
// The Go backend (api/timetable/api.go listPeriods/createPeriod) wraps
// responses in { status, data, message }. Strip that envelope here so
// route-wrapper does not double-wrap the payload.
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";

export const GET = createGetHandler(
  async (_request: NextRequest, token: string) => {
    const response = await apiGet<{ data: unknown }>(
      "/api/timetable/periods",
      token,
    );
    return response.data;
  },
);

export const POST = createPostHandler(
  async (_request: NextRequest, body: unknown, token: string) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/timetable/periods",
      token,
      body ?? {},
    );
    return response.data;
  },
);
