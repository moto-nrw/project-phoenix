// app/api/timetable/instances/route.ts
//
// GET /api/timetable/instances?from=YYYY-MM-DD&to=YYYY-MM-DD — proxies the
// weekly instance list from the backend (WP-F2 prerequisite endpoint added
// in instances_list.go). Forwards from/to query params verbatim; the
// backend validates them.
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const search = request.nextUrl.searchParams.toString();
    const path = `/api/timetable/instances${search ? `?${search}` : ""}`;
    return await apiGet(path, token);
  },
);
