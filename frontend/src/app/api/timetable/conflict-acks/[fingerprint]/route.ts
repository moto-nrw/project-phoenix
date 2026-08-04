// app/api/timetable/conflict-acks/[fingerprint]/route.ts
//
// PUT    /api/timetable/conflict-acks/{fingerprint} — hide one conflict
// DELETE /api/timetable/conflict-acks/{fingerprint} — show it again
//
// Both are idempotent on the backend and scoped to the calling user (#2139).
import { proxyDelete, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const PUT = proxyPut(
  (params) =>
    `/api/timetable/conflict-acks/${requirePathSegmentParam(params, "fingerprint")}`,
);

export const DELETE = proxyDelete(
  (params) =>
    `/api/timetable/conflict-acks/${requirePathSegmentParam(params, "fingerprint")}`,
);
