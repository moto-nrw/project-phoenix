import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

const path = (
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) => `/api/enrollment/phases/${encodeURIComponent(String(params.id))}`;

export const GET = createTenantJsonProxy({
  method: "GET",
  path,
});
export const PUT = createTenantJsonProxy({
  method: "PUT",
  path,
});
export const DELETE = createTenantJsonProxy({
  method: "DELETE",
  path,
});
