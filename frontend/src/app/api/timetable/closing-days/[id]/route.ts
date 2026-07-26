// app/api/timetable/closing-days/[id]/route.ts
//
// PUT    /api/timetable/closing-days/{id} — update a closing day range
// DELETE /api/timetable/closing-days/{id} — remove a closing day range
//
// Both strip the Go envelope ({ status, data, message }) so route-wrapper
// produces a single envelope on the wire.
import { proxyDelete, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const PUT = proxyPut(
  (params) => `/api/timetable/closing-days/${requirePathSegmentParam(params)}`,
);

export const DELETE = proxyDelete(
  (params) => `/api/timetable/closing-days/${requirePathSegmentParam(params)}`,
);
