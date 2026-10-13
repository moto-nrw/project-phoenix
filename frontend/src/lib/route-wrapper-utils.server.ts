import type { NextRequest, NextResponse } from "next/server";
import type { ApiResponse } from "./api-helpers";
import type { ProblemBody } from "./api-error";
import { bffError, problemResponse } from "./api-response-error.server";

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
  if (maxBodyBytes === undefined) {
    const text = await request.text();
    if (!text) {
      return {} as B;
    }
    return JSON.parse(text) as B;
  }
  // The bounded path validates the body strictly: size while reading (413)
  // and JSON shape after (400). A bare JSON.parse SyntaxError carries no
  // status, so handleApiError would answer 500 for what is a client-side
  // malformed request (review #2372).
  const text = await readBodyBounded(request, maxBodyBytes);
  if (!text) {
    return {} as B;
  }
  try {
    return JSON.parse(text) as B;
  } catch {
    throw bffError(400, "request body is not valid JSON");
  }
}

/**
 * Reads the request body while enforcing a byte limit DURING the read: an
 * oversized payload is rejected as soon as the counter passes the limit,
 * instead of being buffered whole by request.text() and only then bounced by
 * the backend. The thrown error carries status 413, which handleApiError
 * forwards.
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
  return bffError(413, `request body exceeds ${maxBodyBytes} bytes`, {
    details: { max_bytes: maxBodyBytes },
  });
}

export function wrapInApiResponse<T>(data: T): ApiResponse<T> {
  if (typeof data === "object" && data !== null && "success" in data) {
    return data as unknown as ApiResponse<T>;
  }
  return { success: true, message: "Success", data };
}

export function createUnauthorizedResponse(
  request?: Request,
): NextResponse<ProblemBody> {
  return problemResponse(401, "Unauthorized", request);
}

/** The refreshed session could not repeat a request the backend answered 401. */
export function createTokenExpiredResponse(
  request?: Request,
): NextResponse<ProblemBody> {
  return problemResponse(401, "Token expired", request);
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
