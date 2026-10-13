import { NextResponse } from "next/server";
import { problemTitle } from "./api-error";
import { problemResponse } from "./api-response-error.server";

/** Forward a backend response, replacing unsafe backend 5xx bodies. */
export function forwardBackendResponse(response: Response): NextResponse {
  const headers = new Headers();
  for (const name of ["Content-Type", "Retry-After"]) {
    const value = response.headers.get(name);
    if (value) headers.set(name, value);
  }

  if (response.status >= 500) {
    return problemResponse(
      response.status,
      problemTitle(response.status),
      {},
      headers,
    );
  }

  return new NextResponse(
    response.status === 204 || response.status === 304 ? null : response.body,
    { status: response.status, headers },
  );
}
