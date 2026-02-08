import { auth, signIn } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

// Logger instance for token refresh
const logger = createLogger({ component: "TokenRefresh" });

type TokenPair = {
  accessToken: string;
  refreshToken: string;
};

let refreshPromise: Promise<TokenPair | null> | null = null;

/**
 * Refresh the user session on the server by exchanging the refresh token for a new access token.
 * Ensures only a single refresh is in-flight at a time to avoid invalidating rotating refresh tokens.
 */
export async function refreshSessionTokensOnServer(): Promise<TokenPair | null> {
  if (refreshPromise) {
    return refreshPromise;
  }

  refreshPromise = (async () => {
    try {
      const session = await auth();

      const refreshToken = session?.user?.refreshToken;
      if (!refreshToken) {
        logger.warn("server-side refresh requested without refresh token", {});
        return null;
      }

      const response = await fetch(`${getServerApiUrl()}/auth/refresh`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${refreshToken}`,
          "Content-Type": "application/json",
        },
      });

      if (!response.ok) {
        const errorText = await response.text().catch(() => "");
        logger.error("server-side token refresh failed", {
          status: response.status,
          error_text: errorText.substring(0, 200),
        });
        return null;
      }

      const tokens = (await response.json()) as {
        access_token: string;
        refresh_token: string;
      };

      try {
        await signIn("credentials", {
          redirect: false,
          internalRefresh: "true",
          token: tokens.access_token,
          refreshToken: tokens.refresh_token,
        });
      } catch (signInError) {
        logger.error("failed to persist refreshed tokens", {
          error:
            signInError instanceof Error
              ? signInError.message
              : String(signInError),
        });
        return null;
      }

      return {
        accessToken: tokens.access_token,
        refreshToken: tokens.refresh_token,
      } satisfies TokenPair;
    } catch (error) {
      logger.error("unexpected error during token refresh", {
        error: error instanceof Error ? error.message : String(error),
      });
      return null;
    }
  })();

  try {
    return await refreshPromise;
  } finally {
    refreshPromise = null;
  }
}
