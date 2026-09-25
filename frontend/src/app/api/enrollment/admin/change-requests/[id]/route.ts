import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: (_request, params) =>
    `/api/enrollment/admin/change-requests/${encodeURIComponent(String(params.id))}`,
  cache: "no-store",
  contentTypeOnGet: false,
  unauthorizedError: "Unauthenticated",
});
