// lib/api-helpers.server.ts
// Server-only API helpers - DO NOT import from client components
import { NextResponse } from "next/server";
import { auth } from "../server/auth";
import type { ApiErrorResponse } from "./api-helpers";

/**
 * Check if the current session is authenticated
 * SERVER-ONLY: This function uses BetterAuth's auth() which requires server context
 * @returns NextResponse with error if not authenticated, null if authenticated
 */
export async function checkAuth(): Promise<NextResponse<ApiErrorResponse> | null> {
  const session = await auth();

  if (!session?.user) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  return null;
}
