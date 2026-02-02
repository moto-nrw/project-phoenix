import { type NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";

// Error response interface
interface ErrorResponse {
  error: string;
}

// POST: Assign a permission to a role
export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ roleId: string; permissionId: string }> },
) {
  try {
    const resolvedParams = await params;
    const { roleId, permissionId } = resolvedParams;

    if (!roleId || !permissionId) {
      return NextResponse.json(
        { error: "Role ID and Permission ID are required" } as ErrorResponse,
        { status: 400 },
      );
    }

    const session = await auth();
    if (!session?.user?.token) {
      return NextResponse.json({ error: "Unauthorized" } as ErrorResponse, {
        status: 401,
      });
    }

    const url = `${getServerApiUrl()}/auth/roles/${roleId}/permissions/${permissionId}`;

    const response = await fetch(url, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${session.user.token}`,
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      const errorText = await response.text();
      console.error(
        `Assign permission to role error: ${response.status}`,
        errorText,
      );
      return NextResponse.json(
        {
          error: errorText || `Failed to assign permission: ${response.status}`,
        } as ErrorResponse,
        { status: response.status },
      );
    }

    return NextResponse.json({ success: true });
  } catch (error) {
    console.error("Assign permission to role route error:", error);
    return NextResponse.json(
      { error: "Internal Server Error" } as ErrorResponse,
      { status: 500 },
    );
  }
}

// DELETE: Remove a permission from a role
export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ roleId: string; permissionId: string }> },
) {
  try {
    const resolvedParams = await params;
    const { roleId, permissionId } = resolvedParams;

    if (!roleId || !permissionId) {
      return NextResponse.json(
        { error: "Role ID and Permission ID are required" } as ErrorResponse,
        { status: 400 },
      );
    }

    const session = await auth();
    if (!session?.user?.token) {
      return NextResponse.json({ error: "Unauthorized" } as ErrorResponse, {
        status: 401,
      });
    }

    const url = `${getServerApiUrl()}/auth/roles/${roleId}/permissions/${permissionId}`;

    const response = await fetch(url, {
      method: "DELETE",
      headers: {
        Authorization: `Bearer ${session.user.token}`,
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      const errorText = await response.text();
      console.error(
        `Remove permission from role error: ${response.status}`,
        errorText,
      );
      return NextResponse.json(
        {
          error: errorText || `Failed to remove permission: ${response.status}`,
        } as ErrorResponse,
        { status: response.status },
      );
    }

    return NextResponse.json({ success: true });
  } catch (error) {
    console.error("Remove permission from role route error:", error);
    return NextResponse.json(
      { error: "Internal Server Error" } as ErrorResponse,
      { status: 500 },
    );
  }
}
