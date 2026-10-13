import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";

const path = (
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) => {
  const accountId = params.accountId;
  if (typeof accountId !== "string" || !accountId) {
    return problemResponse(400, "Invalid accountId parameter", _request);
  }
  return `/auth/accounts/${encodeURIComponent(accountId)}/caregiver-capability`;
};

export const GET = createTenantJsonProxy({
  method: "GET",
  path,
  cache: "no-store",
  explicitGetMethod: true,
  retryOn401: true,
});
export const POST = createTenantJsonProxy({
  method: "POST",
  path,
  cache: "no-store",
  retryOn401: true,
});
export const DELETE = createTenantJsonProxy({
  method: "DELETE",
  path,
  cache: "no-store",
  retryOn401: true,
  includeEmptyBody: true,
});
