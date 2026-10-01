import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

export const POST = createTenantJsonProxy({
  method: "POST",
  path: (_request, params) =>
    `/api/enrollment/admin/change-requests/${encodeURIComponent(String(params.id))}/approve`,
  unauthorizedError: "Unauthenticated",
});
