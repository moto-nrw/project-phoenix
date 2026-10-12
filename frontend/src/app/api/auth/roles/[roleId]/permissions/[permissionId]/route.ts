import { NextResponse } from "next/server";
import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";

function rolePermissionPath(
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) {
  const { roleId, permissionId } = params;
  if (
    typeof roleId !== "string" ||
    !roleId ||
    typeof permissionId !== "string" ||
    !permissionId
  ) {
    return problemResponse(400, "Role ID and Permission ID are required");
  }
  return `/auth/roles/${encodeURIComponent(roleId)}/permissions/${encodeURIComponent(permissionId)}`;
}

export const POST = createTenantJsonProxy({
  method: "POST",
  path: rolePermissionPath,
  body: "none",
  onSuccess: () => NextResponse.json({ success: true }),
});
export const DELETE = createTenantJsonProxy({
  method: "DELETE",
  path: rolePermissionPath,
  onSuccess: () => NextResponse.json({ success: true }),
});
