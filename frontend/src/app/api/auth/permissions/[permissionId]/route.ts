import { NextResponse } from "next/server";
import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

function permissionPath(
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) {
  const permissionId = params.permissionId;
  if (typeof permissionId !== "string" || !permissionId) {
    return NextResponse.json(
      { error: "Permission ID is required" },
      { status: 400 },
    );
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
