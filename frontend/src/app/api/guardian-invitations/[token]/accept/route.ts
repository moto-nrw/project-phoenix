import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

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
    const body = (await request.json()) as AcceptGuardianInvitationBody;
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
        },
        body: JSON.stringify(payload),
      },
    );

    const contentType = response.headers.get("Content-Type") ?? "";
    let payloadBody: unknown = null;
    if (contentType.includes("application/json")) {
      payloadBody = await response.json();
    } else {
      const text = await response.text();
      payloadBody = text ? { error: text } : null;
    }

    return NextResponse.json(payloadBody ?? {}, { status: response.status });
  } catch (error) {
    logger.error("guardian_invitation_accept_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
