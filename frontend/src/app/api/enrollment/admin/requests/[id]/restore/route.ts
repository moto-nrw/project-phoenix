import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const POST = createTenantJsonProxy({
  method: "POST",
  path: (_request, params) =>
    `/api/enrollment/admin/requests/${encodeURIComponent(String(params.id))}/restore`,
  body: "none",
});
