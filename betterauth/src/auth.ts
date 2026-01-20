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
  // Database connection - uses PostgreSQL with SSL
  database: new Pool({
    connectionString: process.env.DATABASE_URL,
    // SSL configuration is handled via connection string (sslmode=require)
  }),

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
