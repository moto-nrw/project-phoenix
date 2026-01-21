/**
 * Admin Organization Approve API Route
 * POST /api/admin/organizations/:orgId/approve
 */

import { type NextRequest, NextResponse } from "next/server";
import { verifyAdminAccess, INTERNAL_API_KEY } from "~/lib/admin-auth";

const BETTERAUTH_INTERNAL_URL =
  process.env.BETTERAUTH_INTERNAL_URL ?? "http://localhost:3001";

export async function POST(
  request: NextRequest,
  context: { params: Promise<{ orgId: string }> },
): Promise<NextResponse> {
  try {
    // Verify admin access
    const adminSession = await verifyAdminAccess(request);
    if (!adminSession) {
      return NextResponse.json(
        { error: "Unauthorized - admin access required" },
        { status: 401 },
      );
    }

    const { orgId } = await context.params;

    const response = await fetch(
      `${BETTERAUTH_INTERNAL_URL}/api/admin/organizations/${encodeURIComponent(orgId)}/approve`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Internal-API-Key": INTERNAL_API_KEY,
        },
        body: JSON.stringify({}),
      },
    );

    const data: unknown = await response.json();

    if (!response.ok) {
      return NextResponse.json(data as Record<string, unknown>, {
        status: response.status,
      });
    }

    return NextResponse.json(data as Record<string, unknown>);
  } catch (error) {
    console.error("Admin approve API error:", error);
    return NextResponse.json(
      { error: "Failed to approve organization" },
      { status: 500 },
    );
  }
}
