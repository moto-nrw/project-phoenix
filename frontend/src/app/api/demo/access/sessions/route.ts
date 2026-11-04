import { captureBffException } from "~/lib/sentry-bff.server";
import { type NextRequest, NextResponse } from "next/server";
import {
  frontendOrigin,
  getClientForwardHeaders,
} from "~/lib/client-headers.server";
import { createLogger } from "~/lib/logger";
import {
  DEMO_TOKEN_COOKIE,
  DEMO_TOKEN_COOKIE_OPTIONS,
  demoBackendUrl,
} from "../forward";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-helpers.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

import { clientEnv } from "~/env.client";
import { setTenantSession, withTenantSessionResponse } from "~/server/auth";
import {
  setParentSession,
  withParentSessionResponse,
} from "~/server/auth/parent";
import { sessionMutationOriginError } from "~/server/auth/same-origin";
import {
  installLoginSession,
  revokeMintedSession,
} from "~/lib/login-route.server";

const logger = createLogger({ component: "DemoSessionRoute" });

// Redeems a demo token in a demo role (#3462, #3467). The entry page sends
// the token from the link; the route keeps it in an httpOnly cookie, and a
// later role switch from the banner sends only the role. The token is never
// logged and never returned.
async function POSTHandler(request: NextRequest) {
  const originError = sessionMutationOriginError(request);
  if (originError) return originError;
  // Choose the cookie context from the proxy-owned host, never from a role
  // supplied by the browser or returned for a fixed-role demo link.
  const host = new URL(frontendOrigin(request)).host;
  if (
    [
      clientEnv.NEXT_PUBLIC_OPERATOR_HOSTNAME,
      clientEnv.NEXT_PUBLIC_SCHOOL_HOSTNAME,
      clientEnv.NEXT_PUBLIC_TRAEGER_HOSTNAME,
      clientEnv.NEXT_PUBLIC_CATERER_HOSTNAME,
    ].some((authority) => new URL(`http://${authority}`).host === host)
  ) {
    return problemResponse(403, "Unsupported demo portal", request);
  }
  const parent =
    host === new URL(`http://${clientEnv.NEXT_PUBLIC_PARENTS_HOSTNAME}`).host;
  const withResponse = parent
    ? withParentSessionResponse
    : withTenantSessionResponse;
  return withResponse((req) => redeem(req, parent))(request);
}

async function redeem(request: NextRequest, parent: boolean) {
  let body: { token?: unknown; role?: unknown } = {};
  try {
    body = (await request.json()) as typeof body;
  } catch {
    // An unreadable body is a switch without a role.
  }
  const linkToken = typeof body.token === "string" ? body.token.trim() : "";
  const token = linkToken || request.cookies.get(DEMO_TOKEN_COOKIE)?.value;
  if (!token) {
    return problemResponse(404, "demo access is unknown", request);
  }
  const role = typeof body.role === "string" ? body.role : undefined;
  try {
    const backend = await fetch(demoBackendUrl("sessions"), {
      method: "POST",
      headers: {
        ...getClientForwardHeaders(request),
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ token, role }),
    });
    if (!backend.ok) return forwardBackendResponse(backend, request);
    const payload = (await backend.json().catch(() => {
      throw new Error("Invalid demo session response");
    })) as Record<string, unknown>;
    const demo = payload.demo as Record<string, unknown> | undefined;
    if (
      !demo ||
      !["lead", "caregiver", "parent", "all"].includes(String(demo.role))
    ) {
      throw new Error("Invalid demo role");
    }
    const matchesPortal = (demo.role === "parent") === parent;
    if (matchesPortal) {
      const failure = await installLoginSession(
        request,
        payload,
        parent ? setParentSession : setTenantSession,
      );
      if (failure) return failure;
    } else {
      // The entry page hands fixed-role links to the right portal. Revoke
      // this unused session family before returning only redirect metadata.
      if (typeof payload.refresh_token !== "string" || !payload.refresh_token)
        throw new Error("Missing demo session credentials");
      await revokeMintedSession(request, payload.refresh_token);
    }
    const response = NextResponse.json(
      {
        session_created: matchesPortal,
        demo: {
          access_id: demo.access_id,
          role: demo.role,
          src: demo.src,
          fixed_role: demo.fixed_role,
        },
      },
      { status: backend.status },
    );
    if (backend.ok && linkToken) {
      response.cookies.set(
        DEMO_TOKEN_COOKIE,
        linkToken,
        DEMO_TOKEN_COOKIE_OPTIONS,
      );
    }
    return response;
  } catch (error) {
    captureBffException(error, request);
    logger.error("demo_session_forward_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "Internal Server Error", request);
  }
}

export const POST = withPrivateNoStore(POSTHandler);
