/**
 * Admin Organizations API Route
 *
 * Proxies requests to BetterAuth admin endpoints for organization management.
 * GET /api/admin/organizations - List all organizations (with optional ?status=pending filter)
 */

import { type NextRequest, NextResponse } from "next/server";

const BETTERAUTH_INTERNAL_URL =
  process.env.BETTERAUTH_INTERNAL_URL ?? "http://localhost:3001";

export async function GET(request: NextRequest): Promise<NextResponse> {
  try {
    const { searchParams } = new URL(request.url);
    const status = searchParams.get("status");

    const targetUrl = new URL(
      `${BETTERAUTH_INTERNAL_URL}/api/admin/organizations`,
    );
    if (status) {
      targetUrl.searchParams.set("status", status);
    }

    // Forward cookies for session auth
    const cookies = request.headers.get("Cookie");

    const response = await fetch(targetUrl.toString(), {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        ...(cookies ? { Cookie: cookies } : {}),
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
