import type { DefaultSession, NextAuthConfig, User } from "next-auth";
import DiscordProvider from "next-auth/providers/discord";
import CredentialsProvider from "next-auth/providers/credentials";
import { env } from "~/env";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

// Logger instance for NextAuth config
const logger = createLogger({ component: "NextAuthConfig" });

/**
 * JWT payload structure from backend tokens
 */
interface JwtPayload {
  id: string | number;
  sub?: string;
  username?: string;
  first_name?: string;
  last_name?: string;
  email?: string;
  roles?: string[];
  is_admin?: boolean;
}

/**
 * Parse JWT token and extract payload
 * @returns Parsed payload or null if invalid
 */
function parseJwtPayload(tokenString: string): JwtPayload | null {
  const tokenParts = tokenString.split(".");
  if (tokenParts.length !== 3) {
    logger.error("invalid jwt token format", {});
    return null;
  }

  const payloadPart = tokenParts[1];
  if (!payloadPart) {
    logger.error("invalid jwt token part", {});
    return null;
  }

  try {
    return JSON.parse(
      Buffer.from(payloadPart, "base64").toString(),
    ) as JwtPayload;
  } catch (e) {
    logger.error("error parsing jwt", {
      error: e instanceof Error ? e.message : String(e),
    });
    return null;
  }
}

/**
 * Build display name from JWT payload with fallbacks
 */
function buildDisplayName(
  payload: JwtPayload,
  fallbackEmail: string,
  ultimateFallback = "User",
): string {
  if (payload.first_name && payload.last_name) {
    return `${payload.first_name} ${payload.last_name}`;
  }
  if (payload.first_name) {
    return payload.first_name;
  }
  return payload.username ?? (fallbackEmail || ultimateFallback);
}

/**
 * Build NextAuth User object from JWT payload
 */
function buildAuthUser(
  payload: JwtPayload,
  token: string,
  refreshToken: string,
  email: string,
  scope?: string,
): User {
  // Defensive check: ensure roles is actually an array (matches original login behavior)
  const roles =
    payload.roles && Array.isArray(payload.roles) ? payload.roles : [];

  return {
    id: String(payload.id),
    name: buildDisplayName(payload, email),
    email: email,
    token: token,
    refreshToken: refreshToken,
    roles: scope === "platform" ? ["operator"] : roles,
    firstName: payload.first_name,
    isAdmin: payload.is_admin ?? false,
    scope: scope,
  };
}

/**
 * Perform operator login API call to backend.
 * Operator login returns an envelope: { status, data: { access_token, refresh_token, operator } }
 */
async function performOperatorLogin(
  email: string,
  password: string,
  isDev: boolean,
): Promise<{ access_token: string; refresh_token: string } | null> {
  const apiUrl = getServerApiUrl();

  if (isDev) {
    logger.debug("attempting operator login", {
      api_url: `${apiUrl}/operator/auth/login`,
    });
  }

  try {
    const response = await fetch(`${apiUrl}/operator/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password }),
    });

    if (isDev) {
      logger.debug("operator login response received", {
        status: response.status,
      });
    }

    if (!response.ok) {
      const text = await response.text();
      logger.error("operator login failed", {
        status: response.status,
        error: text,
      });
      return null;
    }

    const envelope = (await response.json()) as {
      status: string;
      data: {
        access_token: string;
        refresh_token: string;
        operator: { id: number; email: string; display_name: string };
      };
    };

    if (isDev) {
      logger.debug("operator login response parsed", {
        has_tokens: !!envelope.data.access_token,
      });
    }

    return {
      access_token: envelope.data.access_token,
      refresh_token: envelope.data.refresh_token,
    };
  } catch (error) {
    logger.error("operator authentication error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return null;
  }
}

/**
 * Perform login API call to backend
 */
async function performLogin(
  email: string,
  password: string,
  isDev: boolean,
): Promise<{ access_token: string; refresh_token: string } | null> {
  const apiUrl = getServerApiUrl();

  if (isDev) {
    logger.debug("attempting login", { api_url: `${apiUrl}/auth/login` });
  }

  try {
    const response = await fetch(`${apiUrl}/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password }),
    });

    if (isDev) {
      logger.debug("login response received", { status: response.status });
    }

    if (!response.ok) {
      const text = await response.text();
      logger.error("login failed", { status: response.status, error: text });
      return null;
    }

    const responseData = (await response.json()) as {
      access_token: string;
      refresh_token: string;
    };

    if (isDev) {
      logger.debug("login response parsed", {
        has_tokens: !!responseData.access_token,
      });
    }

    return responseData;
  } catch (error) {
    logger.error("authentication error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return null;
  }
}

