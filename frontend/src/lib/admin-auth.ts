/**
 * Admin authentication helpers for SaaS admin dashboard.
 *
 * Verifies that the user is logged in via BetterAuth and their email
 * is in the SAAS_ADMIN_EMAILS list.
 */

import type { NextRequest } from "next/server";

const BETTERAUTH_INTERNAL_URL =
  process.env.BETTERAUTH_INTERNAL_URL ?? "http://localhost:3001";

// Internal API key for server-to-server calls to BetterAuth
export const INTERNAL_API_KEY =
  process.env.INTERNAL_API_KEY ?? "dev-internal-key";

// SaaS admin emails (comma-separated in env)
export const SAAS_ADMIN_EMAILS = (
  process.env.SAAS_ADMIN_EMAILS ?? "admin@example.com"
)
  .split(",")
  .map((e) => e.trim().toLowerCase());

export interface AdminSession {
  email: string;
  userId: string;
  name: string | null;
}

/**
 * Verify the user is logged in and is a SaaS admin.
 * Returns the admin session if authorized, null otherwise.
 */
export async function verifyAdminAccess(
  request: NextRequest,
): Promise<AdminSession | null> {
  try {
    // Get session from BetterAuth
    const cookies = request.headers.get("Cookie");
    const sessionResponse = await fetch(
      `${BETTERAUTH_INTERNAL_URL}/api/auth/get-session`,
      {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
          ...(cookies ? { Cookie: cookies } : {}),
        },
      },
    );

    if (!sessionResponse.ok) {
      return null;
    }

    const session = (await sessionResponse.json()) as {
      user?: { id?: string; email?: string; name?: string | null };
    } | null;

    if (!session?.user?.email || !session?.user?.id) {
      return null;
    }

    const email = session.user.email.toLowerCase();

    // Check if user is a SaaS admin
    if (!SAAS_ADMIN_EMAILS.includes(email)) {
      return null;
    }

    return {
      email,
      userId: session.user.id,
      name: session.user.name ?? null,
    };
  } catch (error) {
    console.error("Failed to verify admin access:", error);
    return null;
  }
}
