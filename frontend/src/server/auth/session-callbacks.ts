import "server-only";
import type { NextAuthConfig, Session } from "next-auth";
import type { JWT } from "next-auth/jwt";
import {
  authLogger,
  parseJwtPayload,
  syncTokenFromPayload,
} from "./jwt-claims";
import type { PortalDescriptor } from "./portals";
import { isServerSessionMutation } from "./session-mutation";
import {
  ACCESS_TOKEN_REFRESH_BUFFER_MS,
  accessTokenExpiry,
  refreshTokenExpiry,
} from "./session-lifetime";
import type { AuthUser, ServerSession } from "./session-types";
import {
  endPreviewOnSessionLoss,
  isPreviewActive,
  isPreviewStartUpdate,
  maintainPreviewSession,
  restoreAdminFromPreview,
  applyPreviewStart,
} from "./staff-preview";
import {
  applyRotation,
  createRefreshRecoveryProof,
  isRenewalForced,
  refreshAlreadyRefused,
  rotateRefreshToken,
} from "./token-refresh";

type Callbacks = NonNullable<NextAuthConfig["callbacks"]>;
type JwtCallback = NonNullable<Callbacks["jwt"]>;
type JwtParams = Parameters<JwtCallback>[0];

function endSession(token: JWT, error: NonNullable<JWT["error"]>): JWT {
  token.error = error;
  token.needsRefresh = true;
  return token;
}

function signIn(token: JWT, user: AuthUser): void {
  token.id = user.id;
  token.name = user.name;
  token.email = user.email;
  token.token = user.token ?? "";
  token.refreshToken = user.refreshToken ?? "";
  token.roles = user.roles;
  token.roleIsSystem = user.roleIsSystem;
  token.permissions = user.permissions;
  token.firstName = user.firstName;
  token.isAdmin = user.isAdmin;
  token.tenantId = user.tenantId;
  token.orgId = user.orgId;
  token.scope = user.scope;
  token.viaTraeger = user.viaTraeger;
  const accessClaims = user.token ? parseJwtPayload(user.token) : null;
  token.tokenExpiry =
    typeof accessClaims?.exp === "number"
      ? accessClaims.exp * 1000
      : Date.now() + accessTokenExpiry;
  token.refreshTokenExpiry = Date.now() + refreshTokenExpiry;
  token.refreshRecoveryProof = createRefreshRecoveryProof();
  token.error = undefined;
  token.needsRefresh = undefined;
}

/** A browser update(): profile name or email change, preview end. */
function applyClientUpdate(token: JWT, update: Record<string, unknown>) {
  if (typeof update.name === "string") token.name = update.name;
  if (typeof update.email === "string") token.email = update.email;
  if (update.emailChanged === true) {
    // Renew on the next callback, so the refresh re-reads the confirmed
    // email. One minute inside the window: if the refresh fails, the access
    // token is still valid and the session stays.
    token.tokenExpiry = Date.now() + (ACCESS_TOKEN_REFRESH_BUFFER_MS - 60_000);
  }
  // Leave preview before maintenance so a stale restored admin can renew.
  if (update.previewEnd === true && isPreviewActive(token)) {
    restoreAdminFromPreview(token);
  }
}

