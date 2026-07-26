import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { apiPost, handleApiError } from "~/lib/api-helpers.server";

async function POSTHandler(
  request: NextRequest,
  context: { params: Promise<Record<string, string | string[] | undefined>> },
) {
  try {
    const session = await auth();

    if (!session?.user?.token) {
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    // Extract parameters from context
    const params = await context.params;
    const accountId = params.accountId as string;
    const permissionId = params.permissionId as string;

    if (!accountId || !permissionId) {
      return NextResponse.json(
        { error: "Account ID and Permission ID are required" },
        { status: 400 },
      );
    }

    // Make the API call to grant permission to account
    await apiPost(
      `/auth/accounts/${accountId}/permissions/${permissionId}/grant`,
      session.user.token,
      {},
    );

    return NextResponse.json({ success: true });
  } catch (error) {
    return handleApiError(error);
  }
}

export const POST = withTenantAuth(POSTHandler);
