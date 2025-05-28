import { NextResponse, type NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { apiPut } from "~/lib/api-helpers";
import type { AxiosError } from "axios";

interface ErrorResponse {
  message?: string;
  error?: string;
}

export async function PUT(request: NextRequest) {
  try {
    const session = await auth();
    
    if (!session?.user?.token) {
      return NextResponse.json(
        { error: "Nicht authentifiziert" },
        { status: 401 }
      );
    }

    const body = await request.json() as { currentPassword?: string; newPassword?: string };
    const { currentPassword, newPassword } = body;

    if (!currentPassword || !newPassword) {
      return NextResponse.json(
        { error: "Aktuelles und neues Passwort sind erforderlich" },
        { status: 400 }
      );
    }

    // Call backend API to change password
    await apiPut(
      "/api/auth/password",
      session.user.token,
      {
        current_password: currentPassword,
        new_password: newPassword,
      }
    );

    return NextResponse.json({ success: true });
  } catch (error) {
    console.error("Password change error:", error);
    
    // Handle specific error messages from backend
    if (error && typeof error === 'object' && 'response' in error) {
      const axiosError = error as AxiosError<ErrorResponse>;
      if (axiosError.response?.data) {
        const errorMessage = axiosError.response.data.message ?? axiosError.response.data.error;
        const statusCode = axiosError.response.status ?? 400;
        
        return NextResponse.json(
          { error: errorMessage ?? "Passwortänderung fehlgeschlagen" },
          { status: statusCode }
        );
      }
    }
    
    return NextResponse.json(
      { error: "Passwortänderung fehlgeschlagen" },
      { status: 500 }
    );
  }
}