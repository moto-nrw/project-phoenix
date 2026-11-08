import "server-only";
import type { NextRequest } from "next/server";
import type { PortalDescriptor } from "~/server/auth/portals";
import { problemResponse } from "./api-response-error.server";
import { sendBackend } from "./backend-client.server";
import { createLogger } from "./logger";

const logger = createLogger({ component: "SessionTransition" });

const ROLLBACK_TIMEOUT_MS = 5000;

/** The backend call that revokes a freshly minted session. */
export interface Rollback {
  path: string;
  bearer?: string;
  json?: unknown;
}

/** A failed handoff never leaves its freshly minted family alive. */
export async function revokeMintedSession(
  request: NextRequest,
  rollback: Rollback,
): Promise<void> {
  try {
    const response = await sendBackend(rollback.path, rollback.bearer, {
      method: "POST",
      json: rollback.json,
      request,
      timeoutMs: ROLLBACK_TIMEOUT_MS,
    });
    if (!response.ok) {
      logger.error("session_rollback_failed", {
        path: rollback.path,
        status: response.status,
      });
    }
  } catch {
    logger.error("session_rollback_failed", { path: rollback.path });
  }
}

/**
 * Installs a backend-minted session through the portal's session writer.
 * Runs inside the portal's response wrapper, so the cookies leave through
 * mutateSession and then finishResponse, the write gate's order (#5399).
 * When installing fails, the minted family is revoked and the request
 * answers 500 without any of its credentials.
 */
export async function installMintedSession(
  request: NextRequest,
  install: () => Promise<void>,
  rollback: Rollback,
): Promise<Response | null> {
  try {
    await install();
    return null;
  } catch {
    await revokeMintedSession(request, rollback);
    return problemResponse(500, "Session creation failed", request);
  }
}

/** The revocation of a minted pair: the token kind the portal's logout takes. */
export function logoutRollback(
  logout: PortalDescriptor["backend"]["logout"],
  pair: { access_token: string; refresh_token: string },
): Rollback {
  const { path, token } = logout;
  return {
    path,
    bearer: token === "access" ? pair.access_token : pair.refresh_token,
  };
}

/** A backend answer's token pair; anything else is an invalid answer. */
export function mintedPair(data: Record<string, unknown>): {
  access_token: string;
  refresh_token: string;
} {
  const { access_token, refresh_token } = data;
  if (
    typeof access_token !== "string" ||
    !access_token ||
    typeof refresh_token !== "string" ||
    !refresh_token
  ) {
    throw new Error("Missing session credentials");
  }
  return { access_token, refresh_token };
}
