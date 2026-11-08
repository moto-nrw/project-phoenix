import type { NextRequest } from "next/server";
import { createPublicJsonProxy } from "./backend-proxy-route.server";

/**
 * A login step before a session exists (MFA resend, enrollment start,
 * passkey login options). The browser's Authorization header carries the
 * step's challenge or enrollment proof, not a session token (#5005).
 */
export function authStepProxy(path: string, body: "optional-json" | "none") {
  return createPublicJsonProxy({
    method: "POST",
    path,
    body,
    timeout: "login",
    additionalHeaders: (request: NextRequest): Record<string, string> => {
      const proof = request.headers.get("authorization");
      return proof ? { Authorization: proof } : {};
    },
  });
}
