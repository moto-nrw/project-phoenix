import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: "/api/enrollment/schema/versions",
  cache: "no-store",
  contentTypeOnGet: false,
  unauthorizedError: "Unauthenticated",
});
