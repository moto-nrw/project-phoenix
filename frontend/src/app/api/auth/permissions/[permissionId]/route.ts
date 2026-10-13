import { NextResponse } from "next/server";
import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";

function permissionPath(
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) {
  const permissionId = params.permissionId;
  if (typeof permissionId !== "string" || !permissionId) {
    return problemResponse(400, "Permission ID is required", _request);
  }
  return `/auth/permissions/${encodeURIComponent(permissionId)}`;
}

export const GET = createTenantJsonProxy({
  method: "GET",
  path: permissionPath,
});
export const PUT = createTenantJsonProxy({
  method: "PUT",
  path: permissionPath,
});
export const DELETE = createTenantJsonProxy({
  method: "DELETE",
  path: permissionPath,
  onSuccess: () => new NextResponse(null, { status: 204 }),
});
