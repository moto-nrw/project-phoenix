import { captureBffException } from "~/lib/sentry-bff.server";
import type { NextRequest } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";
import { withInvitationOwnerSession } from "~/lib/invitation-owner-session.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-helpers.server";

const logger = createLogger({ component: "InvitationAcceptRoute" });

interface AcceptInvitationBody {
  existingAccount?: boolean;
  token?: string;
  firstName?: string;
  lastName?: string;
  password: string;
  confirmPassword: string;
}

export async function POST(request: NextRequest) {
  try {
    // NextAuth reconstructs the original request when resolving the owner session.
    let body: AcceptInvitationBody;
    try {
      body = (await request.clone().json()) as AcceptInvitationBody;
    } catch {
      return problemResponse(400, "Invalid JSON", request);
    }
    if (!body.token) {
      return problemResponse(400, "Missing invitation token", request);
    }

    const { token, ...rest } = body;
    const payload = {
      first_name: rest.firstName,
      last_name: rest.lastName,
      password: rest.password,
      confirm_password: rest.confirmPassword,
    };

    const forward = async (ownerToken?: string) => {
      const response = await fetch(
        `${getServerApiUrl()}/auth/invitations/${encodeURIComponent(token)}/accept`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            ...(ownerToken ? { Authorization: `Bearer ${ownerToken}` } : {}),
          },
          body: JSON.stringify(payload),
        },
      );

      return forwardBackendResponse(response, request);
    };
    return body.existingAccount
      ? await withInvitationOwnerSession(request, forward)
      : await forward();
  } catch (error) {
    captureBffException(error, request);
    logger.error("invitation accept failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "Internal Server Error", request);
  }
}
