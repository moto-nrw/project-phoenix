import { type NextRequest, NextResponse } from "next/server";
import { getClientForwardHeaders } from "~/lib/client-headers.server";
import { createLogger } from "~/lib/logger";
import { getServerApiUrl } from "~/lib/server-api-url";

const logger = createLogger({ component: "SchoolPasswordResetRoute" });

interface PasswordResetRequestBody {
  email: string;
}

interface PasswordResetResponseData {
  message: string;
}

interface ErrorResponse {
  error: string;
}

/**
 * Password reset from the school portal (#2207). School accounts are
 * regular auth.accounts, so this forwards to the shared account-level
 * reset endpoint — mirrored here because the proxy blocks /api/auth/* on
 * the school host. The Retry-After header is preserved: it drives the
 * live countdown in the password-reset modal (cross-layer contract).
 */
export async function POST(request: NextRequest) {
  try {
    const body = (await request.json()) as PasswordResetRequestBody;

    const response = await fetch(`${getServerApiUrl()}/auth/password-reset`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...getClientForwardHeaders(request),
      },
      body: JSON.stringify(body),
    });

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
        logger.warn("failed to parse school password reset error response", {
          error:
            parseError instanceof Error
              ? parseError.message
              : String(parseError),
        });
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
    logger.error("school password reset request failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" } as ErrorResponse,
      { status: 500 },
    );
  }
}
