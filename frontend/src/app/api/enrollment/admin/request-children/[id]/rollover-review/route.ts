import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

export const POST = createTenantJsonProxy({
  method: "POST",
  path: (_request, params) =>
    `/api/enrollment/admin/request-children/${encodeURIComponent(String(params.id))}/rollover-review`,
  unauthorizedError: "Unauthenticated",
});
