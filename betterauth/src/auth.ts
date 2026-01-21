import { betterAuth } from "better-auth";
import { organization } from "better-auth/plugins/organization";
import { Pool } from "pg";
import {
  ac,
  supervisor,
  ogsAdmin,
  bueroAdmin,
  traegerAdmin,
} from "./permissions.js";

// Create a shared pool for database operations including hooks
const pool = new Pool({
  connectionString: process.env.DATABASE_URL,
});

/**
 * BetterAuth configuration for Project Phoenix multi-tenancy.
 *
 * Entity Mapping:
 * - OGS (after-school center) = Organization
 * - Supervisor/Admin = Member with role
 * - Träger (carrier) = Custom field: traegerId
 * - Büro (office) = Custom field: bueroId (nullable)
 */
export const auth = betterAuth({
  // Base URL - this is the frontend URL that browsers interact with
  // BetterAuth needs this to set cookies correctly since we proxy through Next.js
  // The BetterAuth service runs on localhost:3001 but cookies should be for the frontend
  baseURL: process.env.BETTER_AUTH_BASE_URL ?? "http://localhost:3000",

  // Database connection - uses PostgreSQL with SSL (reuse the shared pool)
  database: pool,

  // Enable email/password authentication
  emailAndPassword: {
    enabled: true,
    // Require email verification for security
    requireEmailVerification: false, // Can enable later when email is configured
    // Password requirements
    minPasswordLength: 8,
  },

  // Trusted origins for CSRF protection
  trustedOrigins: process.env.TRUSTED_ORIGINS?.split(",") ?? [
    "http://localhost:3000",
    "http://localhost:8080",
  ],

  // Session configuration
  session: {
    // Session expires after 7 days
    expiresIn: 60 * 60 * 24 * 7, // 7 days in seconds
    // Refresh session if it will expire in less than 1 day
    updateAge: 60 * 60 * 24, // 1 day in seconds
    // Store session in database for multi-device support
    storeSessionInDatabase: true,
  },

  // Advanced configuration for cookie handling
  advanced: {
    // Set cookie path to "/" so cookies are available to all routes
    // Without this, cookies might only be sent to /api/auth/* routes
    defaultCookieAttributes: {
      path: "/",
      sameSite: "lax",
    },
  },

  // Database hooks for auto-provisioning
  databaseHooks: {
    user: {
      create: {
        // After a new user is created, automatically add them to the default organization
        after: async (user) => {
          const DEFAULT_ORG_ID = "first-ogs-organization";
          const DEFAULT_ROLE = "member";

          try {
            // Check if user is already a member (shouldn't happen, but be safe)
            const existingResult = await pool.query(
              `SELECT id FROM public.member WHERE "userId" = $1 AND "organizationId" = $2`,
              [user.id, DEFAULT_ORG_ID]
            );

            if (existingResult.rows.length === 0) {
              // Add user to default organization
              const memberId = `member-${user.id}-${Date.now()}`;
              await pool.query(
                `INSERT INTO public.member (id, "userId", "organizationId", role, "createdAt")
                 VALUES ($1, $2, $3, $4, NOW())`,
                [memberId, user.id, DEFAULT_ORG_ID, DEFAULT_ROLE]
              );
              console.log(
                `[BetterAuth] Auto-added user ${user.email} to default organization`
              );
            }
          } catch (error) {
            // Log but don't fail the signup - user can be added manually
            console.error(
              `[BetterAuth] Failed to auto-add user to org:`,
              error
            );
          }
        },
      },
    },
  },

  // Plugins
  plugins: [
    // Organization plugin for multi-tenancy
    // Each OGS (after-school center) is an Organization
    organization({
      // Allow users to create organizations (OGS registration)
      allowUserToCreateOrganization: true,

      // Access control configuration for role-based permissions
      // See permissions.ts for role definitions and GDPR compliance notes
      ac,
      roles: {
        supervisor,
        ogsAdmin,
        bueroAdmin,
        traegerAdmin,
      },

      // Custom schema configuration
      schema: {
        organization: {
          // Add custom fields for Phoenix multi-tenancy
          additionalFields: {
            // Träger (carrier) ID - required for each OGS
            // Links organization to a carrier/provider
            traegerId: {
              type: "string",
              required: true,
              input: true, // Accept from API
            },
            // Büro (office) ID - optional
            // Some OGS belong to administrative offices
            bueroId: {
              type: "string",
              required: false,
              input: true, // Accept from API
            },
          },
        },
      },
    }),
  ],
});

// Export types for use in other modules
export type Session = typeof auth.$Infer.Session;
export type User = typeof auth.$Infer.Session.user;
