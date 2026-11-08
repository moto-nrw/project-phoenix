import type { NextRequest } from "next/server";
import {
  BackendUnavailableError,
  handleApiError,
  problemResponse,
} from "./api-response-error.server";
import type { BackendCall, BackendMethod } from "./backend-client.server";
import { forwardBackendResponse } from "./backend-proxy-response.server";
import { createLogger } from "./logger";
import type { RouteContext } from "./route-wrapper-utils.server";

const logger = createLogger({ component: "JsonBackendProxy" });

export interface JsonProxyOptions {
  method: BackendMethod;
  /** A fixed path, or one built from the request; a Response answers early. */
  path:
    | string
    | ((
        request: NextRequest,
        params: Record<string, string | string[] | undefined>,
      ) => string | Response);
  forwardQuery?: boolean | readonly string[];
  /** Default: `json` for writes, `none` for GET and DELETE. */
  body?: "json" | "optional-json" | "form" | "none";
  additionalHeaders?: (request: NextRequest) => Record<string, string>;
  mapJsonBody?: (body: unknown, request: NextRequest) => unknown;
  invalidJsonResponse?: (request: NextRequest) => Response;
  onSuccess?: (
    response: Response,
    request: NextRequest,
  ) => Response | Promise<Response>;
  onBackendError?: (response: Response) => void;
  /** A tighter budget than the default; see BACKEND_TIMEOUTS_MS. */
  timeout?: BackendCall["timeout"];
}

/** Sends one backend call: the portal's session client, or a public send. */
export type JsonSend = (
  path: string,
  call: Omit<BackendCall, "request">,
) => Promise<Response>;

async function requestBody(
  request: NextRequest,
  options: JsonProxyOptions,
): Promise<Pick<BackendCall, "json" | "body"> | Response> {
  const kind =
    options.body ??
    (options.method === "GET" || options.method === "DELETE" ? "none" : "json");
  if (kind === "none") return {};
  if (kind === "form") return { body: await request.formData() };
  if (kind === "optional-json") {
    return { json: await request.json().catch(() => ({})) };
  }
  let parsed: unknown;
  try {
    parsed = await request.json();
  } catch {
    return (
      options.invalidJsonResponse?.(request) ??
      problemResponse(400, "Invalid JSON", request)
    );
  }
  if (!options.mapJsonBody) return { json: parsed };
  try {
    return { json: options.mapJsonBody(parsed, request) };
  } catch {
    // A body the route cannot read (a malformed flow ID) is the client's.
    return (
      options.invalidJsonResponse?.(request) ??
      problemResponse(400, "Invalid JSON", request)
    );
  }
}

function withForwardedQuery(
  path: string,
  request: NextRequest,
  { forwardQuery }: JsonProxyOptions,
): string {
  if (!forwardQuery) return path;
  const [pathname = "", existing = ""] = path.split("?", 2);
  const query = new URLSearchParams(existing);
  request.nextUrl.searchParams.forEach((value, key) => {
    if (forwardQuery === true || forwardQuery.includes(key)) {
      query.append(key, value);
    }
  });
  const search = query.toString();
  return search ? `${pathname}?${search}` : pathname;
}

/**
 * Forwards a JSON request to the backend and its answer back as it is:
 * status, body, Content-Type and Retry-After; a 5xx body is sanitized.
 */
export async function forwardJsonRequest(
  request: NextRequest,
  context: RouteContext | undefined,
  options: JsonProxyOptions,
  send: JsonSend,
): Promise<Response> {
  try {
    const resolved =
      typeof options.path === "string"
        ? options.path
        : options.path(request, (await context?.params) ?? {});
    if (resolved instanceof Response) return resolved;
    const body = await requestBody(request, options);
    if (body instanceof Response) return body;
    const response = await send(
      withForwardedQuery(resolved, request, options),
      {
        method: options.method,
        ...body,
        headers: options.additionalHeaders?.(request),
        timeout: options.timeout,
      },
    );
    if (!response.ok) {
      try {
        options.onBackendError?.(response);
      } catch {
        // Observability must not replace a backend response.
      }
      return forwardBackendResponse(response, request);
    }
    return options.onSuccess
      ? options.onSuccess(response, request)
      : forwardBackendResponse(response, request);
  } catch (error) {
    const cause =
      error instanceof BackendUnavailableError ? error.cause : error;
    logger.error("JSON backend proxy failed", {
      method: options.method,
      path:
        typeof options.path === "string"
          ? options.path
          : request.nextUrl.pathname,
      error: cause instanceof Error ? cause.message : String(cause),
    });
    return handleApiError(error, request);
  }
}
