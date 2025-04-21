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
      // ...other properties
      // role: UserRole;
    } & DefaultSession["user"];
  }

  interface User {
    token?: string;
    refreshToken?: string;
    // ...other properties
    // role: UserRole;
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
        password: { label: "Password", type: "password" }
      },
      async authorize(credentials, request) {
        // Adding request parameter to match the expected type signature
        if (!credentials?.email || !credentials?.password) return null;
        
        try {
          // Improved error handling with more detailed logging
          console.log(`Attempting login with API URL: ${env.NEXT_PUBLIC_API_URL}/auth/login`);
          
          const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/auth/login`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              email: credentials.email,
              password: credentials.password
            }),
          });
          
          // Log the response status to help with debugging
          console.log(`Login response status: ${response.status}`);
          
          if (!response.ok) {
            const text = await response.text();
            console.error(`Login failed with status ${response.status}: ${text}`);
            return null;
          }

          const data = await response.json();
          
          console.log("Login response:", JSON.stringify(data));

          // Parse the JWT token to get the user info
          // This avoids making a separate API call and possible auth issues
          const tokenParts = data.access_token.split('.');
          if (tokenParts.length !== 3) {
            console.error("Invalid token format");
            return null;
          }
          
          try {
            // Decode the payload (middle part of JWT)
            const payload = JSON.parse(Buffer.from(tokenParts[1], 'base64').toString());
            console.log("Token payload:", payload);
            
            // Using type assertions for credentials to satisfy TypeScript
            return {
              id: String(payload.id),
              name: payload.sub || payload.username || String(credentials.email),
              email: String(credentials.email),
              token: data.access_token,
              refreshToken: data.refresh_token
            };
          } catch (e) {
            console.error("Error parsing JWT:", e);
            return null;
          }
        } catch (error) {
          console.error("Authentication error:", error);
          return null;
        }
      }
    })
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
    jwt: ({ token, user }) => {
      if (user) {
        token.id = user.id;
        token.name = user.name;
        token.email = user.email;
        token.token = user.token;
        token.refreshToken = user.refreshToken;
      }
      return token;
    },
    session: ({ session, token }) => {
      return {
        ...session,
        user: {
          ...session.user,
          id: token.id as string,
          token: token.token as string
        }
      };
    },
  },
  pages: {
    signIn: "/login"
  }
} satisfies NextAuthConfig;
