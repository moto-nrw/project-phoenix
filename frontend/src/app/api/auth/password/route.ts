import { NextResponse, type NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { apiPost } from "~/lib/api-helpers.server";
import { isAxiosError } from "axios";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AuthPasswordRoute" });

interface ErrorResponse {
  message?: string;
  error?: string;
}

const API_ERROR_PATTERN = /API error \((\d+)\):\s*(.*)/s;

/**
 * Parse error from serverFetchWithRetry format: "API error (status): JSON body"
 */
function parseServerFetchError(
  message: string,
): { error: string; status: number } | null {
  const match = API_ERROR_PATTERN.exec(message);
  if (!match) return null;

  const statusCode = Number.parseInt(match[1] ?? "500", 10);
  try {
    const parsed = JSON.parse(match[2] ?? "{}") as ErrorResponse;
    const backendError = parsed.error ?? parsed.message;
    if (backendError) return { error: backendError, status: statusCode };
  } catch {
    // JSON parse failed
  }
  return null;
}

/**
 * Extract error from Axios error response
 */
function parseAxiosError(
  error: unknown,
): { error: string; status: number } | null {
  if (!isAxiosError<ErrorResponse>(error)) return null;
  if (!error.response?.data) return null;

  const message = error.response.data.message ?? error.response.data.error;
  if (!message) return null;
  return { error: message, status: error.response.status ?? 400 };
}

async function POSTHandler(request: NextRequest) {
  try {
    const session = await auth();

    if (!session?.user?.token) {
      return NextResponse.json(
        { error: "Nicht authentifiziert" },
        { status: 401 },
      );
    }

    const body = (await request.json()) as {
      currentPassword?: string;
      newPassword?: string;
      confirmPassword?: string;
    };
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

    const serverError = parseServerFetchError(errorMessage);
    if (serverError) {
      return NextResponse.json(
        { error: serverError.error },
        { status: serverError.status },
      );
    }

    const axiosError = parseAxiosError(error);
    if (axiosError) {
      return NextResponse.json(
        { error: axiosError.error },
        { status: axiosError.status },
      );
    }

    return NextResponse.json(
      { error: "Passwortänderung fehlgeschlagen" },
      { status: 500 },
    );
  }
}

export const POST = withTenantAuth(POSTHandler);
