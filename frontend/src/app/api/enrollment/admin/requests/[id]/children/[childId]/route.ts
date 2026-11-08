import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const DELETE = createTenantJsonProxy({
  method: "DELETE",
  path: (_request, params) =>
    `/api/enrollment/admin/requests/${encodeURIComponent(String(params.id))}/children/${encodeURIComponent(String(params.childId))}`,
  body: "json",
});
