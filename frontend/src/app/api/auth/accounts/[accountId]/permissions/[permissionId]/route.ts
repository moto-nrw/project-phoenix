import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth, getCookieHeader } from "~/server/auth";
import { apiDelete } from "@/lib/api-client";
import { handleApiError } from "@/lib/api-helpers";

export async function DELETE(
  request: NextRequest,
  context: { params: Promise<Record<string, string | string[] | undefined>> },
) {
  try {
    const session = await auth();

    if (!session?.user) {
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    const cookieHeader = await getCookieHeader();

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

    // Make the API call to remove permission from account
    await apiDelete(
      `/auth/accounts/${accountId}/permissions/${permissionId}`,
      cookieHeader,
    );

    return NextResponse.json({ success: true });
  } catch (error) {
    return handleApiError(error);
  }
}
