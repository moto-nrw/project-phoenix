/**
 * Admin Organization Provisioning API Route
 *
 * Atomic endpoint that creates an organization AND its invitations together.
 * If any validation fails (slug taken, emails registered), nothing is created.
 *
 * POST /api/admin/organizations/provision
 * Body:
 * {
 *   orgName: string,
 *   orgSlug: string,
 *   invitations: Array<{
 *     email: string,
 *     role: 'admin' | 'member' | 'owner',
 *     firstName?: string,
 *     lastName?: string
 *   }>
 * }
 *
 * Access Control:
 * - User must be logged in via BetterAuth
 * - User's email must be in SAAS_ADMIN_EMAILS list
 */

import { type NextRequest, NextResponse } from "next/server";
import { verifyAdminAccess, INTERNAL_API_KEY } from "~/lib/admin-auth";

const BETTERAUTH_INTERNAL_URL =
  process.env.BETTERAUTH_INTERNAL_URL ?? "http://localhost:3001";

interface ProvisionInvitation {
  email: string;
  role: "admin" | "member" | "owner";
  firstName?: string;
  lastName?: string;
}

interface ProvisionRequestBody {
  orgName?: string;
  orgSlug?: string;
  invitations?: ProvisionInvitation[];
}

interface ErrorResponse {
  error: string;
  field?: string;
  unavailableEmails?: string[];
}

interface SuccessResponse {
  success: boolean;
  organization: {
    id: string;
    name: string;
    slug: string;
    status: string;
    createdAt: string;
  };
  invitations: Array<{
    id: string;
    email: string;
    role: string;
  }>;
}

/**
 * Atomic organization provisioning.
 * Creates organization AND invitations in a single atomic operation.
 * Validates all inputs before creating anything.
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

    const body = (await request.json()) as ProvisionRequestBody;

    // Client-side validation for better error messages
    if (!body.orgName?.trim()) {
      return NextResponse.json(
        { error: "Organization name is required", field: "orgName" },
        { status: 400 },
      );
    }

    if (!body.orgSlug?.trim()) {
      return NextResponse.json(
        { error: "Organization slug is required", field: "orgSlug" },
        { status: 400 },
      );
    }

    if (!body.invitations || body.invitations.length === 0) {
      return NextResponse.json(
        { error: "At least one invitation is required", field: "invitations" },
        { status: 400 },
      );
    }

    // Forward to BetterAuth provision endpoint
    const response = await fetch(
      `${BETTERAUTH_INTERNAL_URL}/api/admin/organizations/provision`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Internal-API-Key": INTERNAL_API_KEY,
        },
        body: JSON.stringify({
          orgName: body.orgName.trim(),
          orgSlug: body.orgSlug.trim(),
          invitations: body.invitations,
        }),
      },
    );

    const data = (await response.json()) as ErrorResponse | SuccessResponse;

    if (!response.ok) {
      return NextResponse.json(data, { status: response.status });
    }

    return NextResponse.json(data, { status: 201 });
  } catch (error) {
    console.error("Admin provision organization API error:", error);
    return NextResponse.json(
      { error: "Failed to provision organization" },
      { status: 500 },
    );
  }
}
