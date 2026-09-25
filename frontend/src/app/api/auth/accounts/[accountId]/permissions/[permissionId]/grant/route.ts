import { NextResponse } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createTenantApiAdapter } from "~/lib/backend-proxy-route.server";

export const POST = createTenantApiAdapter(async (_request, token, context) => {
  const params = await context?.params;
  const accountId = params?.accountId;
  const permissionId = params?.permissionId;
  if (
    typeof accountId !== "string" ||
    !accountId ||
    typeof permissionId !== "string" ||
    !permissionId
  ) {
    return NextResponse.json(
      { error: "Account ID and Permission ID are required" },
      { status: 400 },
    );
  }
  await apiPost(
    `/auth/accounts/${accountId}/permissions/${permissionId}/grant`,
    token,
    {},
  );
  return NextResponse.json({ success: true });
});
