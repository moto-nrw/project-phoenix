import { proxyDelete, proxyPut } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

const endpoint = (params: Record<string, unknown>) =>
  `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/meal-participation/${requirePathSegmentParam(params, "date")}`;

export const PUT = proxyPut(endpoint);
export const DELETE = proxyDelete(endpoint);
