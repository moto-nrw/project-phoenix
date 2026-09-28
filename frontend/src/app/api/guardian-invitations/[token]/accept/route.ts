import { captureBffException } from "~/lib/sentry-bff.server";
import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { analyticsSessionHeaders } from "~/lib/analytics-session-header.server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";

const logger = createLogger({ component: "GuardianInvitationAcceptRoute" });

interface RouteContext {
  params: Promise<{ token: string }>;
}

interface AcceptGuardianInvitationBody {
  password: string;
  confirmPassword: string;
}

export async function POST(request: NextRequest, context: RouteContext) {
  const { token } = await context.params;
  if (!token) {
    return NextResponse.json(
      { error: "Missing invitation token" },
      { status: 400 },
    );
  }

  try {
    let body: AcceptGuardianInvitationBody;
    try {
      body = (await request.json()) as AcceptGuardianInvitationBody;
    } catch {
      return NextResponse.json({ error: "Invalid JSON" }, { status: 400 });
    }
    const payload = {
      password: body.password,
      confirm_password: body.confirmPassword,
    };

    const response = await fetch(
      `${getServerApiUrl()}/auth/guardian-invitations/${encodeURIComponent(token)}/accept`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...analyticsSessionHeaders(request.headers),
        },
        body: JSON.stringify(payload),
      },
    );

    return forwardBackendResponse(response);
  } catch (error) {
    captureBffException(error, request);
    logger.error("guardian_invitation_accept_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
