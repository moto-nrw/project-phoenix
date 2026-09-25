import { NextResponse, type NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { apiPost, handleApiError } from "~/lib/api-helpers.server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AuthPasswordRoute" });

async function POSTHandler(request: NextRequest) {
  try {
    const session = await auth();

    if (!session?.user?.token) {
      return NextResponse.json(
        { error: "Nicht authentifiziert" },
        { status: 401 },
      );
    }

    let body: {
      currentPassword?: string;
      newPassword?: string;
      confirmPassword?: string;
    };
    try {
      body = (await request.json()) as typeof body;
    } catch {
      return NextResponse.json({ error: "Invalid JSON" }, { status: 400 });
    }
    const { currentPassword, newPassword, confirmPassword } = body;

    if (!currentPassword || !newPassword || !confirmPassword) {
      return NextResponse.json(
        { error: "Alle Passwortfelder sind erforderlich" },
        { status: 400 },
      );
    }

    if (newPassword !== confirmPassword) {
      return NextResponse.json(
        { error: "Die neuen Passwörter stimmen nicht überein" },
        { status: 400 },
      );
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
