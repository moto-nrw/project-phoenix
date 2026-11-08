import { NextResponse } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = createGetHandler(async (_request, token, params) => {
  const response = await apiGet<{ data: unknown }>(
    `/api/feedback/student/${requirePathSegmentParam(params)}`,
    token,
  );
  return NextResponse.json(
    { status: "success", data: response.data },
    { headers: { "Cache-Control": "no-store" } },
  );
});
