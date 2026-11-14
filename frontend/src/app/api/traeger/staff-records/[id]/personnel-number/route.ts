import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPut } from "~/lib/traeger/route-wrapper.server";

// Die Personalnummer des Trägers einer Akte setzen (#4705).
export const PUT = proxyPut(
  (params) =>
    `/traeger/staff-records/${requirePathSegmentParam(params)}/personnel-number`,
);
