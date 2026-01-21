/**
 * Check if current user is a SaaS admin
 * GET /api/auth/check-saas-admin
 *
 * Returns: { isSaasAdmin: boolean }
 */

import { type NextRequest, NextResponse } from "next/server";

const BETTERAUTH_INTERNAL_URL =
  process.env.BETTERAUTH_INTERNAL_URL ?? "http://localhost:3001";

// SaaS admin emails (comma-separated in env)
const SAAS_ADMIN_EMAILS = (process.env.SAAS_ADMIN_EMAILS ?? "admin@example.com")
  .split(",")
  .map((e) => e.trim().toLowerCase());

export async function GET(request: NextRequest): Promise<NextResponse> {
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
      return NextResponse.json({ isSaasAdmin: false });
    }

    const session = (await sessionResponse.json()) as {
      user?: { email?: string };
    } | null;

    if (!session?.user?.email) {
      return NextResponse.json({ isSaasAdmin: false });
    }

    const email = session.user.email.toLowerCase();
    const isSaasAdmin = SAAS_ADMIN_EMAILS.includes(email);

    return NextResponse.json({ isSaasAdmin });
  } catch (error) {
    console.error("Failed to check SaaS admin status:", error);
    return NextResponse.json({ isSaasAdmin: false });
  }
}
