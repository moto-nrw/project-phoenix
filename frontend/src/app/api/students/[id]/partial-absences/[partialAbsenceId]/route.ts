import { proxyDelete, proxyPut } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const PUT = proxyPut(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/partial-absences/${requirePathSegmentParam(p, "partialAbsenceId")}`,
);

export const DELETE = proxyDelete(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/partial-absences/${requirePathSegmentParam(p, "partialAbsenceId")}`,
);
