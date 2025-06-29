import { type DefaultSession, type NextAuthConfig } from "next-auth";
import DiscordProvider from "next-auth/providers/discord";
import CredentialsProvider from "next-auth/providers/credentials";
import { env } from "~/env";

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
      isTeacher?: boolean;
    } & DefaultSession["user"];
    error?: "RefreshTokenExpired" | "RefreshTokenError";
  }

  interface User {
    token?: string;
    refreshToken?: string;
    roles?: string[];
    firstName?: string;
    isAdmin?: boolean;
    isTeacher?: boolean;
  }
  
  interface JWT {
    id?: string;
    token?: string;
    refreshToken?: string;
    roles?: string[];
    firstName?: string;
    isAdmin?: boolean;
    isTeacher?: boolean;
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
  const num = parseInt(amount, 10);
  return unit === 'h' ? num * 60 * 60 * 1000 : num * 60 * 1000;
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
        // Cast credentials to have string values
        const creds = credentials as Record<string, string> | undefined;
        
        // Handle internal token refresh
        if (creds?.internalRefresh === "true" && creds?.token && creds?.refreshToken) {
          console.log("Handling internal token refresh");
          
          // Parse the JWT token to get user info
          const tokenString = creds.token;
          const tokenParts = tokenString.split(".");
          if (tokenParts.length !== 3) {
            console.error("Invalid token format during refresh");
            return null;
          }
          
          try {
            const payloadPart = tokenParts[1];
            if (!payloadPart) {
              console.error("Invalid token part during refresh");
              return null;
            }
            const payload = JSON.parse(
              Buffer.from(payloadPart, "base64").toString(),
            ) as {
              id: string | number;
              sub?: string;
              username?: string;
              first_name?: string;
              last_name?: string;
              email?: string;
              roles?: string[];
              is_admin?: boolean;
              is_teacher?: boolean;
            };
            
            // Extract email and roles from token
            const email = payload.email ?? payload.sub ?? "";
            const roles = payload.roles ?? [];
            
            // Construct display name
            const displayName = payload.first_name
              ? payload.last_name
                ? `${payload.first_name} ${payload.last_name}`
                : payload.first_name
              : payload.username ?? email ?? "User";
            
            return {
              id: String(payload.id),
              name: displayName,
              email: email,
              token: creds.token,
              refreshToken: creds.refreshToken,
              roles: roles,
              firstName: payload.first_name,
              isAdmin: payload.is_admin ?? false,
              isTeacher: payload.is_teacher ?? false,
            };
          } catch (e) {
            console.error("Error parsing JWT during refresh:", e);
            return null;
          }
        }
        
        // Regular login flow
        if (!creds?.email || !creds?.password) return null;

        try {
          // Improved error handling with more detailed logging
          // Use server URL in server context (Docker environment)
          const apiUrl = env.NEXT_PUBLIC_API_URL;
          console.log(
            `Attempting login with API URL: ${apiUrl}/auth/login`,
          );
          const response = await fetch(
            `${apiUrl}/auth/login`,
            {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({
                email: creds.email,
                password: creds.password,
              }),
            },
          );

          // Log the response status to help with debugging
          console.log(`Login response status: ${response.status}`);

          if (!response.ok) {
            const text = await response.text();
            console.error(
              `Login failed with status ${response.status}: ${text}`,
            );
            return null;
          }

          const responseData = (await response.json()) as {
            access_token: string;
            refresh_token: string;
          };

          console.log("Login response:", JSON.stringify(responseData));

          // Parse the JWT token to get the user info
          // This avoids making a separate API call and possible auth issues
          const tokenParts = responseData.access_token.split(".");
          if (tokenParts.length !== 3) {
            console.error("Invalid token format");
            return null;
          }

          try {
            // Decode the payload (middle part of JWT)
            // Ensure tokenParts[1] is defined before attempting to decode
            if (!tokenParts[1]) {
              console.error("Invalid token part");
              return null;
            }
            const payload = JSON.parse(
              Buffer.from(tokenParts[1], "base64").toString(),
            ) as {
              id: string | number;
              sub?: string;
              username?: string;
              first_name?: string;
              last_name?: string;
              roles?: string[];
              email?: string;
              is_admin?: boolean;
              is_teacher?: boolean;
            };
            console.log("Token payload:", payload);

            // Extract roles directly from the token payload - this is the correct way
            // The backend includes roles in the JWT token already
            let roles: string[] = [];

            if (payload.roles && Array.isArray(payload.roles)) {
              roles = payload.roles;
              console.log("Found roles in token:", roles);
            } else {
              console.warn(
                "No roles found in token, this will cause authorization failures",
              );
            }

            // Construct full name from JWT token, with fallbacks
            let displayName: string;
            if (payload.first_name && payload.last_name) {
              displayName = `${payload.first_name} ${payload.last_name}`;
            } else if (payload.first_name) {
              displayName = payload.first_name;
            } else {
              displayName = payload.username ?? (credentials.email as string);
            }
            console.log("Using display name:", displayName);

            // Using type assertions for credentials to satisfy TypeScript
            return {
              id: String(payload.id),
              name: displayName,
              email: creds.email,
              token: responseData.access_token,
              refreshToken: responseData.refresh_token,
              roles: roles,
              firstName: payload.first_name,
              isAdmin: payload.is_admin ?? false,
              isTeacher: payload.is_teacher ?? false,
            };
          } catch (e) {
            console.error("Error parsing JWT:", e);
            return null;
          }
        } catch (error) {
          console.error("Authentication error:", error);
          return null;
        }
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
      const callerId = `jwt-callback-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
      const stack = new Error().stack;
      const caller = stack?.split('\n')[3]?.trim() ?? 'Unknown caller';
      console.log(`\n=== [${callerId}] JWT Callback Invoked ===`);
      console.log(`[${callerId}] Triggered by: NextAuth internal (server-side session access)`);
      console.log(`[${callerId}] Stack trace hint: ${caller}`);
      console.log(`[${callerId}] Has user object: ${!!user}`);
      console.log(`[${callerId}] Current refresh token: ${token.refreshToken ? (token.refreshToken as string).substring(0, 50) + '...' : 'none'}`);
      console.log(`[${callerId}] Token expiry: ${token.tokenExpiry ? new Date(token.tokenExpiry as number).toISOString() : 'not set'}`)
      
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
        token.isTeacher = user.isTeacher;
        // Store token expiry from environment
        token.tokenExpiry = Date.now() + accessTokenExpiry;
        // Store refresh token expiry (matching backend)
        token.refreshTokenExpiry = Date.now() + refreshTokenExpiry;
        // Clear any previous error states
        token.error = undefined;
        token.needsRefresh = undefined;
        
        // Log token configuration for debugging
        console.log("=== Authentication Token Configuration ===");
        console.log(`Access Token Expiry: ${env.AUTH_JWT_EXPIRY} (expires at ${new Date(token.tokenExpiry as number).toISOString()})`);
        console.log(`Refresh Token Expiry: ${env.AUTH_JWT_REFRESH_EXPIRY} (expires at ${new Date(token.refreshTokenExpiry as number).toISOString()})`);
        console.log(`NextAuth Session Length: ${env.AUTH_JWT_REFRESH_EXPIRY}`);
        console.log(`Token Refresh: Handled by axios interceptor on 401 errors`);
        console.log("========================================");
      }

      // Check if refresh token is expired
      if (token.refreshTokenExpiry && Date.now() > (token.refreshTokenExpiry as number)) {
        console.warn("Refresh token has expired, user needs to re-login");
        token.error = "RefreshTokenExpired";
        token.needsRefresh = true;
        // Keep user data for graceful degradation
        return token;
      }
      
      // JWT callback no longer handles token refresh
      // All token refresh is now handled by the axios interceptor when it receives 401 errors
      // This eliminates race conditions from multiple concurrent refresh attempts
      console.log(`[${callerId}] JWT callback completed - no refresh attempted (handled by axios interceptor)`);
      
      return token;
    },
    session: ({ session, token }) => {
      // Check for token errors
      if (token.error === "RefreshTokenExpired" || 
          token.error === "RefreshTokenError" ||
          !token.token) {
        // Return a minimal session that will trigger auth checks to fail
        return {
          ...session,
          user: {
            ...session.user,
            id: token.id as string || "",
            email: token.email ?? "",
            token: "", // Empty token will cause API calls to fail with 401
            refreshToken: "",
            roles: [],
            firstName: token.firstName as string || "",
            isAdmin: false,
            isTeacher: false,
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
          isAdmin: token.isAdmin as boolean ?? false,
          isTeacher: token.isTeacher as boolean ?? false,
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
