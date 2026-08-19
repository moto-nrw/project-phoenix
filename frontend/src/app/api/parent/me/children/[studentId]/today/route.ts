import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy GET /api/parent/me/children/{studentId}/today → backend.
 * Liefert den Tagesstatus des Kindes: `at_ogs` als binaere Aussage plus den
 * erklaerenden `state` mit den zugehoerigen Uhrzeiten.
 */
export const GET = proxyGet<unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/today`,
);
