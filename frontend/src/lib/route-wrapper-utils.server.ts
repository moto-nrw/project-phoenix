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

/** Only the listed keys, first non-empty value each, in the listed order. */
export function allowedQueryString(
  request: NextRequest,
  allowed: readonly string[],
): string {
  const forwarded = new URLSearchParams();
  for (const key of allowed) {
    const value = request.nextUrl.searchParams.get(key);
    if (value) forwarded.set(key, value);
  }
  const qs = forwarded.toString();
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

/**
 * The request's JSON body, `{}` when empty. A malformed body is the client's
 * mistake (400, review #2372); with `maxBodyBytes` an oversized one is
 * refused with 413 while it is read.
 */
export async function parseRequestBody<B>(
  request: NextRequest,
  maxBodyBytes?: number,
): Promise<B> {
  let text: string;
  if (maxBodyBytes === undefined) {
    text = await request.text();
  } else {
    const bytes = await readBoundedBody(request, maxBodyBytes);
    if (bytes === null) throw bodyTooLargeError(maxBodyBytes);
    text = new TextDecoder().decode(bytes);
  }
  if (!text) return {} as B;
  try {
    return JSON.parse(text) as B;
  } catch {
    throw bffError(400, "request body is not valid JSON");
  }
}

/**
 * Reads the request body up to `limit` bytes and stops as soon as it grows
 * beyond, also without a Content-Length, so an oversized payload is never
 * buffered whole. Null means too large.
 */
export async function readBoundedBody(
  request: Request,
  limit: number,
): Promise<Uint8Array<ArrayBuffer> | null> {
  const declared = Number(request.headers.get("content-length") ?? "0");
  if (Number.isFinite(declared) && declared > limit) return null;
  if (!request.body) return new Uint8Array(0);
  const reader = request.body.getReader();
  const chunks: Uint8Array[] = [];
  let size = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    size += value.byteLength;
    if (size > limit) {
      await reader.cancel();
      return null;
    }
    chunks.push(value);
  }
  const body = new Uint8Array(size);
  let offset = 0;
  for (const chunk of chunks) {
    body.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return body;
}

export function bodyTooLargeError(maxBodyBytes: number): Error {
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
 * A required, non-empty string route param; anything else is the request's
 * mistake (400). This does not URL-encode the value; use
 * requirePathSegmentParam for path interpolation.
 */
export function requireStringParam(
  params: Record<string, unknown>,
  key = "id",
): string {
  const value = params[key];
  if (!isStringParam(value) || value === "") {
    throw bffError(400, `Invalid ${key} parameter`);
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

/** A required numeric route param (a database ID); anything else is 400. */
export function requireNumericParam(
  params: Record<string, unknown>,
  key = "id",
): string {
  const value = params[key];
  if (!isStringParam(value) || !/^\d+$/.test(value)) {
    throw bffError(400, `invalid ${key}`);
  }
  return value;
}
