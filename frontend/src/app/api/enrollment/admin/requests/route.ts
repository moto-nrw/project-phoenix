import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: "/api/enrollment/admin/requests",
  forwardQuery: ["phase_id", "child_status"],
});
