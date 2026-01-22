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
import {
  sendOrgPendingEmail,
  sendOrgInvitationEmail,
  syncUserToGoBackend,
} from "./email.js";

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
 * - Träger (carrier) = Custom field: traegerId (optional, assigned by SaaS admin)
 * - Büro (office) = Custom field: bueroId (optional)
 *
 * Self-Service SaaS Model:
 * - Users create their own organization during signup
 * - Organizations start with status: "pending" (requires SaaS admin approval)
 * - Members can have status: "pending" (requires org admin approval if configured)
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

  // NOTE: Auto-add to default org removed for self-service SaaS model
  // Users now create their own organization during signup

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

      // Send invitation emails when inviteMember is called
      async sendInvitationEmail(data) {
        // Get organization slug for subdomain
        const orgSlug = (data.organization as { slug?: string }).slug ?? "";

        await sendOrgInvitationEmail({
          to: data.email,
          orgName: data.organization.name,
          subdomain: orgSlug,
          invitationId: data.id,
          role: data.role,
          // Note: firstName/lastName passed via URL params from frontend
        });
      },

      // Organization lifecycle hooks for email notifications and user sync
      organizationHooks: {
        // Send org-pending email after organization creation
        afterCreateOrganization: async ({ organization, user }) => {
          // Only send email for new pending organizations
          if (organization.status === "pending") {
            try {
              await sendOrgPendingEmail({
                to: user.email,
                firstName: user.name?.split(" ")[0] ?? undefined,
                orgName: organization.name,
                subdomain: organization.slug ?? "",
              });
            } catch (error) {
              // Log error but don't fail the organization creation
              console.error("Failed to send org-pending email:", error);
            }
          }
        },

        // Sync user to Go backend after accepting invitation
        // Creates Person, Staff, and Teacher records
        afterAcceptInvitation: async ({ member, user, organization }) => {
          try {
            await syncUserToGoBackend({
              betterauthUserId: user.id,
              email: user.email,
              name: user.name ?? user.email.split("@")[0],
              organizationId: organization.id,
              role: member.role,
            });
          } catch (error) {
            // Log error but don't fail the invitation acceptance
            // User is already added to the org in BetterAuth - Go sync can be retried later
            console.error("Failed to sync user to Go backend:", error);
          }
        },
      },

      // Custom schema configuration for self-service SaaS
      schema: {
        organization: {
          additionalFields: {
            // Organization status for approval workflow
            // pending: awaiting SaaS admin approval
            // active: approved and operational
            // suspended: disabled by SaaS admin
            status: {
              type: "string",
              required: true,
              defaultValue: "pending",
              input: false, // Only changeable via admin API
            },
            // Unique subdomain slug for tenant routing
            // e.g., "ogs-musterstadt" for ogs-musterstadt.moto-app.de
            slug: {
              type: "string",
              required: true,
              input: true,
            },
            // Träger (carrier) ID - optional, assigned by SaaS admin after approval
            traegerId: {
              type: "string",
              required: false, // Changed: now optional for self-service signup
              input: true,
            },
            // Büro (office) ID - optional
            bueroId: {
              type: "string",
              required: false,
              input: true,
            },
            // Organization settings for member management
            // Allow public signup on org subdomain
            allowPublicSignup: {
              type: "boolean",
              required: false,
              defaultValue: false,
              input: true,
            },
            // Require org admin approval for new members
            requireMemberApproval: {
              type: "boolean",
              required: false,
              defaultValue: true,
              input: true,
            },
          },
        },
        member: {
          additionalFields: {
            // Member status for approval workflow
            // pending: awaiting org admin approval (if requireMemberApproval)
            // active: approved member
            // suspended: disabled by org admin
            status: {
              type: "string",
              required: true,
              defaultValue: "active", // Org creator is active, others depend on settings
              input: false,
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
