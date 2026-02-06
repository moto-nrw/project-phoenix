import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { getOperatorToken } from "./cookies";
import { handleApiError } from "../api-helpers";
import {
  type RouteContext,
  buildQueryString,
  extractParams,
  parseRequestBody,
  wrapInApiResponse,
  createUnauthorizedResponse,
} from "../route-wrapper-utils";

async function operatorServerFetch<T>(
  endpoint: string,
  token: string,
  options: { method: string; body?: unknown },
): Promise<T> {
  const { getServerApiUrl } = await import("~/lib/server-api-url");
  const url = `${getServerApiUrl()}${endpoint}`;

  const response = await fetch(url, {
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

  if (response.status === 204) {
    return undefined as T;
  }

  const json: unknown = await response.json();

  // Unwrap backend envelope { status, data, message } from common.Respond()
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

export function operatorApiGet<T>(endpoint: string, token: string): Promise<T> {
  return operatorServerFetch<T>(endpoint, token, { method: "GET" });
}

export function operatorApiPost<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return operatorServerFetch<T>(endpoint, token, { method: "POST", body });
}

export function operatorApiPut<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return operatorServerFetch<T>(endpoint, token, { method: "PUT", body });
}

export function operatorApiDelete<T>(
  endpoint: string,
  token: string,
): Promise<T> {
  return operatorServerFetch<T>(endpoint, token, { method: "DELETE" });
}

type NoBodyHandler<T> = (
  request: NextRequest,
  token: string,
  params: Record<string, unknown>,
) => Promise<T>;

type WithBodyHandler<T, B> = (
  request: NextRequest,
  body: B,
  token: string,
  params: Record<string, unknown>,
) => Promise<T>;

export function createOperatorGetHandler<T>(handler: NoBodyHandler<T>) {
  return async (
    request: NextRequest,
    context: RouteContext,
  ): Promise<NextResponse> => {
    try {
      const token = await getOperatorToken();
      if (!token) return createUnauthorizedResponse();
      const params = await extractParams(request, context);
      const data = await handler(request, token, params);
      return NextResponse.json(wrapInApiResponse(data));
    } catch (error) {
      return handleApiError(error);
    }
  };
}

export function createOperatorPostHandler<T, B = unknown>(
  handler: WithBodyHandler<T, B>,
) {
  return async (
    request: NextRequest,
    context: RouteContext,
  ): Promise<NextResponse> => {
    try {
      const token = await getOperatorToken();
      if (!token) return createUnauthorizedResponse();
      const params = await extractParams(request, context);
      const body = await parseRequestBody<B>(request);
      const data = await handler(request, body, token, params);
      return NextResponse.json(wrapInApiResponse(data));
    } catch (error) {
      return handleApiError(error);
    }
  };
}

export function createOperatorPutHandler<T, B = unknown>(
  handler: WithBodyHandler<T, B>,
) {
  return async (
    request: NextRequest,
    context: RouteContext,
  ): Promise<NextResponse> => {
    try {
      const token = await getOperatorToken();
      if (!token) return createUnauthorizedResponse();
      const params = await extractParams(request, context);
      const body = await parseRequestBody<B>(request);
      const data = await handler(request, body, token, params);
      return NextResponse.json(wrapInApiResponse(data));
    } catch (error) {
      return handleApiError(error);
    }
  };
}

export function createOperatorDeleteHandler<T>(handler: NoBodyHandler<T>) {
  return async (
    request: NextRequest,
    context: RouteContext,
  ): Promise<NextResponse> => {
    try {
      const token = await getOperatorToken();
      if (!token) return createUnauthorizedResponse();
      const params = await extractParams(request, context);
      const data = await handler(request, token, params);
      if (data === null || data === undefined) {
        return new NextResponse(null, { status: 204 });
      }
      return NextResponse.json(wrapInApiResponse(data));
    } catch (error) {
      return handleApiError(error);
    }
  };
}

export function createOperatorProxyGetHandler<T>(backendEndpoint: string) {
  return createOperatorGetHandler<T>(
    async (request: NextRequest, token: string) => {
      const endpoint = `${backendEndpoint}${buildQueryString(request)}`;
      return operatorApiGet<T>(endpoint, token);
    },
  );
}

export { isStringParam } from "../route-wrapper-utils";
