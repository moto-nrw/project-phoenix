/**
 * Admin Organizations API Route
 *
 * Proxies requests to BetterAuth admin endpoints for organization management.
 * GET /api/admin/organizations - List all organizations (with optional ?status=pending filter)
 * POST /api/admin/organizations - Create organization with active status (for SaaS admin console)
 *
 * Access Control:
 * - User must be logged in via BetterAuth
 * - User's email must be in SAAS_ADMIN_EMAILS list
 */

import { type NextRequest, NextResponse } from "next/server";
import { verifyAdminAccess, INTERNAL_API_KEY } from "~/lib/admin-auth";

const BETTERAUTH_INTERNAL_URL =
  process.env.BETTERAUTH_INTERNAL_URL ?? "http://localhost:3001";

export async function GET(request: NextRequest): Promise<NextResponse> {
  try {
    // Verify admin access
    const adminSession = await verifyAdminAccess(request);
    if (!adminSession) {
      return NextResponse.json(
        { error: "Unauthorized - admin access required" },
        { status: 401 },
      );
    }

    const { searchParams } = new URL(request.url);
    const status = searchParams.get("status");

    const targetUrl = new URL(
      `${BETTERAUTH_INTERNAL_URL}/api/admin/organizations`,
    );
    if (status) {
      targetUrl.searchParams.set("status", status);
    }

    const response = await fetch(targetUrl.toString(), {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        "X-Internal-API-Key": INTERNAL_API_KEY,
      },
    });

    const data: unknown = await response.json();

    if (!response.ok) {
      return NextResponse.json(data as Record<string, unknown>, {
        status: response.status,
      });
    }

    return NextResponse.json(data as Record<string, unknown>);
  } catch (error) {
    console.error("Admin organizations API error:", error);
    return NextResponse.json(
      { error: "Failed to fetch organizations" },
      { status: 500 },
    );
  }
}

/**
 * Create a new organization with active status (for SaaS admin console).
 * The organization is created without an owner - the first invited admin will manage it.
 */
export async function POST(request: NextRequest): Promise<NextResponse> {
  try {
    // Verify admin access
    const adminSession = await verifyAdminAccess(request);
    if (!adminSession) {
      return NextResponse.json(
        { error: "Unauthorized - admin access required" },
        { status: 401 },
      );
    }

    const body = (await request.json()) as { name?: string; slug?: string };

    const response = await fetch(
      `${BETTERAUTH_INTERNAL_URL}/api/admin/organizations`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Internal-API-Key": INTERNAL_API_KEY,
        },
        body: JSON.stringify(body),
      },
    );

    const data: unknown = await response.json();

    if (!response.ok) {
      return NextResponse.json(data as Record<string, unknown>, {
        status: response.status,
      });
    }

    return NextResponse.json(data as Record<string, unknown>, { status: 201 });
  } catch (error) {
    console.error("Admin create organization API error:", error);
    return NextResponse.json(
      { error: "Failed to create organization" },
      { status: 500 },
    );
  }
}
