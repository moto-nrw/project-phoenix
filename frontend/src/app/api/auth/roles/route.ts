import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: "/auth/roles",
  forwardQuery: true,
});

export const POST = createTenantJsonProxy({
  method: "POST",
  path: "/auth/roles",
});
