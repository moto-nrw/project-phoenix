import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import {
  OPERATOR_INVITATION_COOKIE,
  operatorInvitationCookieOptions,
} from "~/lib/operator/operator-invitation-session.server";

const MAX_TOKEN_LENGTH = 4096;

export async function POST(request: NextRequest): Promise<NextResponse> {
  let body: unknown;
  try {
    body = await request.json();
  } catch {
    return NextResponse.json({ message: "Ungültige Anfrage" }, { status: 400 });
  }

  const token =
    typeof body === "object" &&
    body !== null &&
    "token" in body &&
    typeof body.token === "string"
      ? body.token.trim()
      : "";
  if (!token || token.length > MAX_TOKEN_LENGTH) {
    return NextResponse.json({ message: "Ungültige Anfrage" }, { status: 400 });
  }

  const response = new NextResponse(null, { status: 204 });
  response.cookies.set(
    OPERATOR_INVITATION_COOKIE,
    token,
    operatorInvitationCookieOptions,
  );
  return response;
}
