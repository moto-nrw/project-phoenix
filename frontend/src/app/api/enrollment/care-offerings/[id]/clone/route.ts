import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

export const POST = createTenantJsonProxy({
  method: "POST",
  path: (_request, params) =>
    `/api/enrollment/care-offerings/${encodeURIComponent(String(params.id))}/clone`,
  unauthorizedError: "Unauthenticated",
});
