import { NextResponse } from "next/server";
import {
  getOperatorRefreshToken,
  setOperatorTokens,
} from "~/lib/operator/cookies";
import { extractJwtExpiry } from "~/lib/operator/jwt-utils";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorRefreshRoute" });

interface BackendRefreshPayload {
  access_token: string;
  refresh_token: string;
}

interface BackendEnvelopeResponse {
  status: string;
  data: BackendRefreshPayload;
  message: string;
}

export async function POST() {
  try {
    const refreshToken = await getOperatorRefreshToken();
    if (!refreshToken) {
      return NextResponse.json({ error: "No refresh token" }, { status: 401 });
    }

    const { getServerApiUrl } = await import("~/lib/server-api-url");
    const url = `${getServerApiUrl()}/operator/auth/refresh`;

    const response = await fetch(url, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${refreshToken}`,
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      logger.warn("operator_refresh_failed", { status: response.status });
      return NextResponse.json(
        { error: "Token refresh failed" },
        { status: response.status },
      );
    }

    const envelope = (await response.json()) as BackendEnvelopeResponse;
    const data = envelope.data;

    await setOperatorTokens(data.access_token, data.refresh_token);

    const expiresAt = extractJwtExpiry(data.access_token);

    return NextResponse.json({
      success: true,
      ...(expiresAt !== null && { expiresAt }),
    });
  } catch (error) {
    logger.error("operator_refresh_error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Token refresh failed" },
      { status: 500 },
    );
  }
}
