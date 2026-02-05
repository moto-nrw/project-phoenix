import { NextResponse } from "next/server";
import { getOperatorToken } from "~/lib/operator/cookies";

interface JwtPayload {
  sub: string;
  username: string;
  first_name: string;
  scope: string;
  exp: number;
}

function decodeJwtPayload(token: string): JwtPayload | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;
    const payload = parts[1];
    if (!payload) return null;
    const decoded = Buffer.from(payload, "base64url").toString("utf-8");
    return JSON.parse(decoded) as JwtPayload;
  } catch {
    return null;
  }
}

export async function GET() {
  const token = await getOperatorToken();
  if (!token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const payload = decodeJwtPayload(token);
  if (!payload) {
    return NextResponse.json({ error: "Invalid token" }, { status: 401 });
  }

  // Check expiry
  if (payload.exp * 1000 < Date.now()) {
    return NextResponse.json({ error: "Token expired" }, { status: 401 });
  }

  return NextResponse.json({
    id: payload.sub,
    email: payload.username,
    displayName: payload.first_name,
  });
}
