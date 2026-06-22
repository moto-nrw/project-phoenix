import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { apiDelete, handleApiError } from "~/lib/api-helpers.server";
import { auth } from "~/server/auth";

interface RouteContext {
  params: Promise<{ filename: string }>;
}

export async function DELETE(_request: NextRequest, context: RouteContext) {
  const session = await auth();
  const token = session?.user?.token;
  if (!token) {
    return NextResponse.json({ error: "Unauthenticated" }, { status: 401 });
  }

  const { filename } = await context.params;
  if (!filename) {
    return NextResponse.json({ error: "Invalid filename" }, { status: 400 });
  }

  try {
    await apiDelete(
      `/api/enrollment/legal-documents/${encodeURIComponent(filename)}`,
      token,
    );
    return new NextResponse(null, { status: 204 });
  } catch (error) {
    return handleApiError(error);
  }
}
