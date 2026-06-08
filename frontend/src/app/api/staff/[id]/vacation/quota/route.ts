import type { NextRequest } from "next/server";
import { apiGet, apiPut } from "~/lib/api-helpers.server";
import { createGetHandler, createPutHandler } from "~/lib/route-wrapper.server";

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

interface QuotaBody {
  year?: number;
  entitled_days: number;
  carryover_days?: number;
}

export const PUT = createPutHandler<unknown, QuotaBody>(
  async (
    _request: NextRequest,
    body: QuotaBody,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const response = await apiPut<{ data: unknown }>(
      `/api/staff/${id}/vacation/quota`,
      token,
      body,
    );
    return response.data;
  },
);
