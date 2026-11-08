import { proxyDelete, proxyPut } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

const endpoint = (params: Record<string, unknown>) =>
  `/api/timetable/planning-tracks/${requirePathSegmentParam(params)}`;

export const PUT = proxyPut(endpoint);
export const DELETE = proxyDelete(endpoint);
