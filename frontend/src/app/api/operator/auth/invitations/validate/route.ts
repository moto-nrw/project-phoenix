import { type NextRequest, NextResponse } from "next/server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({
  component: "OperatorInvitationValidateRoute",
});

/**
 * POST /api/operator/auth/invitations/validate
 * Public endpoint (no auth required) — proxies to backend POST /operator/auth/invitations/validate.
 * Token is sent in the request body, never in the URL.
 */
export async function POST(request: NextRequest) {
  let body: unknown;
  try {
    body = await request.json();
  } catch {
    return NextResponse.json({ message: "Ungültige Anfrage" }, { status: 400 });
  }

  try {
    const { getServerApiUrl } = await import("~/lib/server-api-url");
    const { getClientForwardHeaders } = await import("~/lib/client-headers");
    const response = await fetch(
      `${getServerApiUrl()}/operator/auth/invitations/validate`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...getClientForwardHeaders(request),
        },
        body: JSON.stringify(body),
      },
    );

    const contentType = response.headers.get("content-type");
    if (!contentType?.includes("application/json")) {
      const message = (await response.text()) || response.statusText;
      return NextResponse.json({ message }, { status: response.status });
    }

    const data: unknown = await response.json();
    return NextResponse.json(data, { status: response.status });
  } catch (error) {
    logger.error("operator_invitation_validate_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { message: "Ein interner Fehler ist aufgetreten" },
      { status: 500 },
    );
  }
}
