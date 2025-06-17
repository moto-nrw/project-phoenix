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
    } & DefaultSession["user"];
    error?: "RefreshTokenExpired" | "RefreshTokenError";
  }

  interface User {
    token?: string;
    refreshToken?: string;
    roles?: string[];
    firstName?: string;
  }
  
  interface JWT {
    id?: string;
    token?: string;
    refreshToken?: string;
    roles?: string[];
    firstName?: string;
    tokenExpiry?: number;
    refreshTokenExpiry?: number;
    error?: "RefreshTokenExpired" | "RefreshTokenError";
    needsRefresh?: boolean;
  }
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
          const apiUrl = process.env.NODE_ENV === 'production' || process.env.DOCKER_ENV
            ? 'http://server:8080'
            : env.NEXT_PUBLIC_API_URL;
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
              roles?: string[];
              email?: string;
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

            // Use username from JWT token as display name, with email as fallback
            const displayName: string = payload.username ?? (credentials.email as string);
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
      // Initial sign in
      if (user) {
        token.id = user.id;
        token.name = user.name;
        token.email = user.email;
        token.token = user.token ?? "";
        token.refreshToken = user.refreshToken ?? "";
        token.roles = user.roles;
        token.firstName = user.firstName;
        // Store token expiry (15 minutes from now)
        token.tokenExpiry = Date.now() + 15 * 60 * 1000; // 15 minutes
        // Store refresh token expiry (24 hours from now)
        token.refreshTokenExpiry = Date.now() + 24 * 60 * 60 * 1000; // 24 hours
        // Clear any previous error states
        token.error = undefined;
        token.needsRefresh = undefined;
      }

      // Check if refresh token is expired
      if (token.refreshTokenExpiry && Date.now() > (token.refreshTokenExpiry as number)) {
        console.warn("Refresh token has expired, user needs to re-login");
        token.error = "RefreshTokenExpired";
        token.needsRefresh = true;
        // Keep user data for graceful degradation
        return token;
      }

      // Check if access token needs refresh (with 5 minute buffer for proactive refresh)
      if (token.tokenExpiry && Date.now() > (token.tokenExpiry as number) - 5 * 60 * 1000) {
        // Rate limiting: Check if we've attempted refresh recently
        const lastRefreshAttempt = token.lastRefreshAttempt as number | undefined;
        const refreshCooldown = 30 * 1000; // 30 seconds cooldown between attempts
        
        if (lastRefreshAttempt && Date.now() - lastRefreshAttempt < refreshCooldown) {
          console.log("Skipping refresh - cooldown period active");
          return token;
        }
        
        // Retry tracking: Check if we've failed too many times
        const refreshRetries = (token.refreshRetries as number) || 0;
        const maxRetries = 3;
        
        if (refreshRetries >= maxRetries) {
          console.error("Max refresh retries exceeded");
          token.error = "RefreshTokenExpired";
          token.needsRefresh = true;
          return token;
        }
        
        console.log("Access token expiring soon, attempting refresh...");
        
        // Ensure refresh token exists before attempting refresh
        if (!token.refreshToken || typeof token.refreshToken !== 'string') {
          console.error("No refresh token available");
          token.error = "RefreshTokenExpired";
          token.needsRefresh = true;
          return token;
        }
        
        try {
          // Update last refresh attempt timestamp
          token.lastRefreshAttempt = Date.now();
          
          // Attempt to refresh the token
          // Use server URL in server context (Docker environment)
          const apiUrl = process.env.NODE_ENV === 'production' || process.env.DOCKER_ENV
            ? 'http://server:8080'
            : env.NEXT_PUBLIC_API_URL;
          const response = await fetch(`${apiUrl}/auth/refresh`, {
            method: "POST",
            headers: { 
              "Authorization": `Bearer ${token.refreshToken}`
            },
          });

          if (response.ok) {
            const refreshData = (await response.json()) as {
              access_token: string;
              refresh_token: string;
            };
            
            // Update tokens
            token.token = refreshData.access_token;
            token.refreshToken = refreshData.refresh_token;
            token.tokenExpiry = Date.now() + 15 * 60 * 1000; // Reset access token expiry (15 minutes)
            token.refreshTokenExpiry = Date.now() + 24 * 60 * 60 * 1000; // Reset refresh token expiry (24 hours)
            
            // Clear error states and reset retry count on successful refresh
            token.error = undefined;
            token.needsRefresh = undefined;
            token.refreshRetries = 0;
            token.lastRefreshAttempt = undefined;
            
            console.log("Token refreshed successfully");
          } else {
            console.error(`Failed to refresh token: ${response.status}`);
            
            // Increment retry count
            token.refreshRetries = ((token.refreshRetries as number) || 0) + 1;
            
            // Distinguish between different error types
            if (response.status === 401 || response.status === 403) {
              // Refresh token is invalid or expired
              console.error("Refresh token is invalid or expired");
              token.error = "RefreshTokenExpired";
              token.needsRefresh = true;
            } else {
              // Other API errors (500, network issues, etc.)
              console.error("API error during refresh, keeping session but marking as needs refresh");
              token.error = "RefreshTokenError";
              token.needsRefresh = true;
            }
            
            // Keep existing user data for graceful degradation
            return token;
          }
        } catch (error) {
          // Network errors or other exceptions
          console.error("Network error refreshing token:", error);
          
          // Increment retry count
          token.refreshRetries = ((token.refreshRetries as number) || 0) + 1;
          
          token.error = "RefreshTokenError";
          token.needsRefresh = true;
          
          // Keep existing user data for graceful degradation
          return token;
        }
      }
      
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
            token: "", // Empty token will cause API calls to fail with 401
            refreshToken: "",
            roles: [],
            firstName: token.firstName as string || "",
          },
          error: token.error,
        };
      }
      
      return {
        ...session,
        user: {
          ...session.user,
          id: token.id as string,
          token: token.token as string,
          refreshToken: token.refreshToken as string,
          roles: token.roles as string[],
          firstName: token.firstName as string,
        },
      };
    },
  },
  pages: {
    signIn: "/",
  },
  session: {
    strategy: "jwt",
    maxAge: 60 * 60, // 1 hour
  },
} satisfies NextAuthConfig;
