import "server-only";
import { setTenantSession, withTenantSessionResponse } from "~/server/auth";
import { createLoginRoute } from "./login-route.server";

export function createTenantLoginRoute(
  path:
    | "/auth/login"
    | "/auth/mfa/verify"
    | "/auth/mfa/enroll/confirm"
    | "/auth/passkeys/login/verify",
) {
  return createLoginRoute(path, {
    withResponse: withTenantSessionResponse,
    setSession: setTenantSession,
    challenges: path === "/auth/login",
  });
}