async function maintainSession(
  portal: PortalDescriptor,
  { token, user, trigger, session }: JwtParams,
): Promise<JWT> {
  const preview = portal.tenantFeatures === true;
  if (user) signIn(token, user);
  if (trigger === "update" && session) {
    applyClientUpdate(token, session as Record<string, unknown>);
  }

  if (token.refreshTokenExpiry && Date.now() > token.refreshTokenExpiry) {
    authLogger.warn("refresh token expired", {
      expires_at: new Date(token.refreshTokenExpiry).toISOString(),
    });
    // A running preview ends with the session it hangs on, also in the
    // audit trail, before the session callback drops the token.
    if (preview && isPreviewActive(token)) await endPreviewOnSessionLoss(token);
    return endSession(token, "RefreshTokenExpired");
  }

  // Terminal: the backend explicitly rejected the refresh session (401/403).
  // Timeouts, network interruptions and 5xx keep the refresh token, so a
  // sleeping tablet can retry when it is back online (#2952).
  if (token.error === "RefreshTokenError") return token;

  // A cookie the backend already refused to renew ends without a backend
  // call: a late response may have written it back after a forced renewal
  // ended it (#5194).
  if (refreshAlreadyRefused(token)) {
    if (preview && isPreviewActive(token)) await endPreviewOnSessionLoss(token);
    return endSession(token, "RefreshTokenError");
  }

  // While a preview is active its branch owns maintenance; only a preview
  // that just ended falls through, so the restored admin token can renew.
  if (preview && isPreviewActive(token)) {
    const stillPreviewing = await maintainPreviewSession(
      token,
      portal.backend.refresh,
      Date.now(),
    );
    if (stillPreviewing) return token;
  }

  const now = Date.now();
  if (
    !token.tokenExpiry ||
    !token.refreshToken ||
    !token.refreshTokenExpiry ||
    !(
      isRenewalForced() ||
      now > token.tokenExpiry - ACCESS_TOKEN_REFRESH_BUFFER_MS
    ) ||
    now >= token.refreshTokenExpiry
  ) {
    return token;
  }

  const rotation = await rotateRefreshToken(token, portal.backend.refresh);
  if (rotation.status === "success") {
    const accessToken = applyRotation(token, rotation);
    token.token = accessToken;
    token.tokenExpiry = Date.now() + accessTokenExpiry;
    token.error = undefined;
    token.needsRefresh = undefined;
    const payload = parseJwtPayload(accessToken);
    if (payload) syncTokenFromPayload(token, payload, portal.identity);
    authLogger.info(
      rotation.shared
        ? "proactive_token_refresh_deduplicated"
        : "proactive_token_refresh_succeeded",
    );
  } else if (rotation.status === "terminal") {
    authLogger.warn(
      rotation.remembered
        ? "token_refresh_terminal_failure_remembered"
        : "token_refresh_terminal_failure",
    );
    endSession(token, "RefreshTokenError");
  } else {
    authLogger.warn("token_refresh_deferred_after_transient_failure");
  }
  return token;
}

/** The JWT callback of one portal: sign-in, renewal and the staff preview. */
export function createJwtCallback(portal: PortalDescriptor): JwtCallback {
  return async (params) => {
    const update = params.session as Record<string, unknown> | undefined;
    const previewStart =
      portal.tenantFeatures === true &&
      params.trigger === "update" &&
      isServerSessionMutation() &&
      !isPreviewActive(params.token) &&
      isPreviewStartUpdate(update?.previewStart)
        ? update.previewStart
        : undefined;
    const token = await maintainSession(portal, params);
    // unstable_update reads the original request cookie. Renew it (or reuse
    // the outer wrapper's cached renewal) before validating and parking admin state.
    if (!token.error && previewStart) {
      await applyPreviewStart(token, previewStart);
    }
    return token;
  };
}

/**
 * The server's session of a decoded session cookie. The browser never
 * receives it as is: the session routes and layout snapshots map it through
 * toBrowserSession (#5005). A session without an access token always carries
 * an error, so the browser can tell a dead session apart without the token.
 */
export function serverSessionFromJwt(
  token: JWT,
  session: Session,
): ServerSession {
  if (
    token.error === "RefreshTokenExpired" ||
    token.error === "RefreshTokenError" ||
    !token.token
  ) {
    return {
      ...session,
      user: {
        ...session.user,
        id: token.id ?? "",
        email: token.email ?? "",
        token: "",
        refreshToken: "",
        roles: [],
        roleIsSystem: [],
        permissions: [],
        firstName: token.firstName ?? "",
        isAdmin: false,
        scope: token.scope,
      },
      error: token.error ?? "RefreshTokenError",
    };
  }

  return {
    ...session,
    user: {
      ...session.user,
      id: token.id ?? "",
      email: token.email ?? "",
      token: token.token,
      refreshToken: token.refreshToken,
      roles: token.roles,
      roleIsSystem: token.roleIsSystem,
      permissions: token.permissions,
      firstName: token.firstName,
      isAdmin: token.isAdmin ?? false,
      tenantId: token.tenantId,
      orgId: token.orgId,
      scope: token.scope,
      viaTraeger: token.viaTraeger ?? false,
      isPreview: isPreviewActive(token),
      previewTargetName: token.previewTargetName,
      previewTargetAccountId: token.previewTargetAccountId,
    },
  };
}

export const sessionCallback: NonNullable<Callbacks["session"]> = ({
  session,
  token,
}) => serverSessionFromJwt(token, session);
