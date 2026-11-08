import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

const path = (
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) => `/api/enrollment/admin/requests/${encodeURIComponent(String(params.id))}`;

export const GET = createTenantJsonProxy({
  method: "GET",
  path,
});
export const DELETE = createTenantJsonProxy({
  method: "DELETE",
  path: (request, params) => `${path(request, params)}/`,
  body: "json",
});
