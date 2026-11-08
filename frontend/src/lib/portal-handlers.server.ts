import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import type {
  ResponseRouteHandler,
  ServerAuthRequest,
} from "~/server/auth/route-handler";
import {
  bffError,
  handleApiError,
  isApiResponseError,
} from "./api-response-error.server";
import {
  createBackendFetch,
  readBackendJson,
  renewalFailureResponse,
  renewSession,
  sendBackend,
  unwrapEnvelope,
  type BackendMethod,
  type PortalSession,
  type SessionCall,
} from "./backend-client.server";
import type { ErrorCode } from "./error-codes.generated";
import { forwardBackendResponse } from "./backend-proxy-response.server";
import { fileResponse, type FileResponseOptions } from "./file-response.server";
import { forwardJsonRequest, type JsonProxyOptions } from "./json-proxy.server";
import {
  allowedQueryString,
  bodyTooLargeError,
  buildQueryString,
  createUnauthorizedResponse,
  extractParams,
  parseRequestBody,
  readBoundedBody,
  wrapInApiResponse,
  type RouteContext,
} from "./route-wrapper-utils.server";

/** One portal's session plus the wrapper every route of the portal runs in. */
export interface PortalBinding extends PortalSession {
  withAuth: (
    handler: (
      request: ServerAuthRequest,
      context: RouteContext,
    ) => Promise<Response>,
  ) => ResponseRouteHandler<RouteContext>;
}

type Params = Record<string, unknown>;

/**
 * A handler's result: data the factory answers as `{ success, message,
 * data }`, or a Response it answers as it is.
 */
type NoBodyHandler<T> = (
  request: NextRequest,
  token: string,
  params: Params,
) => Promise<T>;

type WithBodyHandler<T, B> = (
  request: NextRequest,
  body: B,
  token: string,
  params: Params,
) => Promise<T>;

type HandlerOptions = Pick<SessionCall, "retry">;

interface BodyOptions extends HandlerOptions {
  /** Reject a larger request body with 413 while reading it. */
  maxBodyBytes?: number;
}

/** A backend endpoint: a fixed path, or a builder from route params. */
export type ProxyEndpoint = string | ((params: Params) => string);

interface ProxyOptions {
  /** Answer the backend JSON whole instead of its envelope's `data`. */
  raw?: boolean;
}

interface GetProxyOptions extends ProxyOptions {
  /** Forward only these query parameters, first non-empty value each. */
  forwardQuery?: readonly string[];
}

interface DeleteProxyOptions {
  /** Forward the JSON request body, e.g. a confirmation payload. */
  withBody?: boolean;
}

interface FileProxyOptions
  extends FileResponseOptions, Pick<SessionCall, "retry"> {
  /** POST forwards the JSON request body, read once for the retry. */
  method?: "GET" | "POST";
  /** Query parameters passed on; none by default, `true` for all. */
  forwardQuery?: true | readonly string[];
  /** The largest POST body; 1 MiB by default. */
  maxBodyBytes?: number;
}

interface UploadProxyOptions {
  method?: "PUT" | "POST";
  tooLargeCode?: ErrorCode;
  /** Values the catalog text of tooLargeCode names, like max_mb. */
  tooLargeDetails?: Record<string, unknown>;
}

const FILE_POST_MAX_BYTES = 1024 * 1024;

/** Room for the multipart boundaries around the largest admitted file. */
const UPLOAD_OVERHEAD_BYTES = 64 * 1024;

const json = (data: unknown) =>
  data instanceof Response ? data : NextResponse.json(wrapInApiResponse(data));

/** A DELETE without a result answers 204, in every portal. */
const deleted = (data: unknown) =>
  data === null || data === undefined
    ? new NextResponse(null, { status: 204 })
    : json(data);

/**
 * Route handlers for one portal, all on the same backend client and with one
 * response contract. A handler that throws a backend 401 runs once more with
 * the renewed session's token; a refused renewal answers "Token expired", an
 * undecided one 503, a
 * second 401 is the backend's refusal and keeps its body.
 */
