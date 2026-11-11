import { NextResponse } from "next/server";
import { problemBody, problemTitle } from "./api-error";
import { applyPrivateNoStore } from "./private-no-store";
import { safeProblemExtensions } from "./api-response-error.server";
import { requestInstance } from "./request-id";

/**
 * The body of a backend fetch response, to hand on in a new Response.
 * undici cancels the body stream of a fetch Response that is garbage
 * collected while nobody has locked that stream yet, and a new Response does
 * not lock the stream it wraps. A collection between this hand-off and Next
 * reading the body (the awaits of the session cookie exit) left a cancelled
 * body: the route answered an empty 500. Piping locks the stream at once.
 */
export function forwardedBody(
  response: Response,
): ReadableStream<Uint8Array> | null {
  return (
    response.body?.pipeThrough(new TransformStream<Uint8Array, Uint8Array>()) ??
    null
  );
}

/** Forward a backend response, replacing unsafe backend 5xx bodies. */
export function forwardBackendResponse(
  response: Response,
  request?: Request,
): NextResponse {
  const headers = new Headers();
  for (const name of ["Content-Type", "Retry-After"]) {
    const value = response.headers.get(name);
    if (value) headers.set(name, value);
  }

  if (response.status >= 500) {
    headers.set("Content-Type", "application/problem+json");
    applyPrivateNoStore(headers);
    return new NextResponse(sanitizedBackendErrorBody(response, request), {
      status: response.status,
      headers,
    });
  }

  // Token routes without a session (enrollment edit links) also pass here.
  applyPrivateNoStore(headers);
  return new NextResponse(
    response.status === 204 || response.status === 304
      ? null
      : forwardedBody(response),
    { status: response.status, headers },
  );
}

function sanitizedBackendErrorBody(
  response: Response,
  request?: Request,
): ReadableStream<Uint8Array> {
  return new ReadableStream({
    async start(controller) {
      let parsed: unknown;
      try {
        parsed = JSON.parse(await response.text());
      } catch {
        // A non-JSON or unreadable upstream body only keeps its status class.
      }

      const body = problemBody(response.status, problemTitle(response.status), {
        ...safeProblemExtensions(parsed, response.status),
        instance: requestInstance(request),
      });
      controller.enqueue(new TextEncoder().encode(JSON.stringify(body)));
      controller.close();
    },
  });
}
