import { proxyDelete, proxyGet, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

type WorkTimeModel = Record<string, unknown>;
type UpdateBody = Record<string, unknown>;

export const GET = proxyGet<WorkTimeModel>(
  (params) => `/api/work-time-models/${requirePathSegmentParam(params)}`,
);

export const PUT = proxyPut<WorkTimeModel, UpdateBody>(
  (params) => `/api/work-time-models/${requirePathSegmentParam(params)}`,
);

export const DELETE = proxyDelete(
  (params) => `/api/work-time-models/${requirePathSegmentParam(params)}`,
);
