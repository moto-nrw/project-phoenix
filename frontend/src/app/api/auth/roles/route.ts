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
interface RolesResponse {
  roles: Role[];
  total?: number;
  page?: number;
  limit?: number;
}

interface ErrorResponse {
  error: string;
}

// Request interface for creating a role
interface CreateRoleRequest {
  name: string;
  description?: string;
  permissions?: number[]; // Permission IDs to associate with the role
}

export async function GET(request: NextRequest) {
  try {
    const session = await auth();

    if (!session?.user) {
      return NextResponse.json({ error: "Unauthorized" } as ErrorResponse, {
        status: 401,
      });
    }

    const cookieHeader = await getCookieHeader();
    const url = new URL(`${env.NEXT_PUBLIC_API_URL}/auth/roles`);
    const searchParams = request.nextUrl.searchParams;

    searchParams.forEach((value, key) => {
      url.searchParams.append(key, value);
    });

    const response = await fetch(url.toString(), {
      headers: {
        Cookie: cookieHeader,
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      const errorText = await response.text();
      return NextResponse.json({ error: errorText } as ErrorResponse, {
        status: response.status,
      });
    }

    const data = (await response.json()) as RolesResponse;
    return NextResponse.json(data);
  } catch (error) {
    console.error("Get roles route error:", error);
    return NextResponse.json(
      { error: "Internal Server Error" } as ErrorResponse,
      { status: 500 },
    );
  }
}

export async function POST(request: NextRequest) {
  try {
    const session = await auth();

    if (!session?.user) {
      return NextResponse.json({ error: "Unauthorized" } as ErrorResponse, {
        status: 401,
      });
    }

    const cookieHeader = await getCookieHeader();
    const body = (await request.json()) as CreateRoleRequest;

    const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/auth/roles`, {
      method: "POST",
      headers: {
        Cookie: cookieHeader,
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      const errorText = await response.text();
      return NextResponse.json({ error: errorText } as ErrorResponse, {
        status: response.status,
      });
    }

    const data = (await response.json()) as Role;
    return NextResponse.json(data);
  } catch (error) {
    console.error("Create role route error:", error);
    return NextResponse.json(
      { error: "Internal Server Error" } as ErrorResponse,
      { status: 500 },
    );
  }
}