export function makePortalHandlers(portal: PortalBinding) {
  const backendFetch = createBackendFetch(portal);

  async function runWithRenewal<T>(
    request: NextRequest,
    token: string,
    run: (token: string) => Promise<T>,
  ): Promise<T | Response> {
    try {
      return await run(token);
    } catch (error) {
      if (!isApiResponseError(error, 401)) throw error;
    }
    const renewal = await renewSession(portal, token);
    if (renewal.outcome !== "renewed") {
      return renewalFailureResponse(renewal.outcome, request);
    }
    return run(renewal.token);
  }

  function route(
    run: (
      request: NextRequest,
      token: string,
      params: Params,
    ) => Promise<Response>,
  ): ResponseRouteHandler<RouteContext> {
    return portal.withAuth(async (request, context) => {
      try {
        const token = (await portal.auth())?.user?.token;
        if (!token) return createUnauthorizedResponse(request);
        return await run(request, token, await extractParams(request, context));
      } catch (error) {
        return handleApiError(error, request);
      }
    });
  }

  /** A handler's backend 401 runs it again unless the endpoint opts out. */
  const renewing = <T>(
    options: HandlerOptions | undefined,
    request: NextRequest,
    token: string,
    run: (token: string) => Promise<T>,
  ) =>
    options?.retry === undefined || options.retry === true
      ? runWithRenewal(request, token, run)
      : run(token);

  const noBody =
    (format: (data: unknown) => Response) =>
    <T>(handler: NoBodyHandler<T>, options?: HandlerOptions) =>
      route(async (request, token, params) => {
        const result = await renewing(options, request, token, (current) =>
          handler(request, current, params),
        );
        return result instanceof Response ? result : format(result);
      });

  const withBody =
    (format: (data: unknown) => Response) =>
    <T, B = unknown>(handler: WithBodyHandler<T, B>, options?: BodyOptions) =>
      route(async (request, token, params) => {
        const body = await parseRequestBody<B>(request, options?.maxBodyBytes);
        const result = await renewing(options, request, token, (current) =>
          handler(request, body, current, params),
        );
        return result instanceof Response ? result : format(result);
      });

  const get = noBody(json);
  const post = withBody(json);
  const put = withBody(json);
  const patch = withBody(json);
  const del = noBody(deleted);
  const delWithBody = withBody(deleted);

  /**
   * The backend's JSON for a token the handler holds. With the browser
   * request, its client identity travels; without, the identity of the
   * request being handled.
   */
  async function api<T>(
    method: BackendMethod,
    endpoint: string,
    token: string,
    body?: unknown,
    options: { idempotencyKey?: string | null; request?: Request } = {},
  ): Promise<T> {
    const response = await sendBackend(endpoint, token, {
      method,
      json: body ?? undefined,
      ...options,
    });
    return readBackendJson<T>(response);
  }

  const resolve = (endpoint: ProxyEndpoint, params: Params) =>
    typeof endpoint === "function" ? endpoint(params) : endpoint;

  const answer = (body: unknown, options?: ProxyOptions) =>
    options?.raw ? body : unwrapEnvelope(body);

  function proxyGet<T = unknown>(
    endpoint: ProxyEndpoint,
    options?: GetProxyOptions,
  ) {
    return get<T>(async (request, token, params) => {
      const query = options?.forwardQuery
        ? allowedQueryString(request, options.forwardQuery)
        : buildQueryString(request);
      const body = await api(
        "GET",
        `${resolve(endpoint, params)}${query}`,
        token,
        undefined,
        { request },
      );
      return answer(body, options) as T;
    });
  }

  function proxyWrite(method: "POST" | "PUT" | "PATCH") {
    const factory = method === "POST" ? post : method === "PUT" ? put : patch;
    return <T = unknown, B = unknown>(
      endpoint: ProxyEndpoint,
      options?: ProxyOptions & BodyOptions,
    ) =>
      factory<T, B>(async (request, body, token, params) => {
        const response = await api(
          method,
          resolve(endpoint, params),
          token,
          body,
          { idempotencyKey: request.headers.get("Idempotency-Key"), request },
        );
        return answer(response, options) as T;
      }, options);
  }

  function proxyDelete(endpoint: ProxyEndpoint, options?: DeleteProxyOptions) {
    if (options?.withBody) {
      return delWithBody(async (request, body, token, params) => {
        await api("DELETE", resolve(endpoint, params), token, body, {
          request,
        });
        return null;
      });
    }
    return del(async (request, token, params) => {
      await api("DELETE", resolve(endpoint, params), token, undefined, {
        request,
      });
      return null;
    });
  }

  const query = (request: NextRequest, forward?: true | readonly string[]) =>
    forward === true
      ? buildQueryString(request)
      : forward
        ? allowedQueryString(request, forward)
        : "";

  /** The backend's answer as it is, on this portal's session. */
  async function forwardToBackend(
    request: NextRequest,
    path: string,
    call: SessionCall = {},
  ): Promise<Response> {
    return forwardBackendResponse(
      await backendFetch(request, path, call),
      request,
    );
  }

  /** A file of the backend on this portal's session; see fileResponse. */
  async function downloadFile(
    request: NextRequest,
    path: string,
    options: FileResponseOptions & SessionCall = {},
  ): Promise<Response> {
    const {
      uploadedFile,
      backendCacheControl,
      cacheControl,
      buffer,
      fallbackType,
      fallbackDisposition,
      ...call
    } = options;
    return fileResponse(await backendFetch(request, path, call), request, {
      uploadedFile,
      backendCacheControl,
      cacheControl,
      buffer,
      fallbackType,
      fallbackDisposition,
    });
  }

  /** Download route: streams the backend's file with its headers. */
  function proxyFile(endpoint: ProxyEndpoint, options: FileProxyOptions = {}) {
    const { method = "GET", forwardQuery, maxBodyBytes, ...rest } = options;
    return portal.withAuth(async (request, context) => {
      try {
        const path = resolve(endpoint, await extractParams(request, context));
        let body: string | undefined;
        if (method === "POST") {
          const limit = maxBodyBytes ?? FILE_POST_MAX_BYTES;
          const bytes = await readBoundedBody(request, limit);
          if (bytes === null) throw bodyTooLargeError(limit);
          body = new TextDecoder().decode(bytes);
        }
        return await downloadFile(
          request,
          `${path}${query(request, forwardQuery)}`,
          {
            ...rest,
            method,
            body,
            contentType: body === undefined ? undefined : "application/json",
          },
        );
      } catch (error) {
        return handleApiError(error, request);
      }
    });
  }

  /**
   * Multipart upload route. The body is read once within maxFileBytes plus
   * multipart room, so the retry can send it again; the backend checks type,
   * size and content.
   */
  function proxyUpload(
    endpoint: ProxyEndpoint,
    maxFileBytes: number,
    options: UploadProxyOptions = {},
  ) {
    const tooLarge = () =>
      bffError(400, "Upload too large", {
        code: options.tooLargeCode ?? "traeger.logo_too_large",
        details: options.tooLargeDetails,
      });
    return portal.withAuth(async (request, context) => {
      try {
        const contentType = request.headers.get("content-type") ?? "";
        if (!contentType.startsWith("multipart/form-data")) {
          throw bffError(400, "Expected a multipart upload");
        }
        const body = await readBoundedBody(
          request,
          maxFileBytes + UPLOAD_OVERHEAD_BYTES,
        );
        if (body === null) throw tooLarge();
        const path = resolve(endpoint, await extractParams(request, context));
        const response = await backendFetch(request, path, {
          method: options.method ?? "PUT",
          body,
          contentType,
        });
        return forwardBackendResponse(response, request);
      } catch (error) {
        return handleApiError(error, request);
      }
    });
  }

  /** Forwards the backend's answer as it is, on this portal's session. */
  function jsonProxy(options: JsonProxyOptions & Pick<SessionCall, "retry">) {
    return portal.withAuth((request, context) =>
      forwardJsonRequest(request, context, options, (path, call) =>
        backendFetch(request, path, { ...call, retry: options.retry }),
      ),
    );
  }

  return {
    route,
    runWithRenewal,
    get,
    post,
    put,
    patch,
    del,
    delWithBody,
    api,
    backendFetch,
    forwardToBackend,
    proxyGet,
    proxyPost: proxyWrite("POST"),
    proxyPut: proxyWrite("PUT"),
    proxyPatch: proxyWrite("PATCH"),
    proxyDelete,
    jsonProxy,
    downloadFile,
    proxyFile,
    proxyUpload,
  };
}
