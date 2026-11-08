import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: "/api/enrollment/phases",
});

export const POST = createTenantJsonProxy({
  method: "POST",
  path: "/api/enrollment/phases",
});
