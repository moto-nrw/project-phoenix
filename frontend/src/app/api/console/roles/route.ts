/**
 * Console Roles API Route
 *
 * Returns the system roles for the SaaS admin console.
 * Uses BetterAuth authentication (not Go backend JWT).
 *
 * The roles are static system roles from the Go backend `auth.roles` table:
 * - admin: System administrator with full access
 * - user: Standard user with basic permissions (Betreuer/Nutzer)
 * - guest: Limited access (Gast)
 *
 * Access Control:
 * - User must be logged in via BetterAuth
 * - User's email must be in SAAS_ADMIN_EMAILS list
 */

import { type NextRequest, NextResponse } from "next/server";
import { verifyAdminAccess } from "~/lib/admin-auth";

// Static system roles from Go backend auth.roles table
// These are seeded in migration 001000004_auth_roles.go
const SYSTEM_ROLES = [
  {
    id: 1,
    name: "admin",
    description: "System administrator with full access",
    displayName: "Administrator",
  },
  {
    id: 2,
    name: "user",
    description: "Standard user with basic permissions",
    displayName: "Nutzer",
  },
  {
    id: 3,
    name: "guest",
    description: "Limited access for unauthenticated users",
    displayName: "Gast",
  },
];

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

    return NextResponse.json({
      status: "success",
      data: SYSTEM_ROLES,
    });
  } catch (error) {
    console.error("Console roles API error:", error);
    return NextResponse.json(
      { error: "Failed to fetch roles" },
      { status: 500 },
    );
  }
}
