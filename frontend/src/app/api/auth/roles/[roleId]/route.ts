import { type NextRequest, NextResponse } from "next/server";
import { auth, getCookieHeader } from "~/server/auth";
import { env } from "~/env";

// Define interface for Role based on backend models
interface Permission {
  id: number;
  created_at: string;
  updated_at: string;
  name: string;
  description: string;
  resource: string;
  action: string;
}

interface Role {
  id: number;
  created_at: string;
  updated_at: string;
  name: string;
  description: string;
  permissions?: Permission[];
}

// Response interfaces
interface RoleResponse {
  data: Role;
}

interface ErrorResponse {
  error: string;
}

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ roleId: string }> },
) {
  try {
    const resolvedParams = await params;
    const roleId = resolvedParams.roleId;
    if (!roleId) {
      return NextResponse.json(
        { error: "Role ID is required" } as ErrorResponse,
        { status: 400 },
      );
    }

    const session = await auth();
    if (!session?.user) {
      return NextResponse.json({ error: "Unauthorized" } as ErrorResponse, {
        status: 401,
      });
    }

    const cookieHeader = await getCookieHeader();
    const url = `${env.NEXT_PUBLIC_API_URL}/auth/roles/${roleId}`;

    const response = await fetch(url, {
      headers: {
        Cookie: cookieHeader,
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      const errorText = await response.text();
      console.error(`Get role error: ${response.status}`, errorText);
      return NextResponse.json({ error: errorText } as ErrorResponse, {
        status: response.status,
      });
    }

    const data = (await response.json()) as RoleResponse;
    return NextResponse.json(data);
  } catch (error) {
    console.error("Get role route error:", error);
    return NextResponse.json(
      { error: "Internal Server Error" } as ErrorResponse,
      { status: 500 },
    );
  }
}

export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ roleId: string }> },
) {
  try {
    const resolvedParams = await params;
    const roleId = resolvedParams.roleId;
    if (!roleId) {
      return NextResponse.json(
        { error: "Role ID is required" } as ErrorResponse,
        { status: 400 },
      );
    }

    const session = await auth();
    if (!session?.user) {
      return NextResponse.json({ error: "Unauthorized" } as ErrorResponse, {
        status: 401,
      });
    }

    const cookieHeader = await getCookieHeader();
    const body = (await request.json()) as unknown;
    const url = `${env.NEXT_PUBLIC_API_URL}/auth/roles/${roleId}`;

    const response = await fetch(url, {
      method: "PUT",
      headers: {
        Cookie: cookieHeader,
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      const errorText = await response.text();
      console.error(`Update role error: ${response.status}`, errorText);
      return NextResponse.json({ error: errorText } as ErrorResponse, {
        status: response.status,
      });
    }

    return NextResponse.json({ success: true });
  } catch (error) {
    console.error("Update role route error:", error);
    return NextResponse.json(
      { error: "Internal Server Error" } as ErrorResponse,
      { status: 500 },
    );
  }
}

export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ roleId: string }> },
) {
  try {
    const resolvedParams = await params;
    const roleId = resolvedParams.roleId;
    if (!roleId) {
      return NextResponse.json(
        { error: "Role ID is required" } as ErrorResponse,
        { status: 400 },
      );
    }

    const session = await auth();
    if (!session?.user) {
      return NextResponse.json({ error: "Unauthorized" } as ErrorResponse, {
        status: 401,
      });
    }

    const cookieHeader = await getCookieHeader();
    const url = `${env.NEXT_PUBLIC_API_URL}/auth/roles/${roleId}`;

    const response = await fetch(url, {
      method: "DELETE",
      headers: {
        Cookie: cookieHeader,
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      const errorText = await response.text();
      console.error(`Delete role error: ${response.status}`, errorText);
      return NextResponse.json({ error: errorText } as ErrorResponse, {
        status: response.status,
      });
    }

    return NextResponse.json({ success: true });
  } catch (error) {
    console.error("Delete role route error:", error);
    return NextResponse.json(
      { error: "Internal Server Error" } as ErrorResponse,
      { status: 500 },
    );
  }
}
