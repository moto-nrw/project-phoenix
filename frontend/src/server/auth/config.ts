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
      },
      async authorize(credentials, _request) {
        // Adding request parameter to match the expected type signature
        if (!credentials?.email || !credentials?.password) return null;

        try {
          // Improved error handling with more detailed logging
          console.log(
            `Attempting login with API URL: ${env.NEXT_PUBLIC_API_URL}/auth/login`,
          );

          const response = await fetch(
            `${env.NEXT_PUBLIC_API_URL}/auth/login`,
            {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({
                email: credentials.email,
                password: credentials.password,
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
              email: credentials.email as string,
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
        // Store refresh token expiry (1 hour from now)
        token.refreshTokenExpiry = Date.now() + 60 * 60 * 1000; // 1 hour
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

      // Check if access token needs refresh (with 1 minute buffer)
      if (token.tokenExpiry && Date.now() > (token.tokenExpiry as number) - 60 * 1000) {
        console.log("Access token expiring soon, attempting refresh...");
        
        try {
          // Attempt to refresh the token
          const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/auth/refresh`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              refresh_token: token.refreshToken,
            }),
          });

          if (response.ok) {
            const refreshData = (await response.json()) as {
              access_token: string;
              refresh_token: string;
            };
            
            // Update tokens
            token.token = refreshData.access_token;
            token.refreshToken = refreshData.refresh_token;
            token.tokenExpiry = Date.now() + 15 * 60 * 1000; // Reset access token expiry
            token.refreshTokenExpiry = Date.now() + 60 * 60 * 1000; // Reset refresh token expiry
            
            // Clear error states on successful refresh
            token.error = undefined;
            token.needsRefresh = undefined;
            
            console.log("Token refreshed successfully");
          } else {
            console.error(`Failed to refresh token: ${response.status}`);
            
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
