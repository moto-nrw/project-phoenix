// app/api/timetable/periods/[id]/route.ts
//
// GET    /api/timetable/periods/{id} — fetch a single calendar period
// PUT    /api/timetable/periods/{id} — update name/dates/active flag
// DELETE /api/timetable/periods/{id} — remove a calendar period
//
// All three strip the Go envelope ({ status, data, message }) so
// route-wrapper produces a single envelope on the wire.
import { proxyDelete, proxyGet, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (params) => `/api/timetable/periods/${requirePathSegmentParam(params)}`,
);

export const PUT = proxyPut(
  (params) => `/api/timetable/periods/${requirePathSegmentParam(params)}`,
);

export const DELETE = proxyDelete(
  (params) => `/api/timetable/periods/${requirePathSegmentParam(params)}`,
);
