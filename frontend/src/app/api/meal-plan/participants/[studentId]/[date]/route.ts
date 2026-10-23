// Küchenliste (#3164): das Team meldet ein Kind für einen Tag an oder ab
// (PUT) oder setzt den Tag auf die festen Tage zurück (DELETE). Die Frist
// gilt dafür nicht; config:update und users:read erzwingt das Backend.

import { proxyDelete, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const PUT = proxyPut(
  (p) =>
    `/api/meal-plan/participants/${requirePathSegmentParam(p, "studentId")}/${requirePathSegmentParam(p, "date")}`,
);

export const DELETE = proxyDelete(
  (p) =>
    `/api/meal-plan/participants/${requirePathSegmentParam(p, "studentId")}/${requirePathSegmentParam(p, "date")}`,
);
