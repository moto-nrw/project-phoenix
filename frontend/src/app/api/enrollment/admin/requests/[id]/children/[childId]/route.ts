import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

export const DELETE = createTenantJsonProxy({
  method: "DELETE",
  path: (_request, params) =>
    `/api/enrollment/admin/requests/${encodeURIComponent(String(params.id))}/children/${encodeURIComponent(String(params.childId))}`,
  body: "json",
  unauthorizedError: "Unauthenticated",
});
