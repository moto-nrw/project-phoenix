import { captureBffException } from "~/lib/sentry-bff.server";
import { type NextRequest, NextResponse } from "next/server";
import { sendBackend } from "~/lib/backend-client.server";
import { frontendOrigin } from "~/lib/client-headers.server";
import { createLogger } from "~/lib/logger";
import {
  DEMO_TOKEN_COOKIE,
  DEMO_TOKEN_COOKIE_OPTIONS,
  demoBackendPath,
} from "../forward";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-helpers.server";
import { withPrivateNoStore } from "~/lib/private-no-store";
import { configuredPortalOfHost } from "~/lib/portal-paths";
import { tenantPortal } from "~/server/auth";
import { parentPortal } from "~/server/auth/parent";
import { sessionMutationOriginError } from "~/server/auth/same-origin";
import { installLoginSession } from "~/lib/login-route.server";
import {
  logoutRollback,
  mintedPair,
  revokeMintedSession,
} from "~/lib/session-transition.server";

const logger = createLogger({ component: "DemoSessionRoute" });

async function POSTHandler(request: NextRequest) {
  const originError = sessionMutationOriginError(request);
  if (originError) return originError;
  // Choose the cookie context from the proxy-owned host, never from a role
  // supplied by the browser or returned for a fixed-role demo link.
  const portal = configuredPortalOfHost(new URL(frontendOrigin(request)).host);
  if (portal !== "tenant" && portal !== "parent") {
    return problemResponse(403, "Unsupported demo portal", request);
  }
  const parent = portal === "parent";
  const session = parent ? parentPortal : tenantPortal;
  return session.withSessionResponse((req) => redeem(req, parent))(request);
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
    const backend = await sendBackend(demoBackendPath("sessions"), null, {
      method: "POST",
      json: { token, role },
      request,
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
    // The session belongs to the portal of its role, not of this host.
    const minted = demo.role === "parent" ? parentPortal : tenantPortal;
    const matchesPortal = (demo.role === "parent") === parent;
    if (matchesPortal) {
      const failure = await installLoginSession(request, payload, minted);
      if (failure) return failure;
    } else {
      // The entry page hands fixed-role links to the right portal. Revoke
      // this unused session family before returning only redirect metadata.
      await revokeMintedSession(
        request,
        logoutRollback(minted.portal.backend.logout, mintedPair(payload)),
      );
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
