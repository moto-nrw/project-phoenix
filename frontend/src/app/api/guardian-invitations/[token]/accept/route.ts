import { captureBffException } from "~/lib/sentry-bff.server";
import type { NextRequest } from "next/server";
import { clientIdentityHeaders } from "~/lib/client-headers.server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-helpers.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

const logger = createLogger({ component: "GuardianInvitationAcceptRoute" });

interface RouteContext {
  params: Promise<{ token: string }>;
}

interface AcceptGuardianInvitationBody {
  password: string;
  confirmPassword: string;
}

async function POSTHandler(request: NextRequest, context: RouteContext) {
  const { token } = await context.params;
  if (!token) {
    return problemResponse(400, "Missing invitation token", request);
  }

  try {
    let body: AcceptGuardianInvitationBody;
    try {
      body = (await request.json()) as AcceptGuardianInvitationBody;
    } catch {
      return problemResponse(400, "Invalid JSON", request);
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
          ...clientIdentityHeaders(request.headers),
          "Content-Type": "application/json",
        },
        body: JSON.stringify(payload),
      },
    );

    return forwardBackendResponse(response, request);
  } catch (error) {
    captureBffException(error, request);
    logger.error("guardian_invitation_accept_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "Internal Server Error", request);
  }
}

export const POST = withPrivateNoStore(POSTHandler);
