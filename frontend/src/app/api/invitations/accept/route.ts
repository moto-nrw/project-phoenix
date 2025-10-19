import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { env } from "~/env";

interface AcceptInvitationBody {
  token?: string;
  firstName?: string;
  lastName?: string;
  password: string;
  confirmPassword: string;
}

export async function POST(request: NextRequest) {
  try {
    const body = await request.json() as AcceptInvitationBody;
    if (!body.token) {
      return NextResponse.json({ error: "Missing invitation token" }, { status: 400 });
    }

    const { token, ...rest } = body;
    const payload = {
      first_name: rest.firstName,
      last_name: rest.lastName,
      password: rest.password,
      confirm_password: rest.confirmPassword,
    };

    const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/auth/invitations/${encodeURIComponent(token)}/accept`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(payload),
    });

    const contentType = response.headers.get("Content-Type") ?? "";
    let payloadBody: unknown = null;
    if (contentType.includes("application/json")) {
      payloadBody = await response.json();
    } else {
      const text = await response.text();
      payloadBody = text ? { error: text } : null;
    }

    return NextResponse.json(payloadBody ?? {}, { status: response.status });
  } catch (error) {
    console.error("Invitation accept proxy error:", error);
    return NextResponse.json({ error: "Internal Server Error" }, { status: 500 });
  }
}
