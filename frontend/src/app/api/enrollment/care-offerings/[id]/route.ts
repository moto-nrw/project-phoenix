import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

const path = (
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) => `/api/enrollment/care-offerings/${encodeURIComponent(String(params.id))}`;

export const GET = createTenantJsonProxy({
  method: "GET",
  path,
  cache: "no-store",
  contentTypeOnGet: false,
  unauthorizedError: "Unauthenticated",
});
export const PUT = createTenantJsonProxy({
  method: "PUT",
  path,
  unauthorizedError: "Unauthenticated",
});
export const DELETE = createTenantJsonProxy({
  method: "DELETE",
  path,
  contentTypeOnDelete: false,
  unauthorizedError: "Unauthenticated",
});
