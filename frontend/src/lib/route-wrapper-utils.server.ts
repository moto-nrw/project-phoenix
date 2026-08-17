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

export async function parseRequestBody<B>(
  request: NextRequest,
  maxBodyBytes?: number,
): Promise<B> {
  const text =
    maxBodyBytes === undefined
      ? await request.text()
      : await readBodyBounded(request, maxBodyBytes);
  if (!text) {
    return {} as B;
  }
  return JSON.parse(text) as B;
}

/**
 * Reads the request body while enforcing a byte limit DURING the read: an
 * oversized payload is rejected as soon as the counter passes the limit,
 * instead of being buffered whole by request.text() and only then bounced by
 * the backend. The thrown message carries the "API error (413)" shape so
 * handleApiError maps it to a 413 response.
 */
async function readBodyBounded(
  request: NextRequest,
  maxBodyBytes: number,
): Promise<string> {
  // Fast reject on the declared size; chunked requests without the header
  // still hit the streaming counter below.
  const declared = request.headers.get("content-length");
  if (declared !== null) {
    const declaredBytes = Number(declared);
    if (Number.isFinite(declaredBytes) && declaredBytes > maxBodyBytes) {
      throw bodyTooLargeError(maxBodyBytes);
    }
  }
  if (!request.body) {
    return "";
  }
  const reader = request.body.getReader();
  const chunks: Uint8Array[] = [];
  let totalBytes = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) {
      break;
    }
    totalBytes += value.byteLength;
    if (totalBytes > maxBodyBytes) {
      await reader.cancel();
      throw bodyTooLargeError(maxBodyBytes);
    }
    chunks.push(value);
  }
  const merged = new Uint8Array(totalBytes);
  let offset = 0;
  for (const chunk of chunks) {
    merged.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return new TextDecoder().decode(merged);
}

function bodyTooLargeError(maxBodyBytes: number): Error {
  return new Error(
    `API error (413): request body exceeds ${maxBodyBytes} bytes`,
  );
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
