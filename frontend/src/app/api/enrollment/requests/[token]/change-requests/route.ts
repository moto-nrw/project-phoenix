import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";

const path = (
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) => {
  const token = params.token;
  if (typeof token !== "string" || !token) {
    return problemResponse(400, "token required", _request);
  }
  return `/api/enrollment/requests/${encodeURIComponent(token)}/change-requests`;
};

export const GET = createPublicJsonProxy({
  method: "GET",
  path,
  cache: "no-store",
  contentTypeOnGet: false,
});
export const POST = createPublicJsonProxy({
  method: "POST",
  path,
});
