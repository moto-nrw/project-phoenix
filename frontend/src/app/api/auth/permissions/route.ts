import { type NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "PermissionsRoute" });

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

export async function GET(request: NextRequest) {
  try {
    const session = await auth();
    if (!session?.user?.token) {
      return NextResponse.json({ error: "Unauthorized" } as ErrorResponse, {
        status: 401,
      });
    }

    const searchParams = request.nextUrl.searchParams;
    const resource = searchParams.get("resource");
    const action = searchParams.get("action");

    const url = new URL(`${getServerApiUrl()}/auth/permissions`);
    if (resource) url.searchParams.append("resource", resource);
    if (action) url.searchParams.append("action", action);

    const response = await fetch(url.toString(), {
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

    const data = (await response.json()) as
      | ApiResponse<BackendPermission[]>
      | BackendPermission[];
    return NextResponse.json(data);
  } catch (error) {
    logger.error("get permissions failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" } as ErrorResponse,
      { status: 500 },
    );
  }
}

export async function POST(request: NextRequest) {
  try {
    const session = await auth();
    if (!session?.user?.token) {
      return NextResponse.json({ error: "Unauthorized" } as ErrorResponse, {
        status: 401,
      });
    }

    const body = (await request.json()) as Partial<BackendPermission>;
    const response = await fetch(`${getServerApiUrl()}/auth/permissions`, {
      method: "POST",
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

    const data = (await response.json()) as ApiResponse<BackendPermission>;
    return NextResponse.json(data);
  } catch (error) {
    logger.error("create permission failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" } as ErrorResponse,
      { status: 500 },
    );
  }
}
