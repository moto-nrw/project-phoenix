import { NextResponse } from "next/server";

/** Forward a backend response without re-parsing or replacing its body. */
export function forwardBackendResponse(response: Response): NextResponse {
  const headers = new Headers();
  for (const name of ["Content-Type", "Retry-After"]) {
    const value = response.headers.get(name);
    if (value) headers.set(name, value);
  }

  return new NextResponse(
    response.status === 204 || response.status === 304 ? null : response.body,
    { status: response.status, headers },
  );
}
