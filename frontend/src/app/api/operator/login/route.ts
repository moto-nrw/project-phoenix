import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { setOperatorTokens } from "~/lib/operator/cookies";

interface LoginRequest {
  email: string;
  password: string;
}

interface BackendLoginPayload {
  access_token: string;
  refresh_token: string;
  operator: {
    id: number;
    email: string;
    display_name: string;
  };
}

interface BackendEnvelopeResponse {
  status: string;
  data: BackendLoginPayload;
  message: string;
}

export async function POST(request: NextRequest) {
  try {
    const body = (await request.json()) as LoginRequest;
    const { getServerApiUrl } = await import("~/lib/server-api-url");
    const url = `${getServerApiUrl()}/operator/auth/login`;

    const ip =
      request.headers.get("x-forwarded-for")?.split(",")[0]?.trim() ??
      request.headers.get("x-real-ip") ??
      "unknown";

    const response = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Forwarded-For": ip,
      },
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      const errorText = await response.text();
      let errorMessage = "Ungültige Anmeldedaten";
      try {
        const errorData = JSON.parse(errorText) as { error?: string };
        errorMessage = errorData.error ?? errorMessage;
      } catch {
        // use default
      }
      return NextResponse.json(
        { error: errorMessage },
        { status: response.status },
      );
    }

    const envelope = (await response.json()) as BackendEnvelopeResponse;
    const data = envelope.data;

    await setOperatorTokens(data.access_token, data.refresh_token);

    return NextResponse.json({
      success: true,
      operator: {
        id: data.operator.id.toString(),
        email: data.operator.email,
        displayName: data.operator.display_name,
      },
    });
  } catch (error) {
    console.error("Operator login error:", error);
    return NextResponse.json(
      { error: "Anmeldefehler. Bitte versuchen Sie es erneut." },
      { status: 500 },
    );
  }
}
