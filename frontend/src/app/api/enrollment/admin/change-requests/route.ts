import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: "/api/enrollment/admin/change-requests",
  forwardQuery: ["request_id", "status"],
});
