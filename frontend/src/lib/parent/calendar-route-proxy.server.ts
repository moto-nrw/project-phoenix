import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { handleApiError } from "../api-helpers.server";
import {
  type RouteContext,
  createUnauthorizedResponse,
  extractParams,
  parseRequestBody,
  wrapInApiResponse,
} from "../route-wrapper-utils.server";

type Params = Record<string, unknown>;

type EndpointBuilder = (request: NextRequest, params: Params) => string;

async function parentCalendarFetch<T>(
  endpoint: string,
  token: string,
  options: { method: string; body?: unknown },
): Promise<T> {
  const { getServerApiUrl } = await import("~/lib/server-api-url");
  const response = await fetch(`${getServerApiUrl()}${endpoint}`, {
    method: options.method,
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: options.body ? JSON.stringify(options.body) : undefined,
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`API error (${response.status}): ${errorText}`);
  }
  if (response.status === 204) return undefined as T;

  const json = (await response.json()) as unknown;
  if (
    typeof json === "object" &&
    json !== null &&
    "data" in json &&
    "status" in json
  ) {
    return (json as { data: T }).data;
  }
  return json as T;
}

async function parentToken(): Promise<string | null> {
  const { parentAuth } = await import("~/server/auth/parent");
  const session = await parentAuth();
  return session?.user?.token ?? null;
}

export function createParentCalendarGetHandler<T>(endpoint: EndpointBuilder) {
  return async (
    request: NextRequest,
    context: RouteContext,
  ): Promise<NextResponse> => {
    try {
      const token = await parentToken();
      if (!token) return createUnauthorizedResponse();
      const params = await extractParams(request, context);
      const data = await parentCalendarFetch<T>(
        endpoint(request, params),
        token,
        {
          method: "GET",
        },
      );
      return NextResponse.json(wrapInApiResponse(data));
    } catch (error) {
      return handleApiError(error);
    }
  };
}

export function createParentCalendarPostHandler<T, B = unknown>(
  endpoint: EndpointBuilder,
) {
  return async (
    request: NextRequest,
    context: RouteContext,
  ): Promise<NextResponse> => {
    try {
      const token = await parentToken();
      if (!token) return createUnauthorizedResponse();
      const params = await extractParams(request, context);
      const body = await parseRequestBody<B>(request);
      const data = await parentCalendarFetch<T>(
        endpoint(request, params),
        token,
        {
          method: "POST",
          body,
        },
      );
      return NextResponse.json(wrapInApiResponse(data));
    } catch (error) {
      return handleApiError(error);
    }
  };
}
