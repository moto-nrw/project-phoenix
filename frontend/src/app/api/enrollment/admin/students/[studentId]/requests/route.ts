import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: (_request, params) =>
    `/api/enrollment/admin/students/${encodeURIComponent(String(params.studentId))}/requests`,
});
