import type { NextRequest } from "next/server";

import { apiGet } from "~/lib/api-helpers.server";
import { proxyPut } from "~/lib/route-proxy.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

function backendPath(params: Record<string, unknown>): string {
  const typeId = requirePathSegmentParam(params, "absenceTypeId");
  const staffId = requirePathSegmentParam(params, "staffId");
  return `/api/absence-types/${typeId}/allowances/${staffId}`;
}

export const GET = createGetHandler(
  async (
    request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const year = request.nextUrl.searchParams.get("year");
    if (!year) throw new Error("year is required");
    const response = await apiGet<{ data: unknown }>(
      `${backendPath(params)}?year=${encodeURIComponent(year)}`,
      token,
    );
    return response.data;
  },
);

export const PUT = proxyPut<unknown, unknown>(backendPath);
