/**
 * BetterAuth Client Configuration
 *
 * This module configures the BetterAuth client for Project Phoenix.
 * It replaces the previous NextAuth.js setup for multi-tenant authentication.
 *
 * Key Changes from NextAuth.js:
 * - No more JWT tokens in session - BetterAuth uses secure cookies
 * - Session contains user info but NOT role/permissions (fetch separately)
 * - Organization plugin handles multi-tenancy (OGS = Organization)
 */

import { createAuthClient } from "better-auth/react";
import { organizationClient } from "better-auth/client/plugins";

/**
 * BetterAuth client instance with organization plugin for multi-tenancy.
 *
 * The client uses same-origin requests (no baseURL) which are proxied through
 * Next.js API routes to the BetterAuth service. This eliminates CORS issues
 * and keeps the auth service URL internal.
 *
 * Flow: Browser → Next.js (/api/auth/*) → BetterAuth service
 *
 * Usage:
 * - Client-side: Use hooks like `useSession()`, `signIn.email()`, etc.
 * - Server-side: Use `getSession()` (async)
 */
export const authClient = createAuthClient({
  // No baseURL = same-origin requests, proxied through Next.js
  plugins: [
    organizationClient(), // Multi-tenant support (OGS = Organization)
  ],
});

// Export commonly used methods for convenience
export const { signIn, signOut, signUp, useSession, getSession, organization } =
  authClient;

/**
 * Session type from BetterAuth.
 *
 * IMPORTANT: The session only includes `activeOrganizationId` (string | null),
 * NOT the full organization object. To get role/permissions, you must call
 * `authClient.organization.getActiveMemberRole()` separately.
 */
export interface BetterAuthSession {
  user: {
    id: string;
    email: string;
    name: string | null;
    emailVerified: boolean;
    image: string | null;
    createdAt: Date;
    updatedAt: Date;
  };
  session: {
    id: string;
    userId: string;
    expiresAt: Date;
    ipAddress: string | null;
    userAgent: string | null;
  };
  // Organization plugin only adds the active org ID, NOT the full object
  activeOrganizationId: string | null;
}

/**
 * Role type returned by getActiveMemberRole().
 * Phoenix roles: supervisor, ogsAdmin, bueroAdmin, traegerAdmin
 */
export type PhoenixRole =
  | "supervisor"
  | "ogsAdmin"
  | "bueroAdmin"
  | "traegerAdmin";

/**
 * Get the current user's role in their active organization.
 * This requires a separate API call - it's NOT included in the session.
 *
 * @returns The role name or null if not in an organization
 *
 * @example
 * ```ts
 * const role = await getActiveRole();
 * if (role === "ogsAdmin" || role === "traegerAdmin") {
 *   // Show admin features
 * }
 * ```
 */
export async function getActiveRole(): Promise<PhoenixRole | null> {
  const { data } = await authClient.organization.getActiveMemberRole({});
  return (data?.role as PhoenixRole) ?? null;
}

/**
 * Check if the current user is an admin (ogsAdmin, bueroAdmin, or traegerAdmin).
 *
 * @returns true if user has admin role
 */
export async function isAdmin(): Promise<boolean> {
  const role = await getActiveRole();
  return (
    role === "ogsAdmin" || role === "bueroAdmin" || role === "traegerAdmin"
  );
}

/**
 * Check if the current user is a supervisor.
 *
 * @returns true if user has supervisor role
 */
export async function isSupervisor(): Promise<boolean> {
  const role = await getActiveRole();
  return role === "supervisor";
}

/**
 * Organization info returned by getFullOrganization().
 */
export interface OrganizationInfo {
  id: string;
  name: string;
  slug: string;
  metadata?: {
    traegerId?: string;
    bueroId?: string;
  };
}

/**
 * Get full organization details for the active organization.
 * The session only contains the ID - use this to get name, slug, etc.
 *
 * @param organizationId - The organization ID (from session.activeOrganizationId)
 * @returns Organization details or null if not found
 */
export async function getOrganizationInfo(
  organizationId: string,
): Promise<OrganizationInfo | null> {
  const { data } = await authClient.organization.getFullOrganization({
    query: { organizationId },
  });

  if (!data) return null;

  return {
    id: data.id,
    name: data.name,
    slug: data.slug,
    metadata: data.metadata as OrganizationInfo["metadata"],
  };
}

/**
 * Switch the active organization (for users with multiple OGS).
 * After switching, the page should be refreshed to update all data.
 *
 * @param organizationId - The new organization ID to switch to
 */
export async function switchOrganization(
  organizationId: string,
): Promise<void> {
  await authClient.organization.setActive({ organizationId });
}

/**
 * Request payload for atomic signup with organization.
 */
export interface SignupWithOrgRequest {
  name: string;
  email: string;
  password: string;
  orgName: string;
  orgSlug: string;
}

/**
 * Response from atomic signup with organization.
 */
export interface SignupWithOrgResponse {
  success: boolean;
  user: {
    id: string;
    email: string;
    name: string;
    emailVerified: boolean;
    createdAt: string;
  };
  organization: {
    id: string;
    name: string;
    slug: string;
    status: string;
  };
  session: {
    token: string;
    expiresAt: string;
  };
}

/**
 * Error response from atomic signup.
 */
export interface SignupWithOrgError {
  error: string;
  code?: string;
  field?: string;
}

/**
 * Atomic signup with organization creation.
 * Creates user + organization in a single atomic transaction.
 * If slug is taken or email is registered, nothing is created.
 *
 * @returns The created user and organization, or throws an error
 */
export async function signupWithOrganization(
  data: SignupWithOrgRequest,
): Promise<SignupWithOrgResponse> {
  const response = await fetch("/api/auth/signup-with-org", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  });

  const result = (await response.json()) as
    | SignupWithOrgResponse
    | SignupWithOrgError;

  if (!response.ok) {
    const errorResult = result as SignupWithOrgError;
    throw new SignupWithOrgException(
      errorResult.error,
      errorResult.code,
      errorResult.field,
      response.status,
    );
  }

  return result as SignupWithOrgResponse;
}

/**
 * Custom error class for signup failures with structured error info.
 */
export class SignupWithOrgException extends Error {
  code?: string;
  field?: string;
  status: number;

  constructor(message: string, code?: string, field?: string, status = 500) {
    super(message);
    this.name = "SignupWithOrgException";
    this.code = code;
    this.field = field;
    this.status = status;
  }
}
