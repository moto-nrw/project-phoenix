import "server-only";
import NextAuth from "next-auth";
import { createPortalAuthConfig } from "./portal-config";
import type { PortalDescriptor } from "./portals";
import { createResponseAwareAuth } from "./route-handler";

/**
 * One portal's Auth.js instance and its response-aware helpers. Called once
 * per portal entry module (index.ts, parent.ts, …), so every portal keeps
 * its own Auth.js instance and its own request context; nothing is cached
 * across portals.
 *
 * Auth.js' server helpers write Next's cookie jar. They never leave this
 * function except through mutateSession, which hands their cookies to the
 * session cookie exit (#5399).
 */
export function createPortalAuth(portal: PortalDescriptor) {
  const nextAuth = NextAuth(createPortalAuthConfig(portal));
  const { mutateSession, ...responseAware } = createResponseAwareAuth(
    nextAuth.auth,
    portal.key,
  );

  /** Installs a backend-minted token pair as this portal's session. */
  const setSession = async (
    token: string,
    refreshToken: string,
  ): Promise<void> => {
    await mutateSession(
      () =>
        nextAuth.signIn(portal.providerId, {
          redirect: false,
          internalRefresh: "true",
          token,
          refreshToken,
        }),
      // Refused Credentials throw. The callback URL is not an error payload.
      (result) => typeof result === "string",
    );
  };

  /** Runs the JWT callback's update trigger with a server-side payload. */
  const updateSession = <T>(
    data: Record<string, unknown>,
    accepts: (
      session: Awaited<ReturnType<typeof nextAuth.unstable_update>>,
    ) => T,
  ) =>
    mutateSession(
      () =>
        nextAuth.unstable_update(
          data as Parameters<typeof nextAuth.unstable_update>[0],
        ),
      (session) => Boolean(accepts(session)),
    );

  return {
    ...responseAware,
    handlers: nextAuth.handlers,
    setSession,
    updateSession,
  };
}
