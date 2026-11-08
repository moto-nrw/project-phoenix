import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: "/api/enrollment/care-offerings",
  forwardQuery: ["phase_id"],
});

export const POST = createTenantJsonProxy({
  method: "POST",
  path: "/api/enrollment/care-offerings",
});
