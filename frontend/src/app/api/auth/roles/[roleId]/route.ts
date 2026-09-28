import { NextResponse } from "next/server";
import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

function rolePath(
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) {
  const roleId = params.roleId;
  if (typeof roleId !== "string" || !roleId) {
    return NextResponse.json({ error: "Role ID is required" }, { status: 400 });
  }
  return `/auth/roles/${encodeURIComponent(roleId)}`;
}

export const GET = createTenantJsonProxy({ method: "GET", path: rolePath });
export const PUT = createTenantJsonProxy({
  method: "PUT",
  path: rolePath,
  onSuccess: () => NextResponse.json({ success: true }),
});
export const DELETE = createTenantJsonProxy({
  method: "DELETE",
  path: rolePath,
  onSuccess: () => NextResponse.json({ success: true }),
});
