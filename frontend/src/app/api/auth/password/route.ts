import { NextResponse, type NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { apiPost } from "~/lib/api-helpers";
import { isAxiosError } from "axios";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AuthPasswordRoute" });

interface ErrorResponse {
  message?: string;
  error?: string;
}

export async function POST(request: NextRequest) {
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

    // Call backend API to change password
    await apiPost("/auth/password", session.user.token, {
      current_password: currentPassword,
      new_password: newPassword,
      confirm_password: confirmPassword,
    });

    return NextResponse.json({ success: true });
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error);
    logger.error("password change failed", { error: errorMessage });

    // serverFetchWithRetry throws Error("API error (status): body")
    // Parse the JSON body from the error message to extract backend error
    const statusMatch = errorMessage.match(/API error \((\d+)\):\s*(.*)/s);
    if (statusMatch) {
      const statusCode = parseInt(statusMatch[1] ?? "500", 10);
      try {
        const parsed = JSON.parse(statusMatch[2] ?? "{}") as ErrorResponse;
        const backendError = parsed.error ?? parsed.message;
        if (backendError) {
          return NextResponse.json(
            { error: backendError },
            { status: statusCode },
          );
        }
      } catch {
        // JSON parse failed — fall through to generic error
      }
    }

    // Handle Axios errors (client-side requests)
    if (isAxiosError<ErrorResponse>(error)) {
      if (error.response?.data) {
        const axiosError =
          error.response.data.message ?? error.response.data.error;
        return NextResponse.json(
          { error: axiosError ?? "Passwortänderung fehlgeschlagen" },
          { status: error.response.status ?? 400 },
        );
      }
    }

    return NextResponse.json(
      { error: "Passwortänderung fehlgeschlagen" },
      { status: 500 },
    );
  }
}
