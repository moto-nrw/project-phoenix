import type { NextRequest } from "next/server";
import {
  createOperatorGetHandler,
  operatorApiGet,
} from "~/lib/operator/route-wrapper";

export const GET = createOperatorGetHandler(
  async (request: NextRequest, token: string) => {
    const params = new URLSearchParams();
    const status = request.nextUrl.searchParams.get("status");
    const search = request.nextUrl.searchParams.get("search");
    const sort = request.nextUrl.searchParams.get("sort");
    if (status) params.set("status", status);
    if (search) params.set("search", search);
    if (sort) params.set("sort", sort);
    const qs = params.toString();
    return await operatorApiGet(
      `/operator/suggestions${qs ? `?${qs}` : ""}`,
      token,
    );
  },
);
