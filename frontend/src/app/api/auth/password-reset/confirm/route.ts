import { type NextRequest, NextResponse } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "PasswordResetConfirmRoute" });

interface PasswordResetConfirmRequest {
  token: string;
  password: string;
}

interface PasswordResetConfirmResponse {
  message: string;
}

export async function POST(request: NextRequest) {
  try {
    const body = (await request.json()) as PasswordResetConfirmRequest;

    const response = await fetch(
      `${getServerApiUrl()}/auth/password-reset/confirm`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      },
    );

    if (!response.ok) {
      let message = "Fehler beim Zurücksetzen des Passworts";

      try {
        const contentType = response.headers.get("Content-Type") ?? "";
        if (contentType.includes("application/json")) {
          const payload = (await response.json()) as {
            error?: string;
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
        logger.warn("failed to parse password reset confirm error response", {
          error:
            parseError instanceof Error
              ? parseError.message
              : String(parseError),
        });
      }

      return NextResponse.json({ error: message }, { status: response.status });
    }

    const data = (await response.json()) as PasswordResetConfirmResponse;
    return NextResponse.json(data);
  } catch (error: unknown) {
    logger.error("password reset confirm failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
