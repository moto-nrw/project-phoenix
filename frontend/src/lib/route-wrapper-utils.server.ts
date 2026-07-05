import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import type { ApiErrorResponse, ApiResponse } from "./api-helpers";

export type RouteContext = {
  params: Promise<Record<string, string | string[] | undefined>>;
};

export function buildQueryString(request: NextRequest): string {
  const queryParams = new URLSearchParams();
  request.nextUrl.searchParams.forEach((value, key) => {
    queryParams.append(key, value);
  });
  const qs = queryParams.toString();
  return qs ? `?${qs}` : "";
}

export async function extractParams(
  request: NextRequest,
  context: RouteContext,
): Promise<Record<string, unknown>> {
  const safeParams: Record<string, unknown> = {};
  const contextParams = await context.params;
  if (contextParams) {
    Object.entries(contextParams).forEach(([key, value]) => {
      if (value !== undefined) {
        safeParams[key] = value;
      }
    });
  }
  const url = new URL(request.url);
  if (!safeParams.id) {
    const potentialIds = url.pathname.split("/").filter((p) => /^\d+$/.test(p));
    if (potentialIds.length > 0) {
      safeParams.id = potentialIds.at(-1);
    }
  }
  url.searchParams.forEach((value, key) => {
    safeParams[key] = value;
  });
  return safeParams;
}

export async function parseRequestBody<B>(request: NextRequest): Promise<B> {
  const text = await request.text();
  if (!text) {
    return {} as B;
  }
  return JSON.parse(text) as B;
}

export function wrapInApiResponse<T>(data: T): ApiResponse<T> {
  if (typeof data === "object" && data !== null && "success" in data) {
    return data as unknown as ApiResponse<T>;
  }
  return { success: true, message: "Success", data };
}

export function createUnauthorizedResponse(): NextResponse<ApiErrorResponse> {
  return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
}

export function isStringParam(param: unknown): param is string {
  return typeof param === "string";
}

/**
 * Returns a required string route param, throwing "Invalid {key} parameter"
 * (→ 500 via handleApiError) when it is missing or not a string. This does not
 * URL-encode the value; use requirePathSegmentParam for path interpolation.
 */
export function requireStringParam(
  params: Record<string, unknown>,
  key = "id",
): string {
  const value = params[key];
  if (!isStringParam(value)) {
    throw new Error(`Invalid ${key} parameter`);
  }
  return value;
}

export function encodePathSegment(param: string): string {
  return encodeURIComponent(param);
}

/**
 * Returns a required route param encoded for use as one URL path segment.
 */
export function requirePathSegmentParam(
  params: Record<string, unknown>,
  key = "id",
): string {
  return encodePathSegment(requireStringParam(params, key));
}
