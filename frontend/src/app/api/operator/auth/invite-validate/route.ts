import { type NextRequest, NextResponse } from "next/server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorInviteValidateRoute" });

/**
 * POST /api/operator/auth/invite-validate
 * Public endpoint (no auth required) — proxies to backend POST /operator/auth/invite-validate.
 * Token is sent in the request body to prevent leaking into server access logs.
 */
export async function POST(request: NextRequest) {
  let body: { token?: string };
  try {
    body = (await request.json()) as { token?: string };
  } catch {
    return NextResponse.json(
      { message: "Ungültiger Request-Body" },
      { status: 400 },
    );
  }

  const token = body.token?.trim();
  if (!token) {
    return NextResponse.json(
      { message: "Token ist erforderlich" },
      { status: 400 },
    );
  }

  try {
    const { getServerApiUrl } = await import("~/lib/server-api-url");
    const { getClientForwardHeaders } = await import("~/lib/client-headers");
    const response = await fetch(
      `${getServerApiUrl()}/operator/auth/invite-validate`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...getClientForwardHeaders(request),
        },
        body: JSON.stringify({ token }),
      },
    );

    const contentType = response.headers.get("content-type");
    if (!contentType?.includes("application/json")) {
      const message = (await response.text()) || response.statusText;
      return NextResponse.json({ message }, { status: response.status });
    }

    const data = await response.json();
    return NextResponse.json(data, { status: response.status });
  } catch (error) {
    logger.error("operator_invite_validate_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { message: "Ein interner Fehler ist aufgetreten" },
      { status: 500 },
    );
  }
}
