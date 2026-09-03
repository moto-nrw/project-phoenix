import { proxyGet, proxyPut } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

const endpoint = (params: Record<string, unknown>) =>
  `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/meal-participation`;

export const GET = proxyGet(endpoint);
export const PUT = proxyPut(endpoint);
