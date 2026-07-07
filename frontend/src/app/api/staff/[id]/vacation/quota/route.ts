import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import { proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = createGetHandler(
  async (
    request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const year = request.nextUrl.searchParams.get("year") ?? "";
    const qs = year ? `?year=${year}` : "";
    const response = await apiGet<{ data: unknown }>(
      `/api/staff/${id}/vacation/quota${qs}`,
      token,
    );
    return response.data;
  },
);

export const PUT = proxyPut(
  (p) => `/api/staff/${requirePathSegmentParam(p)}/vacation/quota`,
);
