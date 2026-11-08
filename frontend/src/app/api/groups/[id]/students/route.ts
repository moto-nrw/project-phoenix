import { NextResponse } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = createGetHandler(async (_request, token, params) => {
  const response = await apiGet(
    `/api/groups/${requirePathSegmentParam(params)}/students`,
    token,
  );
  // Keep this endpoint's established bare-array success shape.
  if (response && typeof response === "object" && "data" in response) {
    return NextResponse.json(response.data);
  }
  return NextResponse.json(response);
});
