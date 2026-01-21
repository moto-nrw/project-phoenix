import { NextResponse, type NextRequest } from "next/server";
import { auth, getCookieHeader } from "~/server/auth";
import { apiPost } from "~/lib/api-helpers";
import { isAxiosError } from "axios";

interface ErrorResponse {
  message?: string;
  error?: string;
}

export async function POST(request: NextRequest) {
  try {
    const session = await auth();

    if (!session?.user) {
      return NextResponse.json(
        { error: "Nicht authentifiziert" },
        { status: 401 },
      );
    }

    const cookieHeader = await getCookieHeader();

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
    await apiPost("/auth/password", cookieHeader, {
      current_password: currentPassword,
      new_password: newPassword,
      confirm_password: confirmPassword,
    });

    return NextResponse.json({ success: true });
  } catch (error) {
    console.error("Password change error:", error);

    // Handle specific error messages from backend
    if (isAxiosError<ErrorResponse>(error)) {
      if (error.response?.data) {
        const errorMessage =
          error.response.data.message ?? error.response.data.error;
        const statusCode = error.response.status ?? 400;

        return NextResponse.json(
          { error: errorMessage ?? "Passwortänderung fehlgeschlagen" },
          { status: statusCode },
        );
      }
    }

    return NextResponse.json(
      { error: "Passwortänderung fehlgeschlagen" },
      { status: 500 },
    );
  }
}