/**
 * Module augmentation for `next-auth` types. Allows us to add custom properties to the `session`
 * object and keep type safety.
 *
 * @see https://next-auth.js.org/getting-started/typescript#module-augmentation
 */
declare module "next-auth" {
  interface Session extends DefaultSession {
    user: {
      id: string;
      token?: string;
      refreshToken?: string;
      roles?: string[];
      firstName?: string;
      isAdmin?: boolean;
      scope?: string;
    } & DefaultSession["user"];
    error?: "RefreshTokenExpired" | "RefreshTokenError";
  }

  interface User {
    token?: string;
    refreshToken?: string;
    roles?: string[];
    firstName?: string;
    isAdmin?: boolean;
    scope?: string;
  }

  interface JWT {
    id?: string;
    token?: string;
    refreshToken?: string;
    roles?: string[];
    firstName?: string;
    isAdmin?: boolean;
    scope?: string;
    tokenExpiry?: number;
    refreshTokenExpiry?: number;
    error?: "RefreshTokenExpired" | "RefreshTokenError";
    needsRefresh?: boolean;
  }
}

function parseDurationToMs(duration: string): number {
  const regex = /^(\d+)([hm])$/;
  const match = regex.exec(duration);
  if (!match) return 12 * 60 * 60 * 1000; // 12h default
  const amount = match[1]!;
  const unit = match[2]!;
  const num = Number.parseInt(amount, 10);
  return unit === "h" ? num * 60 * 60 * 1000 : num * 60 * 1000;
}

// Get token expiries from environment
const accessTokenExpiry = parseDurationToMs(env.AUTH_JWT_EXPIRY);
const refreshTokenExpiry = parseDurationToMs(env.AUTH_JWT_REFRESH_EXPIRY);

// Frontend-side singleflight for proactive token refresh.
// Deduplicates concurrent JWT callbacks that all try to refresh the same token.
// The cache covers late-arriving callbacks after the in-flight promise resolves.
type RefreshResult = { access_token: string; refresh_token: string };
let activeRefreshPromise: Promise<RefreshResult | null> | null = null;
let activeRefreshKey: string | null = null;
let refreshCache: {
  oldToken: string;
  result: RefreshResult;
  expiresAt: number;
} | null = null;
const REFRESH_CACHE_TTL_MS = 5 * 60 * 1000; // Match REFRESH_BUFFER_MS — covers the full pre-expiry window

/** @internal Exposed for unit testing only */
export const _testHelpers = {
  parseJwtPayload,
  buildDisplayName,
  buildAuthUser,
  performLogin,
  performOperatorLogin,
  parseDurationToMs,
} as const;

/** @internal Reset module-level refresh state (test isolation only) */
export function _resetRefreshState(): void {
  activeRefreshPromise = null;
  activeRefreshKey = null;
  refreshCache = null;
}

/**
 * Options for NextAuth.js used to configure adapters, providers, callbacks, etc.
 *
 * @see https://next-auth.js.org/configuration/options
 */
