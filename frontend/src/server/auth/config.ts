import type { DefaultSession, NextAuthConfig, User } from "next-auth";
import DiscordProvider from "next-auth/providers/discord";
import CredentialsProvider from "next-auth/providers/credentials";
import { env } from "~/env";

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
    console.error("Invalid token format");
    return null;
  }

  const payloadPart = tokenParts[1];
  if (!payloadPart) {
    console.error("Invalid token part");
    return null;
  }

  try {
    return JSON.parse(
      Buffer.from(payloadPart, "base64").toString(),
    ) as JwtPayload;
  } catch (e) {
    console.error("Error parsing JWT:", e);
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
    roles: roles,
    firstName: payload.first_name,
    isAdmin: payload.is_admin ?? false,
  };
}

/**
 * Perform login API call to backend
 */
async function performLogin(
  email: string,
  password: string,
  isDev: boolean,
): Promise<{ access_token: string; refresh_token: string } | null> {
  const apiUrl = env.NEXT_PUBLIC_API_URL;

  if (isDev) {
    console.log(`Attempting login with API URL: ${apiUrl}/auth/login`);
  }

  try {
    const response = await fetch(`${apiUrl}/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password }),
    });

    if (isDev) {
      console.log(`Login response status: ${response.status}`);
    }

    if (!response.ok) {
      const text = await response.text();
      console.error(`Login failed with status ${response.status}: ${text}`);
      return null;
    }

    const responseData = (await response.json()) as {
      access_token: string;
      refresh_token: string;
    };

    if (isDev) {
      console.log("Login response:", JSON.stringify(responseData));
    }

    return responseData;
  } catch (error) {
    console.error("Authentication error:", error);
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
    } & DefaultSession["user"];
    error?: "RefreshTokenExpired" | "RefreshTokenError";
  }

  interface User {
    token?: string;
    refreshToken?: string;
    roles?: string[];
    firstName?: string;
    isAdmin?: boolean;
  }

  interface JWT {
    id?: string;
    token?: string;
    refreshToken?: string;
    roles?: string[];
    firstName?: string;
    isAdmin?: boolean;
    tokenExpiry?: number;
    refreshTokenExpiry?: number;
    error?: "RefreshTokenExpired" | "RefreshTokenError";
    needsRefresh?: boolean;
    isRefreshing?: boolean;
    lastRefreshAttempt?: number;
    refreshRetries?: number;
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
            console.log("Handling internal token refresh");
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
          console.log("Token payload:", payload);
          if (payload.roles && Array.isArray(payload.roles)) {
            console.log("Found roles in token:", payload.roles);
          } else {
            console.warn(
              "No roles found in token, this will cause authorization failures",
            );
          }
          console.log(
            "Using display name:",
            buildDisplayName(payload, creds.email),
          );
        }

        return buildAuthUser(
          payload,
          loginResult.access_token,
          loginResult.refresh_token,
          creds.email,
        );
      },
    }),
    /**
     * ...add more providers here.
     *
     * Most other providers require a bit more work than the Discord provider. For example, the
     * GitHub provider requires you to add the `refresh_token_expires_in` field to the Account
     * model. Refer to the NextAuth.js docs for the provider you want to use. Example:
     *
     * @see https://next-auth.js.org/providers/github
     */
  ],
  callbacks: {
    jwt: async ({ token, user }) => {
      // Only log in development to avoid production log spam
      const isDev = process.env.NODE_ENV === "development";

      if (isDev) {
        const callerId = `jwt-callback-${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
        const stack = new Error("Stack trace for caller identification").stack;
        const caller = stack?.split("\n")[3]?.trim() ?? "Unknown caller";
        console.log(`\n=== [${callerId}] JWT Callback Invoked ===`);
        console.log(
          `[${callerId}] Triggered by: NextAuth internal (server-side session access)`,
        );
        console.log(`[${callerId}] Stack trace hint: ${caller}`);
        console.log(`[${callerId}] Has user object: ${!!user}`);
        console.log(
          `[${callerId}] Current refresh token: ${token.refreshToken ? (token.refreshToken as string).substring(0, 50) + "..." : "none"}`,
        );
        console.log(
          `[${callerId}] Token expiry: ${token.tokenExpiry ? new Date(token.tokenExpiry as number).toISOString() : "not set"}`,
        );
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
        // Store token expiry from environment
        token.tokenExpiry = Date.now() + accessTokenExpiry;
        // Store refresh token expiry (matching backend)
        token.refreshTokenExpiry = Date.now() + refreshTokenExpiry;
        // Clear any previous error states
        token.error = undefined;
        token.needsRefresh = undefined;

        // Log token configuration for debugging (only in development)
        if (isDev) {
          console.log("=== Authentication Token Configuration ===");
          console.log(
            `Access Token Expiry: ${env.AUTH_JWT_EXPIRY} (expires at ${new Date(token.tokenExpiry as number).toISOString()})`,
          );
          console.log(
            `Refresh Token Expiry: ${env.AUTH_JWT_REFRESH_EXPIRY} (expires at ${new Date(token.refreshTokenExpiry as number).toISOString()})`,
          );
          console.log(
            `NextAuth Session Length: ${env.AUTH_JWT_REFRESH_EXPIRY}`,
          );
          console.log(
            `Token Refresh: Handled by axios interceptor on 401 errors`,
          );
          console.log("========================================");
        }
      }

      // Check if refresh token is expired
      if (
        token.refreshTokenExpiry &&
        Date.now() > (token.refreshTokenExpiry as number)
      ) {
        console.warn("Refresh token has expired, user needs to re-login");
        token.error = "RefreshTokenExpired";
        token.needsRefresh = true;
        // Keep user data for graceful degradation
        return token;
      }

      // JWT callback no longer handles token refresh
      // All token refresh is now handled by the axios interceptor when it receives 401 errors
      // This eliminates race conditions from multiple concurrent refresh attempts
      if (isDev) {
        console.log(
          "JWT callback completed - no refresh attempted (handled by axios interceptor)",
        );
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
