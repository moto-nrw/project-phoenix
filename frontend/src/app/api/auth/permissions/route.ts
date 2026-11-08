import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: "/auth/permissions",
  forwardQuery: ["resource", "action"],
});

export const POST = createTenantJsonProxy({
  method: "POST",
  path: "/auth/permissions",
});
