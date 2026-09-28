import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: "/api/enrollment/phases",
  cache: "no-store",
  contentTypeOnGet: false,
  unauthorizedError: "Unauthenticated",
});

export const POST = createTenantJsonProxy({
  method: "POST",
  path: "/api/enrollment/phases",
  unauthorizedError: "Unauthenticated",
});
