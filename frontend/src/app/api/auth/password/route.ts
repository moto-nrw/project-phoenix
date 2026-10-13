import { NextResponse, type NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import {
  apiPost,
  handleApiError,
  problemResponse,
} from "~/lib/api-helpers.server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AuthPasswordRoute" });

async function POSTHandler(request: NextRequest) {
  try {
    const session = await auth();

    if (!session?.user?.token) {
      return problemResponse(401, "Unauthenticated", request);
    }

    let body: {
      currentPassword?: string;
      newPassword?: string;
      confirmPassword?: string;
    };
    try {
      body = (await request.json()) as typeof body;
    } catch {
      return problemResponse(400, "Invalid JSON", request);
    }
    const { currentPassword, newPassword, confirmPassword } = body;

    if (!currentPassword || !newPassword || !confirmPassword) {
      return problemResponse(400, "all password fields are required", request);
    }

    if (newPassword !== confirmPassword) {
      return problemResponse(400, "new passwords do not match", request);
    }

    await apiPost("/auth/password", session.user.token, {
      current_password: currentPassword,
      new_password: newPassword,
      confirm_password: confirmPassword,
    });

    return NextResponse.json({ success: true });
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error);
    logger.error("password change failed", { error: errorMessage });

    return handleApiError(error, request);
  }
}

export const POST = withTenantAuth(POSTHandler);
