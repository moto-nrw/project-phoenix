/**
 * Console Invitations API Route
 *
 * Creates invitations via the Go backend's internal API.
 * Uses BetterAuth authentication (not Go backend JWT).
 *
 * This is used by the SaaS admin console to send invitations to users
 * who will manage organizations. The Go backend's internal invitation
 * endpoint is called, which uses the system admin account as the creator.
 *
 * Access Control:
 * - User must be logged in via BetterAuth
 * - User's email must be in SAAS_ADMIN_EMAILS list
 */

import { type NextRequest, NextResponse } from "next/server";
import { verifyAdminAccess } from "~/lib/admin-auth";

// Go backend internal API URL
// In Docker: use internal network hostname (server:8080)
// In local dev: use localhost:8080
// The NEXT_PUBLIC_API_URL is set to the backend URL for the current environment
const GO_BACKEND_INTERNAL_URL =
  process.env.GO_BACKEND_INTERNAL_URL ??
  process.env.NEXT_PUBLIC_API_URL ??
  "http://localhost:8080";

interface CreateInvitationRequest {
  email: string;
  role_id: number;
  first_name?: string;
  last_name?: string;
  position?: string;
}

interface BackendInvitationResponse {
  status: string;
  data?: {
    id: number;
    email: string;
    role_id: number;
    token: string;
    expires_at: string;
    first_name?: string;
    last_name?: string;
    position?: string;
  };
  error?: string;
}

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

    // Parse request body
    const body = (await request.json()) as CreateInvitationRequest;

    // Validate required fields
    if (!body.email) {
      return NextResponse.json({ error: "Email is required" }, { status: 400 });
    }
    if (!body.role_id || body.role_id <= 0) {
      return NextResponse.json(
        { error: "Valid role_id is required" },
        { status: 400 },
      );
    }

    // Call the Go backend's internal invitation endpoint
    // This endpoint doesn't require JWT auth - it's secured via Docker network isolation
    const backendUrl = `${GO_BACKEND_INTERNAL_URL}/api/internal/invitations`;

    console.log(
      `[Console Invitations] Creating invitation for ${body.email} with role ${body.role_id}`,
    );

    const response = await fetch(backendUrl, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        email: body.email,
        role_id: body.role_id,
        first_name: body.first_name,
        last_name: body.last_name,
        position: body.position,
      }),
    });

    const result = (await response.json()) as BackendInvitationResponse;

    if (!response.ok) {
      console.error(
        `[Console Invitations] Backend error: ${response.status}`,
        result,
      );
      return NextResponse.json(
        { error: result.error ?? "Failed to create invitation" },
        { status: response.status },
      );
    }

    console.log(
      `[Console Invitations] Invitation created successfully for ${body.email}`,
    );

    return NextResponse.json({
      status: "success",
      data: result.data,
    });
  } catch (error) {
    console.error("[Console Invitations] API error:", error);
    return NextResponse.json(
      { error: "Failed to create invitation" },
      { status: 500 },
    );
  }
}
