import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  // Only protect operator routes (except login)
  if (
    pathname.startsWith("/operator") &&
    !pathname.startsWith("/operator/login")
  ) {
    const operatorToken = request.cookies.get("phoenix-operator-token");
    if (!operatorToken?.value) {
      return NextResponse.redirect(new URL("/operator/login", request.url));
    }
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/operator/((?!login).*)"],
};
