import { type NextRequest, NextResponse } from "next/server";
import { env } from "~/env";

interface PasswordResetRequestBody {
  email: string;
}

interface PasswordResetResponseData {
  message: string;
}

interface ErrorResponse {
  error: string;
}

export async function POST(request: NextRequest) {
  try {
    const body = (await request.json()) as PasswordResetRequestBody;

    const response = await fetch(
      `${env.NEXT_PUBLIC_API_URL}/auth/password-reset`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      },
    );

    if (!response.ok) {
      const retryAfter = response.headers.get("Retry-After");
      let message = "Fehler beim Senden der Passwort-Zurücksetzen-E-Mail";

      try {
        const contentType = response.headers.get("Content-Type") ?? "";
        if (contentType.includes("application/json")) {
          const payload = (await response.json()) as Partial<ErrorResponse> & {
            message?: string;
          };
          message = payload.error ?? payload.message ?? message;
        } else {
          const text = (await response.text()).trim();
          if (text) {
            message = text;
          }
        }
      } catch (parseError) {
        console.warn(
          "Failed to parse password reset error response",
          parseError,
        );
      }

      const nextResponse = NextResponse.json(
        { error: message } as ErrorResponse,
        { status: response.status },
      );
      if (retryAfter) {
        nextResponse.headers.set("Retry-After", retryAfter);
      }
      return nextResponse;
    }

    const data = (await response.json()) as PasswordResetResponseData;
    return NextResponse.json(data);
  } catch (error) {
    console.error("Password reset route error:", error);
    return NextResponse.json(
      { error: "Internal Server Error" } as ErrorResponse,
      { status: 500 },
    );
  }
}
