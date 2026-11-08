import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

const path = (
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) =>
  `/api/enrollment/admin/requests/${encodeURIComponent(String(params.id))}/form-labels`;

export const GET = createTenantJsonProxy({
  method: "GET",
  path,
});
