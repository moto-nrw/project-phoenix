import { type NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "PermissionDetailRoute" });

// Backend model type (lowercase fields)
interface BackendPermission {
  id: number;
  name: string;
  description: string;
  resource: string;
  action: string;
  created_at: string;
  updated_at: string;
}

interface ApiResponse<T> {
  data: T;
  status?: string;
  message?: string;
}
interface ErrorResponse {
  error: string;
}

export async function GET(
  _request: NextRequest,
  { params }: { params: Promise<{ permissionId: string }> },
) {
  try {
    const { permissionId } = await params;
    if (!permissionId) {
      return NextResponse.json(
        { error: "Permission ID is required" } as ErrorResponse,
        { status: 400 },
      );
    }

    const session = await auth();
    if (!session?.user?.token) {
      return NextResponse.json({ error: "Unauthorized" } as ErrorResponse, {
        status: 401,
      });
    }

    const url = `${getServerApiUrl()}/auth/permissions/${permissionId}`;
    const response = await fetch(url, {
      headers: {
        Authorization: `Bearer ${session.user.token}`,
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      const errorText = await response.text();
      return NextResponse.json({ error: errorText } as ErrorResponse, {
        status: response.status,
      });
    }

    const data = (await response.json()) as ApiResponse<BackendPermission>;
    return NextResponse.json(data);
  } catch (error) {
    logger.error("get permission failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" } as ErrorResponse,
      { status: 500 },
    );
  }
}

export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ permissionId: string }> },
) {
  try {
    const { permissionId } = await params;
    if (!permissionId) {
      return NextResponse.json(
        { error: "Permission ID is required" } as ErrorResponse,
        { status: 400 },
      );
    }

    const session = await auth();
    if (!session?.user?.token) {
      return NextResponse.json({ error: "Unauthorized" } as ErrorResponse, {
        status: 401,
      });
    }

    const body = (await request.json()) as unknown;
    const url = `${getServerApiUrl()}/auth/permissions/${permissionId}`;
    const response = await fetch(url, {
      method: "PUT",
      headers: {
        Authorization: `Bearer ${session.user.token}`,
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

    // Forward backend JSON (expected to contain the updated permission)
    const data = (await response.json()) as
      | ApiResponse<BackendPermission>
      | BackendPermission;
    return NextResponse.json(data);
  } catch (error) {
    logger.error("update permission failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" } as ErrorResponse,
      { status: 500 },
    );
  }
}

export async function DELETE(
  _request: NextRequest,
  { params }: { params: Promise<{ permissionId: string }> },
) {
  try {
    const { permissionId } = await params;
    if (!permissionId) {
      return NextResponse.json(
        { error: "Permission ID is required" } as ErrorResponse,
        { status: 400 },
      );
    }

    const session = await auth();
    if (!session?.user?.token) {
      return NextResponse.json({ error: "Unauthorized" } as ErrorResponse, {
        status: 401,
      });
    }

    const url = `${getServerApiUrl()}/auth/permissions/${permissionId}`;
    const response = await fetch(url, {
      method: "DELETE",
      headers: {
        Authorization: `Bearer ${session.user.token}`,
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      const errorText = await response.text();
      return NextResponse.json({ error: errorText } as ErrorResponse, {
        status: response.status,
      });
    }

    // Mirror a 204-style success so upstream code won't try to map a body
    return new NextResponse(null, { status: 204 });
  } catch (error) {
    logger.error("delete permission failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" } as ErrorResponse,
      { status: 500 },
    );
  }
}
