import { NextResponse } from "next/server";
import { problemBody, problemTitle } from "./api-error";
import { safeProblemExtensions } from "./api-response-error.server";

/** Forward a backend response, replacing unsafe backend 5xx bodies. */
export function forwardBackendResponse(response: Response): NextResponse {
  const headers = new Headers();
  for (const name of ["Content-Type", "Retry-After"]) {
    const value = response.headers.get(name);
    if (value) headers.set(name, value);
  }

  if (response.status >= 500) {
    headers.set("Content-Type", "application/problem+json");
    return new NextResponse(sanitizedBackendErrorBody(response), {
      status: response.status,
      headers,
    });
  }

  return new NextResponse(
    response.status === 204 || response.status === 304 ? null : response.body,
    { status: response.status, headers },
  );
}

function sanitizedBackendErrorBody(
  response: Response,
): ReadableStream<Uint8Array> {
  return new ReadableStream({
    async start(controller) {
      let parsed: unknown;
      try {
        parsed = JSON.parse(await response.text());
      } catch {
        // A non-JSON or unreadable upstream body only keeps its status class.
      }

      const body = problemBody(
        response.status,
        problemTitle(response.status),
        safeProblemExtensions(parsed, response.status),
      );
      controller.enqueue(new TextEncoder().encode(JSON.stringify(body)));
      controller.close();
    },
  });
}