export const authConfig = {
  providers: [
    DiscordProvider,
    CredentialsProvider({
      name: "Credentials",
      credentials: {
        email: { label: "Email", type: "email" },
        password: { label: "Password", type: "password" },
        internalRefresh: { label: "Internal Refresh", type: "text" },
        token: { label: "Token", type: "text" },
        refreshToken: { label: "Refresh Token", type: "text" },
      },
      async authorize(credentials, _request) {
        const creds = credentials as Record<string, string> | undefined;
        const isDev = process.env.NODE_ENV === "development";

        // Handle internal token refresh
        if (
          creds?.internalRefresh === "true" &&
          creds?.token &&
          creds?.refreshToken
        ) {
          if (isDev) {
            logger.debug("handling internal token refresh", {});
          }

          const payload = parseJwtPayload(creds.token);
          if (!payload) return null;

          const email = payload.email ?? payload.sub ?? "";
          return buildAuthUser(payload, creds.token, creds.refreshToken, email);
        }

        // Regular login flow
        if (!creds?.email || !creds?.password) return null;

        const loginResult = await performLogin(
          creds.email,
          creds.password,
          isDev,
        );
        if (!loginResult) return null;

        const payload = parseJwtPayload(loginResult.access_token);
        if (!payload) return null;

        // Development logging for debugging
        if (isDev) {
          logger.debug("token payload parsed", { has_roles: !!payload.roles });
          if (payload.roles && Array.isArray(payload.roles)) {
            logger.debug("found roles in token", { roles: payload.roles });
          } else {
            logger.warn("no roles found in token", {});
          }
          logger.debug("display name", {
            name: buildDisplayName(payload, creds.email),
          });
        }

        return buildAuthUser(
          payload,
          loginResult.access_token,
          loginResult.refresh_token,
          creds.email,
        );
      },
    }),
    CredentialsProvider({
      id: "operator-credentials",
      name: "Operator Credentials",
      credentials: {
        email: { label: "Email", type: "email" },
        password: { label: "Password", type: "password" },
        internalRefresh: { label: "Internal Refresh", type: "text" },
        token: { label: "Token", type: "text" },
        refreshToken: { label: "Refresh Token", type: "text" },
      },
      async authorize(credentials, _request) {
        const creds = credentials as Record<string, string> | undefined;
        const isDev = process.env.NODE_ENV === "development";

        // Handle internal token refresh (same pattern as teacher)
        if (
          creds?.internalRefresh === "true" &&
          creds?.token &&
          creds?.refreshToken
        ) {
          if (isDev) {
            logger.debug("handling operator internal token refresh", {});
          }

          const payload = parseJwtPayload(creds.token);
          if (!payload) return null;

          const email = payload.email ?? payload.sub ?? "";
          return buildAuthUser(
            payload,
            creds.token,
            creds.refreshToken,
            email,
            "platform",
          );
        }

        // Regular operator login flow
        if (!creds?.email || !creds?.password) return null;

        const loginResult = await performOperatorLogin(
          creds.email,
          creds.password,
          isDev,
        );
        if (!loginResult) return null;

        const payload = parseJwtPayload(loginResult.access_token);
        if (!payload) return null;

        return buildAuthUser(
          payload,
          loginResult.access_token,
          loginResult.refresh_token,
          creds.email,
          "platform",
        );
      },
    }),
  ],
  callbacks: {
    jwt: async ({ token, user }) => {
      // Only log in development to avoid production log spam
      const isDev = process.env.NODE_ENV === "development";

      if (isDev) {
        const callerId = `jwt-callback-${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
        const stack = new Error("Stack trace for caller identification").stack;
        const caller = stack?.split("\n")[3]?.trim() ?? "Unknown caller";
        logger.debug("jwt callback invoked", {
          caller_id: callerId,
          caller,
          has_user: !!user,
          has_refresh_token: !!token.refreshToken,
          token_expiry: token.tokenExpiry
            ? new Date(token.tokenExpiry as number).toISOString()
            : "not set",
        });
      }

      // Initial sign in
      if (user) {
        token.id = user.id;
        token.name = user.name;
        token.email = user.email;
        token.token = user.token ?? "";
        token.refreshToken = user.refreshToken ?? "";
        token.roles = user.roles;
        token.firstName = user.firstName;
        token.isAdmin = user.isAdmin;
        token.scope = user.scope;
        // Store token expiry from environment
        token.tokenExpiry = Date.now() + accessTokenExpiry;
        // Store refresh token expiry (matching backend)
        token.refreshTokenExpiry = Date.now() + refreshTokenExpiry;
        // Clear any previous error states
        token.error = undefined;
        token.needsRefresh = undefined;

        // Log token configuration for debugging (only in development)
        if (isDev) {
          logger.debug("authentication token configuration", {
            access_token_expiry: env.AUTH_JWT_EXPIRY,
            access_expires_at: new Date(
              token.tokenExpiry as number,
            ).toISOString(),
            refresh_token_expiry: env.AUTH_JWT_REFRESH_EXPIRY,
            refresh_expires_at: new Date(
              token.refreshTokenExpiry as number,
            ).toISOString(),
          });
        }
      }

      // Check if refresh token is expired
      if (
        token.refreshTokenExpiry &&
        Date.now() > (token.refreshTokenExpiry as number)
      ) {
        logger.warn("refresh token expired", {
          expires_at: new Date(
            token.refreshTokenExpiry as number,
          ).toISOString(),
        });
        token.error = "RefreshTokenExpired";
        token.needsRefresh = true;
        // Keep user data for graceful degradation
        return token;
      }

      // Proactive token refresh: refresh access token before it expires.
      // SessionProvider.refetchInterval (4 min) ensures this runs regularly.
      // Backend singleflight protects against concurrent refresh calls.
      // Frontend singleflight + cache prevents stale token overwrites when
      // multiple JWT callbacks race (the late-arriving callback would otherwise
      // use the old refresh token, get 401, and overwrite good tokens).
      // Fires whenever the access token is within the pre-expiry buffer OR
      // already expired, as long as the refresh token is still valid.
      // After a successful refresh, tokenExpiry resets to now + 1h, so the
      // condition won't re-fire until 55 minutes later.
      const REFRESH_BUFFER_MS = 5 * 60 * 1000; // 5 minutes before expiry
      const REFRESH_TIMEOUT_MS = 5_000; // 5 second timeout

      const now = Date.now();
      const tokenExpiry = token.tokenExpiry as number;
      if (
        token.tokenExpiry &&
        token.refreshToken &&
        token.refreshTokenExpiry &&
        now > tokenExpiry - REFRESH_BUFFER_MS &&
        now < (token.refreshTokenExpiry as number)
      ) {
        const currentRefreshToken = token.refreshToken as string;

        // Check cache: this refresh token was recently rotated by another callback
        if (
          refreshCache?.oldToken === currentRefreshToken &&
          Date.now() < (refreshCache?.expiresAt ?? 0)
        ) {
          token.token = refreshCache.result.access_token;
          token.refreshToken = refreshCache.result.refresh_token;
          token.tokenExpiry = Date.now() + accessTokenExpiry;
          token.refreshTokenExpiry = Date.now() + refreshTokenExpiry;
          token.error = undefined;
          token.needsRefresh = undefined;
          logger.info("proactive_token_refresh_deduplicated");
          return token;
        }

        // Join in-flight refresh if one exists for this token
        if (activeRefreshPromise && activeRefreshKey === currentRefreshToken) {
          const result = await activeRefreshPromise;
          if (result) {
            token.token = result.access_token;
            token.refreshToken = result.refresh_token;
            token.tokenExpiry = Date.now() + accessTokenExpiry;
            token.refreshTokenExpiry = Date.now() + refreshTokenExpiry;
            token.error = undefined;
            token.needsRefresh = undefined;
            logger.info("proactive_token_refresh_succeeded");
          } else if (now > tokenExpiry) {
            token.error = "RefreshTokenError";
            token.needsRefresh = true;
            logger.warn("token_refresh_failed_post_expiry");
          }
          return token;
        }

        // Start new refresh and share via module-level promise
        const isOperator = token.scope === "platform";
        const refreshUrl = isOperator
          ? `${getServerApiUrl()}/operator/auth/refresh`
          : `${getServerApiUrl()}/auth/refresh`;

        activeRefreshKey = currentRefreshToken;
        activeRefreshPromise = (async (): Promise<RefreshResult | null> => {
          try {
            const response = await fetch(refreshUrl, {
              method: "POST",
              headers: {
                Authorization: `Bearer ${currentRefreshToken}`,
                "Content-Type": "application/json",
              },
              signal: AbortSignal.timeout(REFRESH_TIMEOUT_MS),
            });

            if (response.ok) {
              // Operator refresh returns envelope { data: { access_token, refresh_token } }
              // Teacher refresh returns flat { access_token, refresh_token }
              let tokens: RefreshResult;
              if (isOperator) {
                const envelope = (await response.json()) as {
                  data: RefreshResult;
                };
                tokens = envelope.data;
              } else {
                tokens = (await response.json()) as RefreshResult;
              }
              // Cache result so late-arriving callbacks with the old token
              // get the new tokens instead of sending a doomed request
              refreshCache = {
                oldToken: currentRefreshToken,
                result: tokens,
                expiresAt: Date.now() + REFRESH_CACHE_TTL_MS,
              };
              return tokens;
            }
            logger.warn("proactive_token_refresh_failed", {
              status: response.status,
              scope: token.scope,
            });
            return null;
          } catch (err) {
            logger.warn("proactive_token_refresh_error", {
              error: err instanceof Error ? err.message : String(err),
              scope: token.scope,
            });
            return null;
          } finally {
            activeRefreshPromise = null;
            activeRefreshKey = null;
          }
        })();

        const result = await activeRefreshPromise;
        if (result) {
          token.token = result.access_token;
          token.refreshToken = result.refresh_token;
          token.tokenExpiry = Date.now() + accessTokenExpiry;
          token.refreshTokenExpiry = Date.now() + refreshTokenExpiry;
          token.error = undefined;
          token.needsRefresh = undefined;
          logger.info("proactive_token_refresh_succeeded");
        } else if (now > tokenExpiry) {
          token.error = "RefreshTokenError";
          token.needsRefresh = true;
          logger.warn("token_refresh_failed_post_expiry");
        }
      }

      return token;
    },
    session: ({ session, token }) => {
      // Check for token errors
      if (
        token.error === "RefreshTokenExpired" ||
        token.error === "RefreshTokenError" ||
        !token.token
      ) {
        // Return a minimal session that will trigger auth checks to fail
        return {
          ...session,
          user: {
            ...session.user,
            id: (token.id as string) || "",
            email: token.email ?? "",
            token: "", // Empty token will cause API calls to fail with 401
            refreshToken: "",
            roles: [],
            firstName: (token.firstName as string) || "",
            isAdmin: false,
            scope: token.scope as string | undefined,
          },
          error: token.error,
        };
      }

      return {
        ...session,
        user: {
          ...session.user,
          id: token.id as string,
          email: token.email ?? "",
          token: token.token as string,
          refreshToken: token.refreshToken as string,
          roles: token.roles as string[],
          firstName: token.firstName as string,
          isAdmin: (token.isAdmin as boolean) ?? false,
          scope: token.scope as string | undefined,
        },
      };
    },
  },
  pages: {
    signIn: "/",
  },
  session: {
    strategy: "jwt",
    maxAge: Math.floor(refreshTokenExpiry / 1000), // Match refresh token expiry
  },
} satisfies NextAuthConfig;
