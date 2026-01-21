/**
 * Admin Organization Reject API Route
 * POST /api/admin/organizations/:orgId/reject
 */

import { type NextRequest, NextResponse } from "next/server";

const BETTERAUTH_INTERNAL_URL =
  process.env.BETTERAUTH_INTERNAL_URL ?? "http://localhost:3001";

export async function POST(
  request: NextRequest,
  context: { params: Promise<{ orgId: string }> },
): Promise<NextResponse> {
  try {
    const { orgId } = await context.params;
    const cookies = request.headers.get("Cookie");
    const body: unknown = await request.json().catch(() => ({}));

    const response = await fetch(
      `${BETTERAUTH_INTERNAL_URL}/api/admin/organizations/${encodeURIComponent(orgId)}/reject`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(cookies ? { Cookie: cookies } : {}),
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

    return NextResponse.json(data as Record<string, unknown>);
  } catch (error) {
    console.error("Admin reject API error:", error);
    return NextResponse.json(
      { error: "Failed to reject organization" },
      { status: 500 },
    );
  }
}
