import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: (_request, params) =>
    `/api/enrollment/phases/${encodeURIComponent(String(params.id))}/review`,
  cache: "no-store",
  contentTypeOnGet: false,
  unauthorizedError: "Unauthenticated",
});
