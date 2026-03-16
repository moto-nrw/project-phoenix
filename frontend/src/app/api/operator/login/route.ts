import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { setOperatorTokens } from "~/lib/operator/cookies";
import { extractJwtExpiry } from "~/lib/operator/jwt-utils";
import { getClientForwardHeaders } from "~/lib/client-headers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorLoginRoute" });

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

    const response = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...getClientForwardHeaders(request),
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

    const expiresAt = extractJwtExpiry(data.access_token);

    return NextResponse.json({
      success: true,
      operator: {
        id: data.operator.id.toString(),
        email: data.operator.email,
        displayName: data.operator.display_name,
      },
      ...(expiresAt !== null && { expiresAt }),
    });
  } catch (error) {
    logger.error("operator_login_error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Anmeldefehler. Bitte versuchen Sie es erneut." },
      { status: 500 },
    );
  }
}
